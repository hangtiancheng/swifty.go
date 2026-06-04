# lark_cache 代码评审

本文档基于 `lark_cache/` 目录下的全部源码（含测试），对模块的整体设计、各文件职责、对齐 groupcache 的程度、潜在问题与改进建议进行系统性评审。所有结论均与源码一一对应，便于追踪修订。

模块路径: `github.com/hangtiancheng/lark-go/lark_cache`

参考实现: `github.com/golang/groupcache`

---

## 一、整体架构评审

### 1.1 模块分层

lark_cache 采用单包扁平结构（除 `pb/`），整体可以划分为三层：

- 数据存储层: `byte_view.go`、`store.go`、`lru.go`
- 命名空间与读穿层: `cache.go`、`group.go`、`single_flight.go`
- 分布式层: `peers.go`、`server.go`、`client.go`、`register.go`、`con_hash.go`、`config.go`、`utils.go`、`pb/`

各层职责清晰，依赖方向单一（上层依赖下层），整体设计是合理的。

### 1.2 与 groupcache 的对齐度

| 维度                    | groupcache                              | lark_cache                                                    | 是否对齐                     |
| ----------------------- | --------------------------------------- | ------------------------------------------------------------- | ---------------------------- |
| 命名空间 Group + Getter | 有                                      | 有，签名一致（除多一个 `context.Context`）                    | 基本对齐，且 ctx 更现代      |
| 全局 group 注册表       | 有                                      | 有，并提供 `ListGroups` / `DestroyGroup` / `DestroyAllGroups` | 对齐且增强                   |
| ByteView 不可变值类型   | 有                                      | 有，`b` 字段不可导出，对外返回防御性拷贝                      | 对齐                         |
| singleflight 防穿透     | 有，`groupcache/singleflight`           | 有，`SingleFlightGroup`                                       | 基本对齐                     |
| 一致性哈希 + 虚拟节点   | 有，`consistenthash.Map`                | 有，`ConHashMap`，额外支持自动再平衡                          | 增强                         |
| 远端 Peer 抽象          | `PeerPicker` + `ProtoGetter` (Get only) | `PeerPicker` + `Peer`，支持 Get/Set/Delete                    | 主动写入是 lark_cache 的扩展 |
| 传输协议                | HTTP/Protobuf                           | gRPC                                                          | 不同                         |
| 服务发现                | 静态注入 peers                          | 内置 etcd 服务发现                                            | 增强                         |
| 主写转发 / 删除转发     | 无                                      | 有                                                            | 增强                         |
| LRU 实现                | 单层 LRU                                | 桶分片 + L1/L2 双层 LRU-2                                     | 增强                         |

设计上的总体偏差：groupcache 是只读穿透、最终一致的“分布式只读缓存”，而 lark_cache 主动支持 `Set/Delete` 转发。这是一项有意识的扩展，但带来了一致性与并发上的若干新问题，详见下文。

---

## 二、按文件逐项评审

### 2.1 `byte_view.go`

实现简洁，符合 groupcache 风格。

亮点:

- `b` 字段不可导出，外部无法直接修改底层切片。
- `ByteSlice()` 返回 `cloneBytes(b)`，每次给调用方一份独立拷贝，杜绝写穿。
- `String()` 直接 `string(b.b)` 复用底层数组（Go 的 `string([]byte)` 仍会拷贝）。

不足:

- 没有提供基于 `string` 的零拷贝构造（groupcache 中 `ByteView` 同时支持 `[]byte` 和 `string` 两种底层表示，避免 string 与 byte 之间反复拷贝）。
- 缺少 `Equal`、`At(i)`、`Slice(from,to)`、`WriteTo(io.Writer)` 等便利方法。
- `cloneBytes` 对 `nil` 输入也会分配一个长度为 0 的新切片，不算错但有微小浪费，可加 `if len(b) == 0` 短路返回 `nil`。

### 2.2 `store.go`

定义了 `Value` / `Store` 接口与 `StoreOptions` 默认值。`Store` 抽象到位，便于以后替换实现。

不足:

- `NewStoreOptions().MaxBytes = 8192`（8KB）与 `DefaultCacheOptions().MaxBytes = 8 MiB` 默认值不一致，容易引起误解。
- `MaxBytes` 字段在 `lruStore` 中实际上未被使用（参见 2.3），属于死字段。这是接口契约与实现的明显脱节，必须修复或在文档中明确声明：当前实现仅按条数驱逐。

### 2.3 `lru.go`

这是该项目最复杂、也最值得仔细看的文件，整体实现一个 “桶分片 + L1/L2 双层 LRU-2” 的 in-memory 缓存。

设计要点:

- `BucketCount` 经 `MaskOfNextPowOf2` 规约成 2 的幂，按 `HashBKRD(key) & mask` 分片。
- 每个分片含两级 `cache`：L1（直接写入层）、L2（被读过一次后的提升层），各自独立的 `CapPerBucket` / `Level2Cap` 容量与 O(1) 双向链表。
- 用全局 `clock`（每 100ms 由 init 启的 goroutine 用 `atomic` 推进）作为粗时钟，避免每次 `time.Now()` 系统调用，是合理的高频路径优化。
- TTL 通过 `expireAt > 0` 表示“有效”；`expireAt = 0` 表示“逻辑删除”。

实现层面的具体问题:

1. MaxBytes 未生效（重要）。`NewStore` 不读取 `opts.MaxBytes`，`put` 路径也仅按 `c.last == cap(c.m)` 判断容量是否满。这意味着声明的“按字节数限制”的语义并不存在，实际只按“每桶最多 N 条”进行驱逐。建议二选一：要么明确文档为“按条数限制”，要么实现真正的字节计数与超限驱逐。

2. `Clear()` 与 `Len()` 中持锁后再调用 `s.Delete(key)` / `s.delete(...)` 有死锁/重复加锁风险（重要）。`Clear` 在 walk 时持有 `s.locks[i]`，walk 结束后释放，最后再调用 `s.Delete(key)`（会再次加同一锁），实际不会死锁但效率低；不过 walk 阶段 `s.caches[i][1].walk` 内部对 L1 的 `keys` 做了 O(N) 去重扫描，复杂度退化为 O(N^2)，应当用 `map[string]struct{}` 收集。`Len()` 同样存在“L1 与 L2 同时计数 + 没有去重”的隐患，因为 L1 命中升级到 L2 时 L1 的槽位 `expireAt` 被置 0，会被 `walk` 跳过，所以正常情况下不会重复计数；但这种隐式不变量极脆弱，建议显式维护 `count` 字段。

3. L1/L2 提升语义。`Get` 在 L1 命中后调用 `c.caches[idx][0].del(key)`，这里的 `del` 实际只是把 `expireAt` 置 0 并把节点挪到链表 LRU 尾（`adjust(idx, n, p)`），并未从 `hashMap` 删除。等下一次该槽位被复用时（达到 cap 后 `put` 走到尾节点复用分支），才会从 `hashMap` 删除。这意味着：在 L1 命中升级后到下次驱逐之间，L1 的 `hashMap[key]` 仍指向旧槽位，`get` 仍可能再次命中并再次升级，多次重复 promotion。这是一个隐藏的语义瑕疵，应在 `del` 中显式 `delete(c.hashMap, key)`。

4. `_get`（L2 命中路径）调用 `c.get(key)` 时会 `adjust(idx, p, n)`，把节点移到链表头，是标准 LRU 行为，符合预期。但 L2 的过期校验是 `n.expireAt <= 0 || currentTime >= n.expireAt` 时返回 `(nil, 0)`，并未触发删除，过期项要等下一次 `cleanupLoop` 才能被回收，会占用容量，需要在 `_get` 中检测到过期时主动 `s.delete(key, idx)`。

5. 全局 `clock` 推进 goroutine。`init` 中启动的 goroutine 没有任何停止入口，并且对 `clock` 的精度仅有 100ms 量级。对单元测试中要求毫秒级精度的 TTL 测试（如 150ms 过期）依赖了实测的 sleep 400ms 来掩盖偏差，业务上对 TTL 精度敏感的场景需要注意。

6. `cleanupLoop` 在 L2 校验时也用了 O(N) 的 `for _, k := range expiredKeys` 去重，复杂度同 `Clear`。

7. `cache.put` 实现细节虽巧妙（使用 `[2]uint16` 数组承载“双向链表的前驱/后继索引”），但可读性较差，且 `p, n` 是包级变量 `0` 和 `1`，硬编码味重；建议加上明确的常量名 `prev = 0, next = 1`，或封装为内嵌方法。

8. `var clock, p, n = ...` 把三个不相关的全局变量放在同一行声明，`p` 和 `n` 既是 `uint16` 常量索引，又作为可变变量，极易误用。

9. `lruStore` 的 `cleanupLoop` 与 `Close` 之间没有 join。`Close` 关闭 `closeCh`，cleanup goroutine 会在下一个 tick 退出，但 `Close` 返回时该 goroutine 仍可能在执行，不影响正确性但需要意识到。

10. `HashBKRD` 是 int32 返回，`& mask`（uint16）后强转 int32 的隐式语义在 mask 为最大 65535 时仍正确，但若以后 `BucketCount` 超过 65536 会有问题（uint16 表示上限）。

### 2.4 `cache.go`

`Cache` 是 `Store` 的薄包装，提供延迟初始化、命中统计、关闭幂等。

亮点:

- `ensureInitialized` 使用 atomic 双检 + `mu`，避免每次 Get/Add 都加锁。
- `Close` 用 `CompareAndSwapInt32` 保障幂等。
- `Stats()` 输出便于观测。

不足:

1. 读写锁使用反直觉。`Add`、`Delete`、`Clear` 是写操作，但 `Add` 完全没有加 `mu`；`Delete` 仅加 `mu.RLock()`；`Clear` 加 `mu.Lock()`。实际上下层 `store` 自带分片锁，`Cache.mu` 仅应保护 `store` 引用本身的替换（`ensureInitialized`/`Close`），目前的锁结构虽然不会立刻出错，但容易让维护者误以为 `mu` 是数据锁。建议：把 `mu` 的语义限定为“保护 `store` 字段的初始化与置 nil”，所有数据操作不要再持有这把锁。
2. `Get` 在 `c.store.Get(key)` 之后再判断 `bv, ok := val.(ByteView)`；理论上 store 只接收 `ByteView`（由 `Cache.Add` 唯一入口写入），断言不会失败，可以省一次反射开销，或在 set 处加约束。
3. `AddWithExpiration` 把绝对时间转相对时间后再传给 `store.SetWithExpiration`，再在 store 内转回绝对时间，两次换算，建议直接在 store 上提供绝对时间接口。
4. `Stats()` 在并发场景下读 `hits` 与 `misses` 用了原子 Load，但 `hit_rate = hits / (hits+misses)` 不是原子计算，可能产生轻微偏差，无大碍。

### 2.5 `group.go`

`Group` 是对外的核心 API，组合 `mainCache + loader + peers`。

亮点:

- 与 groupcache 高度一致：`Getter` 接口、`GetterFunc` 适配器、`NewGroup` + 全局 registry。
- 提供了 `Set` / `Delete` / `Clear` / `Close` / `Stats`，超出 groupcache 的能力。
- `loadData` 的优先级：本地未命中 -> 选 peer -> 远端 peer 拉 -> 失败回退 `getter` 本地加载。这是合理的“最终回退”策略。
- 通过 `peerRequestContextKey` 区分“调用源是否为对端 peer”，避免 `Set/Delete` 在网内循环转发，是关键的正确性细节。
- `Stats` 输出维度丰富（loads / local*hits / local_misses / peer_hits / peer_misses / loader_hits / loader_errors / hit_rate / avg_load_time_ms / cache*\*）。

问题与改进:

1. `syncToPeers` 使用 `context.Background()` 重新构造 ctx，丢弃了上游传入的超时、取消、trace。建议改为 `withPeerRequest(context.WithoutCancel(ctx))`（Go 1.21+）或显式 `WithTimeout`。
2. `syncToPeers` 启用 `go` 是 fire-and-forget，失败只在日志中体现，调用方无法感知。可以增加可选回调或基于 channel 的错误收集，避免悄无声息丢失数据。
3. `Set` 当前直接写本地 + 异步通知 owning peer。一个 key 在哪台机器上是“owner”依赖 `PickPeer`；当多节点同时 `Set` 同一 key 但路由暂未稳定（节点上下线 + 再平衡），会出现各节点本地副本不一致的情况。文档应当明确：本框架的 `Set` 是“最终一致”的写传播，不是强一致复制。
4. `Set` 当 `len(value) == 0` 报 `ErrValueRequired`，但缓存空值有时是合理需求（用于 negative cache 防穿透），建议显式提供 `SetEmpty` 或允许零长度。
5. `Get` 当 `peers.PickPeer` 返回 `isSelf == true` 时走 `getter`，但当 PickPeer 返回的 `peer` 正是 self，本地却已 miss，意味着 owner 节点的缓存被驱逐或从未写入。当前实现会走 getter 重新加载，但其他节点的本地副本不会被同步刷新，存在“旧 vs 新”的数据漂移，需要业务自行处理。
6. `Stats` 的 key 名是 string，没有常量声明，调用方拼写易错。建议提供常量或单独结构体。
7. `RegisterPeers` 第二次调用直接 `panic`。考虑到 `NewGroup` 还有 `WithPeers` 入口，调用方组合使用时容易触发 panic，可改为返回 error。
8. `loadData` 中 `peer.Get` 失败仅记 `peer_misses` 并日志输出，未做退避、未做熔断。生产环境一旦某个 peer 宕机或网络抖动，每次 miss 都要走完“远端 RPC -> 失败 -> 本地 getter”，p99 抖动会非常剧烈。建议引入熔断或本地 negative cache。
9. `loader` 字段是 `*SingleFlightGroup`，但 `SingleFlightGroup` 没有 panic recover；若 `getter` panic，`g.loader.Do` 中 `defer c.wg.Done()` 不会执行（panic 会绕过 defer 之前的 panic 抛出？这里实际 defer 仍会执行，但 `g.m.Delete(key)` 后续等待者拿到的 val/err 都是零值，是“静默失败”）。这是危险路径，建议在 `SingleFlightGroup.Do` 内 recover 并把 panic 转 error。
10. `DestroyGroup` 与 `DestroyAllGroups` 都调用 `g.Close()`，但 `Close` 内又会拿一次 `groupsMu` 写锁删除 self；当前用 `delete(groups, name)` 在外层先删，再 `Close`，可以避免双锁；但 `Close` 内部还是会执行一次 `delete(groups, g.name)`（已经不存在了，nop），整体没问题，但逻辑可以理顺。

### 2.6 `single_flight.go`

实现极简：`sync.Map.LoadOrStore` 配合 `sync.WaitGroup`。

不足:

1. 无 `panic` 恢复。`fn` panic 会先触发 `defer c.wg.Done()` 与 `defer g.m.Delete(key)`，但 `c.val`、`c.err` 不会被赋值，等待者拿到的是零值与 nil error，等于静默吞掉 panic 后果。
2. 没有提供 `DoChan` / `Forget` 等 `golang.org/x/sync/singleflight` 标准方法，扩展性差。
3. 没有 `ctx` 支持，超长任务会让所有等待者陷入饥饿；至少可以支持 `Forget(key)`。
4. `call.val` 是 `interface{}`，调用方需要类型断言，缺少泛型版本。

### 2.7 `con_hash.go` + `config.go`

亮点:

- 内置基于 `LoadBalanceThreshold` 的自动再平衡，是相对原生 groupcache 的增强。
- 提供 `GetStats` 观测流量分布。

不足:

1. `Get` 使用了 `mu.Lock()`（写锁），实际只在末尾 `nodeCounts[node]++` 与 `atomic.AddInt64(&m.totalRequests, 1)` 是写操作。可以拆为：用 `mu.RLock` 读 ring，统计计数改为 `atomic` 或单独 `sync.Map`。这是分布式缓存的热点路径，写锁会成为吞吐瓶颈。
2. `removeNodeLocked` 中删除虚节点是 O(N) 线性扫描 + `append` 拼接切片，replicas=200 时一次删除最坏 `200 * len(keys)` 次操作。建议把 `keys` 切换为 `sortedset`（如树或维护倒排索引 `hash -> indexInKeys`），删除复杂度可降到 O(log N) 或 O(1)。
3. `startBalancer` 启动的 goroutine 没有停止入口，`ConHashMap` 无 `Close`。在测试 / 多次创建场景容易 goroutine 泄漏。
4. `rebalanceNodes` 的策略：`newReplicas = currentReplicas / loadRatio` 或 `currentReplicas * (2 - loadRatio)`，对 `loadRatio` 极值情形（接近 0 或 >> 2）会产生剧烈跳变。`(2 - loadRatio)` 在 `loadRatio > 2` 时为负数，被 clamp 到 `MinReplicas`，会出现“最热节点 replicas 被砍到底”+“最冷节点 replicas 被砍到底”的同时下降，反而加剧不均衡。需要换一种平滑策略（如 EWMA + 比例放缩）。
5. `nodeCounts` 在 `rebalanceNodes` 末尾被清零，但若 `checkAndRebalance` 期间有并发 `Get`，会丢统计。
6. `Get(key)` 调用方与 `Add/Remove` 调用方共享一把 `sync.RWMutex`，与 2.6 中的统计写一同竞争同一把锁。
7. `DefaultConHashConfig` 是 `*ConHashConfig` 全局变量，被多个 `ConHashMap` 共享。若调用方修改了 `DefaultConHashConfig.DefaultReplicas`，会污染所有未传 `WithConHashConfig` 的实例。建议改为函数返回值 `func DefaultConHashConfig() *ConHashConfig`。

### 2.8 `peers.go`

定义 `PeerPicker` / `Peer` 接口与基于 etcd 的 `ClientPicker`。

亮点:

- 接口设计干净。`PickPeer` 返回三元组 `(peer, ok, self)` 让调用方明确处理“自身命中”。
- etcd watch 自动维护 peers。

不足:

1. `handleWatchEvents` 在持 `mu.Lock()` 时调用 `p.set(addr)`，而 `p.set` 又调用 `NewClient(...)`（含 `grpc.NewClient`、可能阻塞）。在写锁内做网络 I/O 会卡住所有 `PickPeer` 调用方。建议把 client 创建放在锁外，构造完后再短暂加锁登记。
2. `Close` 中遍历 `clients` 并 `client.Close()`，但 cancel 后 watcher goroutine 也可能并发清理 client，存在双关闭可能。`Client.Close` 内部对 `conn` 多次 `Close` 会得到 error，没有保护。
3. `ClientPicker` 字段 `consHash *ConHashMap` 没有 `Close`，goroutine 泄漏（见 2.7）。
4. `defaultSvcName = "lark_cache"` 与 `register.go` 中 `key := fmt.Sprintf("/services/%s/%s", svcName, addr)` 共享前缀，但常量是私有的，外部使用 `WithServiceName` 时必须与 server 端一致，缺乏运行时校验。
5. `set` 与 `remove` 都没有判断 `consHash.Add` 的 error 返回值。
6. `fetchAllServices` 在初始化时遍历 `resp.Kvs`，没有判 `cancel()` 是否提前完成；使用 3 秒超时是合理的，但如果 etcd 卡顿超过 3 秒就直接返回失败，调用方会拿不到 picker。建议提供降级（先返回空 picker，watch 异步刷新）。
7. `set/remove` 方法名太通用，且与 sync.Map 风格冲突，建议命名 `addPeerLocked` / `removePeerLocked`。

### 2.9 `server.go`

亮点:

- gRPC + health check + 可选 TLS，结构完整。
- `Stop` 通过 `sync.Once` 保证幂等。
- 设置了 `MaxRecvMsgSize`。

不足:

1. `groups *sync.Map` 字段在 `Server` 上声明但从未被读写。`Get/Set/Delete` 都通过包级 `GetGroup` 取，与字段命名误导，建议删除。
2. `MaxRecvMsgSize` 设置了，`MaxSendMsgSize` 没设置，超过 4MB 的响应将被 gRPC 默认值（4MB）拒绝，行为不对称。
3. `Set` 内 `if !isPeerRequest(ctx) { ctx = withPeerRequest(ctx) }` 等价于无条件 `ctx = withPeerRequest(ctx)`，多余的判断。
4. `Get` 返回时使用 `view.ByteSlice()` 触发一次拷贝，再被 gRPC 序列化又一次拷贝。groupcache 的 HTTP 实现使用 sink + sync.Pool 减少拷贝；如果性能敏感可考虑 `ByteView` 暴露 `unsafeBytes()` 内部方法仅供同包使用。
5. `Server.addr` 中如果传入 `":8001"`，`net.Listen` 是 OK 的，但 `Register` 会在 register 内做 ip 拼接得到对外可访问的 addr，server 自身却用原 addr 注册到 etcd 之外，需要保证 `ClientPicker.selfAddr` 与 `Register` 拼接出来的 addr 一致，否则 `PickPeer` 判定 self 会失败，导致自身请求又走 RPC，再次回到自己形成短循环（被 `peerRequestContextKey` 截断但浪费一次网络）。
6. `Start` 启动 `Register` goroutine 后才 `Serve`，如果 `Register` 失败仅打日志不影响 `Serve`，相当于该节点对外不可发现但仍然能处理请求，造成集群视图不一致。
7. `Stop` 没有调用 `healthServer.SetServingStatus(svcName, NOT_SERVING)`，关闭过程客户端仍认为健康。

### 2.10 `client.go`

不足:

1. `NewClient` 中如果传入 `etcdCli == nil`，会内部 `client_v3.New(...)`，但 `Client.Close` 仅关闭 `conn`，不关闭 `etcdCli`，**存在 etcd 连接泄漏**。要么强制要求外部传入，要么在 `Close` 时关闭。
2. `Set` 中 `log.Printf("grpc set request resp: %+v", resp)` 是调试日志遗留，生产环境会刷屏。
3. 所有 RPC 调用使用 `context.WithTimeout(context.Background(), 3*time.Second)`，丢弃了 `Group.syncToPeers` 传入的 `ctx`（实际 `Set` 用了入参 `ctx`，`Get/Delete` 没有）。`Get/Delete` 应改为接收 `ctx` 参数（也意味着 `Peer` 接口需要扩展）。
4. `grpc.NewClient` + `WaitForReady(true)` 在 `peer.Get` 时如果连接不通会阻塞到 3 秒超时再返回；对调用方延迟影响较大。建议设置 keepalive 与连接级 `WithBlock + WithTimeout` 或快速失败。
5. 没有重试 / 熔断。

### 2.11 `register.go`

不足:

1. `Register` 函数同时承担：连 etcd、租约、注册、keepalive、监听 stopCh。职责过多，且 etcd client 在 register 内部独立创建，与 `Server` 中的 etcd client 是两份连接。可以复用 `Server.etcdCli`，减少连接数。
2. `getLocalIP` 选第一个非 loopback 的 IPv4，多网卡或 docker 环境会拿错 IP。建议允许通过参数指定或环境变量覆盖。
3. `addr[0] == ':'` 假设格式（直接索引第一个字节），若传入空字符串会 panic。应使用 `strings.HasPrefix(addr, ":")`。
4. 租约 TTL 写死 10s，无可配置入口；keepalive 失败时只打日志，没有重连或重新注册。
5. `Revoke` 使用 `context.Background()` + 3s 超时，如果 etcd 不可达会卡 3s，但 `Stop` 期望 graceful，需要传可中断 ctx。

### 2.12 `utils.go`

`ValidPeerAddr` 实现过于简单：

- 仅判断点分四段或 `localhost`，IPv6 直接 false。
- 没有校验端口数字与范围。
- 没有校验四段都是 0-255 数字。
- TODO 也明确写了“more selections”。

建议改用 `net.SplitHostPort` + `net.ParseIP`。

### 2.13 `pb/`

`pb/lark.proto` 接口设计极简：`Request{group,key,value}`、`ResponseForGet{value}`、`ResponseForDelete{value}`。

不足:

1. 同一个 `Request` 同时承担 Get / Set / Delete，字段语义按方法不同，可读性差，扩展性差（例如未来加 TTL、Tags 需要新加字段并约定语义）。建议为不同 RPC 提供独立 message。
2. `ResponseForGet` 用于 `Get` 与 `Set` 两个 RPC（`Set` 也返回 `ResponseForGet`），命名误导。
3. 没有 metadata（trace id、租户、版本号等）字段。
4. `Delete` 返回 `bool` 而不是“是否存在过”的 enum，无法区分“key 不存在”与“key 存在并删除成功”。

### 2.14 测试覆盖

测试覆盖面较广，覆盖了：

- `Group` 的 happy path、参数校验、关闭语义、过期、Set/Delete sync、peer 回退、singleflight、RegisterPeers 二次 panic、registry 操作。
- `lruStore` 的基本读写、容量驱逐、过期、L1 升 L2、清理与并发。
- `ConHashMap` 的 add/get/remove、自动再平衡。
- `SingleFlightGroup` 的去重与错误传递。
- `Client` 通过 mock `pb.LarkCacheClient` 测试 RPC happy path 与错误路径。
- `Server` 通过直接调用方法测试 RPC 处理（不真的走网络）。

待补充:

1. 没有 `Server` + `Client` 的端到端 gRPC 测试（用 `bufconn` 很方便）。
2. 没有 etcd 集成测试或假 etcd（可使用 `embed` 包跑一个内嵌实例）。
3. 没有混沌测试：peer 失败 + 自动回退 + 再恢复。
4. `lruStore.cleanupLoop` 的回收行为只在 TTL 短的用例间接覆盖，没有单独的“cleanup goroutine 触发了多少次”观测。
5. 没有基准测试（`Benchmark*`），无法量化 `con_hash.Get` 写锁、`lru.put` 等热点的吞吐。

---

## 三、跨文件设计性问题汇总

1. 一致性模型未声明。`Group.Set` / `Delete` 异步转发给 owner peer，但不保证 “每个节点的本地副本与 owner 一致”。需要文档明确为“最终一致 + 各节点最长 TTL 后收敛”。

2. `MaxBytes` 与“真实容量”脱节。`CacheOptions.MaxBytes` 看起来像“最多占多少字节”，实际只有“每桶 N 条”生效，业务方按字节预算配置后会与现实出入很大。

3. `etcd client` 的生命周期分散在 `Server`、`ClientPicker`、`Register`、`Client` 四处，没有统一管理，存在多连接 / 漏关 / 谁应该 Close 的混乱。建议在包内引入一个 `EtcdSession` 抽象统一持有。

4. goroutine 生命周期不可控。至少三处会启动后台 goroutine 且没有显式停止入口：
   - `init()` 内的 `clock` 推进 goroutine（全局，一次性，影响小）。
   - `lruStore.cleanupLoop`（通过 `Close` 可停止）。
   - `ConHashMap.startBalancer`（无停止入口）。
   - `ClientPicker.watchServiceChanges`（通过 `ctx cancel` 可停止）。

5. 错误处理不一致。部分路径返回 `error`，部分静默 `log.Printf`；部分 panic（`NewGroup`、`RegisterPeers` 二次）。建议梳理：构造期参数错误用 panic，运行期错误用 error，事件性失败用日志 + 指标。

6. 日志使用标准库 `log` 全局 logger，没有日志级别、没有结构化，业务接入难。建议引入接口 `Logger` 由调用方注入。

7. 缺少 metrics 钩子。`Stats()` 是 pull 模型；生产环境通常希望 push 到 Prometheus。可暴露 `Collector` 接口或 `Stats() <-chan Metric`。

8. 包内私有类型暴露过多。`lruStore`、`call`、`cache`（lru 内部）虽未导出，但 `Create`（首字母大写）反而被导出，并在 `lru_test.go` 之外不应当被使用。建议把 `Create` 改为 `create`。同理 `HashBKRD`、`MaskOfNextPowOf2`、`Now` 是否真的需要导出值得商榷。

9. `peerRequestContextKey{}` 是 unexported 类型作为 ctx key 的标准写法，但 `withPeerRequest` 与 `isPeerRequest` 也是 unexported，意味着调用方无法手动设置此标志。`Server.Delete` 调用 `withPeerRequest(ctx)` 是包内调用，OK；但若以后想给客户端透传“我是 peer”，需要导出此 API。

10. 没有节点权重（weight）支持。同构集群尚可，异构集群（不同机器性能不同）需要按权重分配虚拟节点数，目前的 `LoadBalanceThreshold` 自适应可以部分弥补但不是首选方案。

---

## 四、安全与生产可用性

1. gRPC 默认 `insecure.NewCredentials()` 客户端不支持 TLS，与 `Server.WithTLS` 不对称（server 启了 TLS，client 连不上）。
2. 没有 mTLS、没有鉴权（任何能连到端口的请求都能读写任意 group）。生产部署需要在 gRPC interceptor 层补 token / cert 校验。
3. `ValidPeerAddr` 未在 `Register` 或 `ClientPicker.set` 中被调用，恶意/异常 addr 会直接被加进 ring。
4. 没有限流 / 没有 max in-flight，被恶意 key 打穿后端的风险存在（singleflight 仅去重同一 key 的并发）。
5. etcd 端点写死 `localhost:2379`，多数生产环境会失败。`DefaultServerOptions` 与 `DefaultRegisterConfig` 都需要业务显式覆盖。

---

## 五、优先级修复建议

按建议处理顺序排列（高 -> 低）：

1. 修复 `Client` 内部自建 etcd client 的泄漏（client.go）。
2. 修复 `cache.del` 不清理 `hashMap` 导致的重复 promotion（lru.go）。
3. 修复 `Server.Get` 路径未在过期时主动清理 L2 过期项（lru.go `_get`）。
4. 明确 `MaxBytes` 语义并实现按字节驱逐，或修改文档与字段名为 `MaxEntriesPerBucket` 等更准确的名称（store.go / lru.go）。
5. 给 `SingleFlightGroup.Do` 增加 panic recover（single_flight.go）。
6. 修复 `ConHashMap.Get` 用写锁的问题，统计分离 + 改用 RLock（con_hash.go）。
7. 给 `ConHashMap` 增加 `Close` 停止 balancer goroutine（con_hash.go）。
8. `ClientPicker.handleWatchEvents` 把 `NewClient` 移出写锁（peers.go）。
9. 删除 `Server.groups` 死字段，删除 `Client.Set` 的调试日志（server.go / client.go）。
10. `Group.syncToPeers` 不要丢弃上游 ctx，使用 `context.WithoutCancel` 或 `WithTimeout`（group.go）。
11. 重新设计 pb 接口，每个 RPC 独立 message，并加入 metadata 字段（pb/lark.proto）。
12. 引入 `Logger` 接口与 `Metrics` 钩子，移除散落的 `log.Printf`（全局）。
13. 增加端到端 gRPC + etcd 的集成测试（test）。
14. 增加 keepalive、熔断、重试与限流（生产化）。
15. 改进 `con_hash.removeNodeLocked` 的复杂度（con_hash.go）。

---

## 六、结论

lark_cache 的整体架构与 groupcache 风格保持一致，并在主动写传播、服务发现、自适应一致性哈希等方向做了合理扩展，是一个结构清晰、可读性较好的中型 Go 项目。但作为分布式缓存的生产候选实现，仍存在若干必须修复的资源泄漏、并发瑕疵与契约脱节，主要集中在 `lru.go` 的双层 LRU 细节、`client.go` 的 etcd 生命周期、`con_hash.go` 的锁与生命周期、以及 RPC 协议设计上。按本文档第五节的优先级逐步修复后，可显著提升其稳定性与可运维性。
