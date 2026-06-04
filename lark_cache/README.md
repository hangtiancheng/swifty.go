# lark_cache

一款使用 Go 编写、API 风格对齐 [groupcache](https://github.com/golang/groupcache) 的分布式缓存框架。lark_cache 在保留 groupcache 简洁优雅的命名空间模型与读穿透机制的基础上，扩展了写传播、基于 etcd 的服务发现、带自动再平衡的一致性哈希、桶分片 + 双层 LRU 的本地存储等能力，开箱即用、可独立部署。

模块路径: `github.com/hangtiancheng/lark-go/lark_cache`

源码目录: `lark_cache/`

---

## 特性一览

- 命名空间 Group：以 `Group + Getter` 的方式组织缓存，与 groupcache 一致的开发体验。
- 读穿透 + singleflight：本地未命中时自动加载，并对同一 key 的并发加载进行去重，避免缓存击穿。
- 写传播：在 groupcache 只读的基础上，额外提供 `Set` / `Delete` 接口，并按一致性哈希异步同步到 owning peer。
- 桶分片 + L1/L2 双层 LRU：本地存储分桶减少锁争用，热点 key 从 L1 自动晋升到 L2，提升复用。
- TTL 与后台清理：支持按 key 设置过期，后台定时扫描并回收。
- 带自动再平衡的一致性哈希：支持虚拟节点；当节点负载偏差超过阈值时自动调整副本数。
- 基于 etcd 的服务发现：服务端注册租约 + keepalive，客户端 watch 自动维护 peers 列表。
- gRPC 传输：跨节点 RPC 使用 gRPC，支持 health check 与可选 TLS。
- 完整的统计：`Group.Stats()` 暴露命中率、加载耗时、各级命中等指标，便于接入监控。
- 显式生命周期：`Group`、`Cache`、`Server`、`ClientPicker` 均提供 `Close` / `Stop`，便于在测试与服务退出场景安全释放资源。

---

## 安装

```bash
go get github.com/hangtiancheng/lark-go/lark_cache
```

依赖：

- Go 1.21+（推荐使用 `go.mod` 中声明的版本，当前为 `go 1.26.0`）
- gRPC: `google.golang.org/grpc`
- etcd 客户端: `go.etcd.io/etcd/client/v3`（分布式模式使用，单机模式可不部署 etcd）

---

## 快速开始

### 单机模式：只用本地缓存

```go
package main

import (
    "context"
    "fmt"
    "time"

    cache "github.com/hangtiancheng/lark-go/lark_cache"
)

func main() {
    ctx := context.Background()

    // 1. 创建一个命名空间 Group
    //    - 第二个参数是缓存容量（注意：当前实现按"条数"驱逐，详见配置项说明）
    //    - 第三个参数是 Getter：本地未命中时调用以加载数据
    scores := cache.NewGroup("scores", 64<<20, cache.GetterFunc(
        func(ctx context.Context, key string) ([]byte, error) {
            // 模拟从数据库加载
            return []byte("score-of-" + key), nil
        },
    ), cache.WithExpiration(5*time.Minute))
    defer scores.Close()

    // 2. 读取（首次穿透到 Getter，之后命中本地缓存）
    view, err := scores.Get(ctx, "player:42")
    if err != nil {
        panic(err)
    }
    fmt.Println(view.String())

    // 3. 主动写入
    _ = scores.Set(ctx, "player:42", []byte("9999"))

    // 4. 主动删除
    _ = scores.Delete(ctx, "player:42")

    // 5. 查看统计
    fmt.Printf("stats: %+v\n", scores.Stats())
}
```

### 分布式模式：多节点 + 服务发现

每个节点同时运行 gRPC `Server`（对外提供缓存服务）和 `ClientPicker`（发现其他节点）：

```go
package main

import (
    "context"
    "log"
    "time"

    cache "github.com/hangtiancheng/lark-go/lark_cache"
)

func main() {
    ctx := context.Background()

    // 1. 启动本节点的缓存 gRPC 服务
    srv, err := cache.NewServer(
        ":9001",        // 本节点监听地址
        "lark_cache",   // etcd 服务名（所有节点保持一致）
        cache.WithEtcdEndpoints([]string{"127.0.0.1:2379"}),
        cache.WithDialTimeout(5*time.Second),
    )
    if err != nil {
        log.Fatal(err)
    }
    go func() {
        if err := srv.Start(); err != nil {
            log.Printf("server stopped: %v", err)
        }
    }()
    defer srv.Stop()

    // 2. 构造 PeerPicker，通过 etcd 自动发现其他节点
    picker, err := cache.NewClientPicker(
        ":9001",
        cache.WithServiceName("lark_cache"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer picker.Close()

    // 3. 创建带 peers 的 Group
    g := cache.NewGroup("scores", 64<<20, cache.GetterFunc(
        func(ctx context.Context, key string) ([]byte, error) {
            return []byte("loaded-" + key), nil
        },
    ), cache.WithPeers(picker), cache.WithExpiration(time.Minute))
    defer g.Close()

    // 4. 跨节点读写：
    //    - Get 时，若 key 属于其他节点的分片，会自动走 gRPC 拉取
    //    - Set/Delete 时，本节点先写本地，再异步同步到 owning peer
    view, _ := g.Get(ctx, "player:42")
    log.Printf("value=%s", view.String())
}
```

---

## 核心概念

### Group

`Group` 是一个独立的缓存命名空间，承载一份 `Getter`、一份本地缓存 `Cache`、可选的 `PeerPicker` 与一个 `SingleFlightGroup`。同名 Group 在进程内全局唯一。

### Getter

数据加载回调：

```go
type Getter interface {
    Get(ctx context.Context, key string) ([]byte, error)
}

type GetterFunc func(ctx context.Context, key string) ([]byte, error)
```

本地与 peer 都未命中时由 `Getter` 加载，返回值会被缓存并通过 `ByteView` 暴露给调用方。

### ByteView

不可变的字节视图：

```go
view.Len()        // 字节数
view.ByteSlice()  // 防御性拷贝，避免外部修改污染缓存
view.String()     // 转字符串
```

### PeerPicker / Peer

`PeerPicker` 通过一致性哈希为 key 选择 owner 节点；`Peer` 是该节点的 RPC 客户端封装。lark_cache 默认提供 `ClientPicker`（基于 etcd 自动发现） 与 `Client`（gRPC 实现）。

### SingleFlightGroup

对同一 key 的并发加载进行去重：第一个调用者执行 `fn`，其余调用者等待并复用结果，避免缓存击穿与重复回源。

### Cache / Store / lruStore

`Cache` 是 `Store` 的轻量包装，提供延迟初始化与命中统计；底层默认实现 `lruStore` 采用桶分片 + L1/L2 双层 LRU 结构，每个分片独立加锁，并由后台 goroutine 定时清理过期项。

### ConHashMap

一致性哈希环，支持虚拟节点与基于负载偏差的自动再平衡。

---

## API 参考

### 创建与注册 Group

```go
func NewGroup(name string, cacheBytes int64, getter Getter, opts ...GroupOption) *Group

func WithExpiration(d time.Duration) GroupOption     // 默认 TTL；为 0 表示不过期
func WithPeers(peers PeerPicker) GroupOption         // 注入 peers
func WithCacheOptions(opts CacheOptions) GroupOption // 替换默认的本地缓存配置

func GetGroup(name string) *Group        // 获取已注册的 Group
func ListGroups() []string               // 列出所有 Group 名称
func DestroyGroup(name string) bool      // 关闭并移除单个 Group
func DestroyAllGroups()                  // 关闭并移除所有 Group
```

构造时若 `getter == nil` 会 panic；若同名 Group 已存在会被替换并打印警告。

### Group 方法

```go
func (g *Group) Get(ctx context.Context, key string) (ByteView, error)
func (g *Group) Set(ctx context.Context, key string, value []byte) error
func (g *Group) Delete(ctx context.Context, key string) error
func (g *Group) Clear()
func (g *Group) Close() error
func (g *Group) RegisterPeers(peers PeerPicker)   // 仅可调用一次
func (g *Group) Stats() map[string]interface{}    // 命中率、加载耗时、各级命中
```

哨兵错误：`ErrKeyRequired`、`ErrValueRequired`、`ErrGroupClosed`。

### 本地缓存

```go
type CacheOptions struct {
    MaxBytes     int64                                  // 容量上限（当前按条数生效）
    BucketCount  uint16                                 // 分片数，向上取整为 2 的幂
    CapPerBucket uint16                                 // 每个分片 L1 容量
    Level2Cap    uint16                                 // 每个分片 L2 容量
    CleanupTime  time.Duration                          // 后台清理周期
    OnEvicted    func(key string, value Value)          // 驱逐回调
}

func DefaultCacheOptions() CacheOptions
func NewCache(opts CacheOptions) *Cache
```

### 一致性哈希

```go
type ConHashConfig struct {
    DefaultReplicas      int                                  // 默认虚拟节点数 50
    MinReplicas          int                                  // 自动再平衡下限 10
    MaxReplicas          int                                  // 自动再平衡上限 200
    HashFunc             func(data []byte) uint32             // 默认 crc32.ChecksumIEEE
    LoadBalanceThreshold float64                              // 触发再平衡的最大偏差 0.25
}

func NewConHash(opts ...ConHashOption) *ConHashMap
func WithConHashConfig(config *ConHashConfig) ConHashOption

func (m *ConHashMap) Add(nodes ...string) error
func (m *ConHashMap) Remove(node string) error
func (m *ConHashMap) Get(key string) string
func (m *ConHashMap) GetStats() map[string]float64
```

后台再平衡器每秒检查一次：当累计请求超过 1000 且节点间偏差超过 `LoadBalanceThreshold` 时，按负载比例调整每个节点的虚拟节点数（受 `MinReplicas` / `MaxReplicas` 钳制）。

### 服务端 / 客户端

```go
// 服务端
func NewServer(addr, svcName string, opts ...ServerOption) (*Server, error)
func WithEtcdEndpoints(endpoints []string) ServerOption
func WithDialTimeout(timeout time.Duration) ServerOption
func WithTLS(certFile, keyFile string) ServerOption

func (s *Server) Start() error
func (s *Server) Stop()

// 客户端 / 服务发现
func NewClientPicker(addr string, opts ...PickerOption) (*ClientPicker, error)
func WithServiceName(name string) PickerOption

func (p *ClientPicker) PickPeer(key string) (Peer, bool, bool)
func (p *ClientPicker) PrintPeers()
func (p *ClientPicker) Close() error

// 服务注册（被 Server.Start 内部调用，外部一般无需直接使用）
func Register(svcName, addr string, stopCh <-chan error) error
```

`PickPeer` 返回三元组 `(peer, ok, self)`：

- `ok == false`：环上没有可用节点。
- `ok == true && self == true`：key 属于本节点，调用方应走本地路径。
- `ok == true && self == false`：返回的 `Peer` 是远端节点的 RPC 客户端。

### single flight

```go
type SingleFlightGroup struct{ /* ... */ }
func (g *SingleFlightGroup) Do(key string, fn func() (interface{}, error)) (interface{}, error)
```

### 工具函数

```go
func ValidPeerAddr(addr string) bool       // 简易地址校验
func HashBKRD(s string) int32              // BKDR 哈希
func MaskOfNextPowOf2(cap uint16) uint16   // 下一个 2 的幂减一
func Now() int64                           // 内部使用的粗粒度时钟（纳秒）
```

---

## 配置项与默认值

| 配置                                        | 默认值               | 说明                                                                                     |
| ------------------------------------------- | -------------------- | ---------------------------------------------------------------------------------------- |
| `CacheOptions.MaxBytes`                     | 8 MiB                | 设计上为字节预算；当前实现按"每桶条数"驱逐，使用时建议关注 `CapPerBucket` 与 `Level2Cap` |
| `CacheOptions.BucketCount`                  | 16                   | 自动向上取整为 2 的幂                                                                    |
| `CacheOptions.CapPerBucket`                 | 512                  | 每个分片 L1 容量                                                                         |
| `CacheOptions.Level2Cap`                    | 256                  | 每个分片 L2 容量                                                                         |
| `CacheOptions.CleanupTime`                  | 1 分钟               | 后台过期清理周期                                                                         |
| `WithExpiration`                            | 0（不过期）          | Group 级默认 TTL                                                                         |
| `ConHashConfig.DefaultReplicas`             | 50                   | 每个节点的初始虚拟节点数                                                                 |
| `ConHashConfig.MinReplicas` / `MaxReplicas` | 10 / 200             | 自动再平衡的下界与上界                                                                   |
| `ConHashConfig.LoadBalanceThreshold`        | 0.25                 | 偏差超过 25% 时触发再平衡                                                                |
| `ServerOptions.EtcdEndpoints`               | `["localhost:2379"]` | etcd 接入点（生产部署务必覆盖）                                                          |
| `ServerOptions.DialTimeout`                 | 5 秒                 | etcd 连接超时                                                                            |
| `ServerOptions.MaxMsgSize`                  | 4 MiB                | gRPC `MaxRecvMsgSize`，超过该值的响应会被拒绝                                            |
| `RegisterConfig.Endpoints`                  | `["localhost:2379"]` | 服务注册使用的 etcd 接入点                                                               |
| 服务注册租约                                | 10 秒                | 通过 keepalive 续约                                                                      |

---

## 架构与请求流程

### 读路径

```
Group.Get(ctx, key)
  -> 本地 mainCache 命中？是  -> 返回 ByteView
                            否  -> SingleFlightGroup.Do(key)
                                     -> 有 peers？是  -> PickPeer
                                                            -> self 命中？是 -> getter.Get
                                                                              否 -> peer.Get (gRPC)
                                                                                     -> 成功 -> 写本地 -> 返回
                                                                                     -> 失败 -> 回退 getter.Get
                                                       否  -> getter.Get
                                     -> 写入本地（按 TTL）
```

### 写路径

```
Group.Set(ctx, key, value)
  -> 写本地缓存（按 TTL）
  -> ctx 上没有 peerRequest 标记 ? 异步 syncToPeers
                                       -> PickPeer
                                       -> 命中本节点 -> 跳过
                                       -> 命中其他节点 -> peer.Set，并在 ctx 上打 peerRequest 标记防止回环
```

`Group.Delete` 与 `Group.Set` 同结构。`peerRequest` 标记通过私有 ctx key 传递，确保 owner 节点收到后只写本地、不再二次转发。

### 服务发现

```
Server.Start
  -> net.Listen
  -> goroutine: Register(svcName, addr, stopCh)
       -> etcd Grant 10s 租约
       -> Put /services/<svc>/<addr> 带租约
       -> KeepAlive 持续续约
  -> grpcServer.Serve

ClientPicker
  -> 启动时 Get /services/<svc>/ (prefix) 拉取现存节点 -> NewClient + 加入 ring
  -> 后台 Watch /services/<svc>/
       -> PUT  -> NewClient + 加入 ring
       -> DEL  -> client.Close + 从 ring 移除
```

---

## 与 groupcache 的对照

| 维度         | groupcache                   | lark_cache                               |
| ------------ | ---------------------------- | ---------------------------------------- |
| 命名空间     | `Group + Getter`             | `Group + Getter`（多 `context.Context`） |
| 全局注册表   | 有                           | 有，并提供 `ListGroups` / `DestroyGroup` |
| 不可变值类型 | `ByteView`                   | `ByteView`                               |
| 防穿透       | `singleflight`               | `SingleFlightGroup`                      |
| 一致性哈希   | `consistenthash`             | `ConHashMap`，带自动再平衡               |
| 写传播       | 无（只读）                   | 有 `Set` / `Delete`                      |
| 节点抽象     | `PeerPicker` + `ProtoGetter` | `PeerPicker` + `Peer`（Get/Set/Delete）  |
| 传输         | HTTP / Protobuf              | gRPC                                     |
| 服务发现     | 调用方静态注入               | 内置 etcd                                |
| 本地存储     | 单层 LRU                     | 桶分片 + L1/L2 双层 LRU                  |

整体上 lark_cache 保留了 groupcache 的核心 API 风格与心智模型（`Group + Getter + PeerPicker + singleflight`），同时面向“需要写传播 + 自动服务发现”的微服务场景做了一系列增强。

---

## 测试

仓库内提供了较完整的单元测试：

```bash
cd lark_cache
go test ./...
```

测试覆盖的关键场景：

- `group_test.go`：Get 命中本地、参数校验、关闭语义、过期、Set/Delete 同步到 peer、peer 失败回退本地、`RegisterPeers` 二次 panic、`SingleFlightGroup` 去重、`DestroyGroup` / `DestroyAllGroups` 不死锁。
- `lru_test.go`：基础读写、容量驱逐、过期与后台清理、L1 升 L2、并发读写。
- `con_hash_test.go`：节点增删、统计、`checkAndRebalance` 不死锁。
- `single_flight_test.go`：去重、错误传递、顺序调用计数。
- `client_test.go`：通过 mock `pb.LarkCacheClient` 测试 Get/Set/Delete 的成功与错误路径。

---

## 目录结构

```
lark_cache/
  byte_view.go        # ByteView 不可变字节视图
  store.go            # Value / Store 接口，StoreOptions
  lru.go              # 桶分片 + L1/L2 双层 LRU 实现，含全局粗时钟
  cache.go            # Cache 包装：延迟初始化 + 命中统计 + 关闭幂等
  group.go            # Group、Getter、全局注册表、读穿透 / 写传播 / 统计
  single_flight.go    # 并发同 key 加载去重
  con_hash.go         # 一致性哈希环 + 自动再平衡
  config.go           # ConHashConfig 默认值
  peers.go            # PeerPicker / Peer 接口与 ClientPicker（基于 etcd）
  server.go           # gRPC 服务端 + health check + 可选 TLS
  client.go           # gRPC Peer 客户端实现
  register.go         # etcd 服务注册 + 租约 keepalive
  utils.go            # ValidPeerAddr 等工具
  pb/                 # protobuf 定义与生成代码
  *_test.go           # 各文件对应的单元测试
```

---

## 路线图与已知限制

如需了解当前实现的细节问题、改进建议与优先级，可参考同目录下的 `code-review.md`。主要包含：

- `CacheOptions.MaxBytes` 当前按条数生效，与字段名语义不完全一致。
- `Client` 在自建 etcd client 时存在连接泄漏隐患，建议外部统一注入 etcd client。
- `ConHashMap` 后台再平衡 goroutine 缺少显式关闭入口。
- `SingleFlightGroup` 未对 `fn` 的 panic 做 recover，需调用方在 Getter 中自行兜底。
- gRPC 默认未启用 TLS / 鉴权，生产部署需补充 interceptor。
- `ServerOptions.EtcdEndpoints` 默认 `localhost:2379`，多环境部署必须覆盖。

欢迎提交 Issue 与 PR 一起改进。

---

## License

参见仓库根目录 `LICENSE`。
