# lark_rpc 代码评审

本文档对 lark_rpc 框架进行系统性的代码评审。lark_rpc 是一个基于 TCP 的 RPC 框架，公共 API 风格对齐 grpc-go，并自带了请求多路复用、流式调用、可插拔编解码、连接池、熔断器、令牌桶限流、负载均衡和 etcd 服务发现等能力。

模块路径：github.com/hangtiancheng/lark-go/lark_rpc

源码根目录：lark_rpc/

## 一、目录与模块总览

仓库内部按"对外 API"与"内部实现"两层组织，分层清晰，便于演进。

```
lark_rpc/
├── pkg/
│   ├── rpc/        对外公共 API，风格对齐 grpc-go
│   │   ├── rpc.go      类型别名、CodecJSON / CodecProto、NewRegistry
│   │   ├── server.go   Server、NewServer、Register、Serve、GracefulStop、Stop
│   │   ├── client.go   ClientConn、Dial、Invoke、NewStream、Close
│   │   └── stream.go   ServerStream / ClientStream 类型别名
│   └── api/        示例服务（Arith），用于演示与单元测试
└── internal/
    ├── server/         TCP 服务端、反射分发、服务端流
    ├── client/         注册中心模式客户端、熔断、限流、负载均衡组合
    ├── transport/      TCP 连接、连接池、异步 Future、客户端流
    ├── protocol/       线协议：Header、Message、编码解码
    ├── codec/          Codec 接口、JSON、Protobuf、Gzip 压缩
    ├── breaker/        三态熔断器
    ├── limiter/        令牌桶限流器
    ├── load_balance/   LoadBalancer 接口、RoundRobin、Random、WeightedRR
    ├── registry/       基于 etcd 的服务注册与发现
    └── stream/         ServerStream / ClientStream 接口定义
```

设计意图上有两条清晰的边界：

1. pkg/rpc 只暴露最小可用 API（NewServer / Dial / Invoke / NewStream），并通过 type alias 把 codec、registry、load_balance 三个内部包的类型对外重新导出，避免业务层直接 import internal。
2. internal/\* 之间互相依赖，但通过 stream 这个最小接口包打破了 server 与 transport 之间的循环依赖，保证 server 与 client 都不需要知道对方的存在。

## 二、公共 API 评审（pkg/rpc）

### 2.1 Server

文件：pkg/rpc/server.go

`Server` 是对 `internal/server.Server` 的薄包装，对外暴露的 API 与 grpc-go 完全一致：

```go
func NewServer(opts ...ServerOption) *Server
func (s *Server) Register(name string, service interface{})
func (s *Server) Serve(lis net.Listener) error
func (s *Server) GracefulStop()
func (s *Server) Stop()
```

设计要点：

- 构造函数只接受 Option，不接受地址，监听器由调用方传入 Serve，这一点与 grpc-go 的 `s.Serve(lis)` 模式一致，方便业务自定义监听（如 TLS、Unix Socket、test fixture 用 net.Listen("tcp", "127.0.0.1:0")）。
- `NewServer` 内部调用 `internal_server.NewServer`，若选项校验失败直接 panic。这是合理的"快速失败"策略，因为选项错误属于编程错误而非运行时错误。
- 关停分 `GracefulStop`（等待存量连接结束）与 `Stop`（立刻关闭所有连接）两种，语义同 grpc-go。

可改进点：

- `ServerOption` 目前只有 `WithCodec` 一个，缺少超时、最大连接数、最大消息长度、TLS、拦截器等常见配置项，但作为最小可用版本已经够用。

### 2.2 ClientConn

文件：pkg/rpc/client.go

`ClientConn` 是一个统一的客户端入口，同时支持静态地址直连和注册中心发现：

```go
func Dial(target string, opts ...DialOption) (*ClientConn, error)
func (cc *ClientConn) Invoke(ctx, service, method string, args, reply interface{}) error
func (cc *ClientConn) NewStream(ctx, service, method string, args interface{}) (ClientStream, error)
func (cc *ClientConn) Close() error
```

设计要点：

- 模式切换通过 `connMode` 内部枚举（modeStatic / modeRegistry）实现。当 `WithRegistry` 被传入时进入 registry 模式，target 参数被忽略；否则按 target 创建直连连接池。这与 grpc-go 通过 resolver scheme（"dns:///"、"passthrough:///"）切换 resolver 的方式精神一致。
- 默认超时 5 秒，默认编解码为 JSON，与示例期望一致。
- 静态模式下直接使用 `transport.NewConnectionPool(target, 0, 1)`，注册中心模式则委托给 `internal/client.Client`。

可改进点：

- 静态模式与注册中心模式在 `Invoke`/`NewStream` 的实现里走的是两条独立路径，超时与压缩的处理细节略有重复（静态分支自带 `context.WithTimeout`，而注册中心分支由 `internal/client` 在 `Invoke` 内部再加一次超时），未来若加入 Interceptor 这类横切关注点会比较麻烦。可以考虑把"取一个 conn、序列化、发送、等结果"这条主路径抽到一个内部接口，让两种模式只负责"提供一个 \*TCPClient"。
- `Close` 永远返回 nil，没有把内部错误透出来。

### 2.3 Stream

文件：pkg/rpc/stream.go

公共流接口是对 `internal/stream` 的 type alias，定义极简：

```go
type ServerStream = stream.ServerStream
type ClientStream = stream.ClientStream
```

两个接口只暴露最核心的能力：

- ServerStream：`Send(msg) error`、`Context() context.Context`
- ClientStream：`Recv(msg) error`、`Context() context.Context`

设计上仅支持服务端流（server-streaming），缺少 grpc-go 的 client-streaming 和 bi-directional streaming。这一点与协议层 `StreamFlag` 的实现一致——目前的帧格式没有为"客户端发起的后续流帧"留位置。

## 三、内部实现评审

### 3.1 协议层 protocol

文件：internal/protocol/header.go、message.go

线协议采用长度定界的二进制格式：

```
+--------+-----------+---------+--------+--------+
| Magic  | HeaderLen | BodyLen | Header | Body   |
| 2 byte | 4 byte    | 4 byte  | N byte | M byte |
+--------+-----------+---------+--------+--------+
```

- Magic 固定 0x1234，用于在字节流中重新对齐（PacketBuffer 在校验失败时会把 buf 前移 1 字节继续尝试，是一种朴素但实用的恢复策略）。
- HeaderLen / BodyLen 都是大端 4 字节无符号整数，单包上限受 `math.MaxInt` 限制，Decode 阶段会显式校验，避免越界。
- Header 本身被序列化为 JSON，再放到字节流里。这种"Header 用 JSON、Body 可切换 Codec"的混合方案对调试很友好（抓包就能看到方法名和错误），但带来了额外开销，每条消息都会触发一次 `codec.New(codec.JSON)` 的 map 查找加 sync.RWMutex 加锁。可以在 protocol 包内缓存 headerCodec 单例，避免反复构造。
- Body 在 Header.Compression 非 None 时会经过 gzip 压缩。当前所有写出帧（server handler、serverStream、client invoke、client stream）都强制 `Compression: codec.CompressionGzip`，没有按消息大小动态判断，对小消息会得不偿失（gzip 头本身 18 字节，几乎一定会让 10 字节级别的消息变大）。

StreamFlag 设计：

```go
StreamNone  = 0  // 普通响应
StreamData  = 1  // 流帧
StreamEnd   = 2  // 流正常结束
StreamError = 3  // 流异常结束
```

四种状态足够支撑服务端流，但要扩展到双向流就需要扩展该枚举并区分方向。

### 3.2 编解码 codec

文件：internal/codec/codec.go、json.go、protobuf.go、compress.go

编解码采用注册表 + 工厂模式，符合可扩展原则：

- `Codec` 接口只有 Marshal / Unmarshal 两个方法。
- 通过 `Register(Type, Factory)` 在 init 阶段注册 JSON（Type=1）和 Proto（Type=2）。重复注册会 panic，符合"快速失败"的设计。
- `New(Type)` 每次都会构造一个新实例。由于 JSONCodec / protoCodec 都是无状态的，可以做成单例 reuse，减少分配。
- 压缩器同样走注册表，目前只有 GzipCompressor 一种实现，留好了扩展位（zstd / snappy 等）。

需要注意：

- proto codec 强制要求 v 实现 `proto.Message`，否则返回 `not proto.Message`，但当前所有客户端发送路径都没有先把 args 转换或校验为 proto.Message，业务一旦选了 Proto 编码就必须自己保证类型正确，否则会在 send 阶段失败。

### 3.3 传输层 transport

#### TCPConnection

文件：internal/transport/tcp_connection.go

- 用 `bufio.Reader` 加自定义 `PacketBuffer` 解决 TCP 粘包/拆包。Read 流程是"先尝试从 buffer 取一个完整帧，取不到就再 read 一次"，是经典的边读边拆模式。
- Write 用 writeMu 串行化，避免多 goroutine 并发写同一连接导致帧交错。注意到 Close 时调用了 `SetLinger(0)`，会触发 TCP RST 立即关闭连接，这在 server graceful stop 路径上会导致客户端读到 connection reset 而不是 EOF，可能影响优雅停机的体验。

#### TCPClient

文件：internal/transport/tcp_client.go

这是整个传输层最关键的组件，负责"一个 TCP 连接上多路复用多个 RPC 请求"。核心数据结构：

- `pending sync.Map`：RequestID -> \*Future，存放还未拿到响应的同步调用。
- `streams sync.Map`：RequestID -> \*ClientStreamConn，存放尚未结束的流。
- `seq uint64`：用 `atomic.AddUint64` 自增分配请求 ID。

readLoop 中根据 `StreamFlag` 分发：

- StreamData：找到对应 stream，把 body push 进去。
- StreamEnd：把 stream 从 map 删除并 close。
- StreamError：删除并报错。
- 默认（StreamNone）：从 pending 取出 future 完成它。

设计要点：

- 一旦 readLoop 报错（远端关闭/网络错误），通过 `fail(err)` 统一收尾，使用 `atomic.CompareAndSwapInt32` 保证 close 只发生一次，然后遍历 pending 和 streams 把所有等待者唤醒，避免 goroutine 泄漏。这是非常正确的处理。
- SendAsync / SendStream 都在写之前先把 future/stream 注册到 map 里，再做 write，保证不会因为响应比写回更快返回（不太可能但理论存在）而错过响应。
- `nextSeq` 从 1 开始（AddUint64 先加再返回），永远不会用 0，避免与 protocol 默认值冲突。

可改进点：

- 连接级别没有心跳/keepalive，只能依赖 OS TCP keepalive，弱网环境下连接半死可能延迟很久才被发现。
- `pending` 没有超时清理，超时控制完全依赖调用方的 `GetResultWithContext`。如果调用方忘了传 ctx 超时，对应 future 会永远挂在 map 里直到 fail 触发。

#### ConnectionPool

文件：internal/transport/tcp_connection_pool.go

- `maxIdle` 参数实际未使用（构造函数收下但没保存）。
- `Acquire` 流程：在 mu 保护下，若连接数 < maxActive 则新建；否则按 next 索引轮询找一个未 closed 的复用；都不行就再新建一个。
- pkg/rpc 和 internal/client 在创建池时都传 `(addr, 0, 1)`，意味着 maxActive=1，整个连接池实际只持有一条 TCP 连接，所有请求通过 multiplexing 跑在同一条连上。这是符合 RPC 多路复用思路的，但 maxActive 暴露成参数却又被硬编码为 1，容易让人误读。
- 整个 Acquire 全程持锁，包括 `net.DialTimeout`（在 newTCPClient 里）。在新建连接时，其它 goroutine 全部要等到这次 dial 完成（最多 5s）才能往下走。生产环境若被这一刻撞上会导致明显的尾延迟。

#### Future

文件：internal/transport/future.go

异步结果容器，提供 `Done(res, err)` 让 readLoop 推送结果，提供 `GetResult / GetResultWithContext / Wait / WaitWithContext / WaitWithTimeout / OnComplete / IsDone / DoneChan` 多种消费方式。

- 用 `complete bool` 加 close(done) 双重保护，多次 Done 是幂等的。
- OnComplete 注册时若已 complete，会同步触发回调；否则保存等 Done 时调用。这种"先注册先生效、后注册立即触发"的模式非常友好。
- 内部 codec 默认 JSON，可以通过 `NewFutureWithCodec` 注入。这一点与传输上"Body 可切 Codec"的设计保持了一致。

#### ClientStreamConn

文件：internal/transport/stream.go

- 内部 `ch chan streamFrame`，缓冲 64。如果生产端（readLoop push）远快于消费端（业务 Recv），并不会丢帧而是 push 时阻塞在 select，直到 ctx 取消才返回。这种"背压传到网络层"的策略对正确性是好的，但代价是 readLoop 会被某一个 slow consumer 阻塞，从而拖累同一连接上的其它请求/流。在多路复用前提下这是一个潜在风险点。
- End / Error 用 sync.Once 保证只发送一次终止帧到 channel，下游 Recv 会得到 io.EOF 或具体错误。
- Recv 在 ctx.Done 时返回 ctx.Err，但已经发出的 push 不会被取消、仍占着 channel 容量。

### 3.4 服务端 server

#### Server

文件：internal/server/server.go

- 用 `services map[string]interface{}` 持有所有注册的服务实例，Register / Serve 之间的并发用单一 `mu` 保护。
- Serve 是经典 Accept-then-go 循环，每条 accept 出来的连接包装成 TCPConnection 并放进 `conns` 集合追踪，goroutine 退出时再从集合中删除。
- 限流器固定 10000 QPS（`limiter.NewTokenBucket(10000)`），且作用于"每条连接的每个请求"——也就是全 Server 共享同一个桶。这没有暴露为 Option，业务无法调整。
- Handle 拿到 msg 后先过 limiter，再加 mu 读 services，最后交给 handler.Process。读 services 用 mu 是为了对应 Register 的写，但同步路径上每次都加锁会成为热点，可以改成 atomic.Pointer 或者 RWMutex。
- GracefulStop 关闭 listener、等待存量 goroutine、停止 limiter；Stop 在此基础上额外强制 close 所有 conn。两个方法都用 sync.Once 保证幂等。

#### Handler

文件：internal/server/handler.go

`Handler.invoke` 用反射区分三类签名：

1. grpc-go 风格：`Method(ctx context.Context, req *T) (*R, error)`，numIn==2, numOut==2，in[0] 实现 context.Context，in[1] 是指针，out[1] 实现 error。
2. 流式：`Method(req *T, stream ServerStream) error`，numIn==2, numOut==1，in[1] 实现 stream.ServerStream。
3. net/rpc 兼容：`Method(req *T, reply *R) error`，numIn==2, numOut==1，in[1] 是指针。

判断逻辑里值得注意的细节：

- grpc-go 风格识别依赖 `methodType.In(0).Implements(contextType)`，但 context.Context 本身就是接口，且函数参数声明为 `context.Context`，类型即为接口本体。`Implements` 在这种情形下是工作的，但更标准的写法是 `methodType.In(0) == contextType`。
- 流式识别用 `secondParam.Implements(serverStreamType)`，由于业务方法的第二个参数通常声明为 `ServerStream` 接口本体，这里也是 OK 的。
- grpc-go 风格里若 `results[0].IsNil()` 直接返回 (nil, false, nil)，意味着即使方法返回 nil reply 也算成功。Handler 上层在 result==nil 时会发回一个空 Body 的响应，行为正确。

writeError 把错误写进 Header.Error 字段，客户端 readLoop 看到 `Header.Error != ""` 就走错误分支。这种"控制信息走 Header、业务数据走 Body"的拆分干净简洁。

#### serverStream

文件：internal/server/stream.go

- Send 直接走 conn.Write，每条 stream 帧都打 StreamData，结束打 StreamEnd，异常打 StreamError。
- 没有显式的 Recv 端配合（因为目前只支持 server-streaming，客户端发完首帧后只读不写）。

### 3.5 客户端 client

#### Client

文件：internal/client/client.go、invoke.go、option.go、resolver.go

`internal/client.Client` 是注册中心模式的核心实现，组合了：

- `registry.Registry`：服务发现
- `load_balance.LoadBalancer`：实例选择（默认 RoundRobin）
- `limiter.TokenBucket`：客户端侧限流，默认 10000
- `pools sync.Map`：地址 -> ConnectionPool
- `breaker sync.Map`：service|addr -> CircuitBreaker

调用链 invoke：

1. limiter.Allow，未通过直接 `rate limit exceeded`。
2. getAddr：通过 registry 发现 + LB 选址。
3. getBreaker：按 `service|addr` 拿到熔断器；未通过返回 `circuit breaker open`。
4. getPool(addr).Acquire 拿连接。
5. codec.Marshal(args)。
6. conn.SendAsyncWithCodec 发送并返回 Future。
7. 注册 OnComplete 回调，将结果反馈给熔断器（RecordSuccess / RecordFailure）。

`Invoke` 包了一层 `context.WithTimeout(ctx, c.timeout)`，把请求超时统一兜底。`InvokeStream` 路径相同但走 SendStream，并且没有结合熔断回调（只有 send 失败那一刻 RecordFailure，流中途出错不计入），这是当前实现的一个偏差。

#### resolver

文件：internal/client/resolver.go

- `getAddr`：每次调用都会触发 `registry.Discover(service)`，由于 Discover 内部已经做了"首次拉取后建立 watch + 缓存"，所以从第二次开始走的是本地 map 拷贝，开销可控。
- `getBreaker`：熔断器粒度是 service|addr，意味着同一服务的不同实例独立熔断，符合"熔断只该惩罚不健康的那台机器"的直觉，但参数硬编码（windowSize=10, threshold=0.6, openTimeout=5s），没有暴露给业务调整。

### 3.6 熔断 breaker

文件：internal/breaker/breaker.go

经典三态实现（Closed / Open / HalfOpen）：

- Closed：累计 success/failure，当总数达到 windowSize 时按比例判断是否进 Open。
- Open：从最后一次状态变化起计时，超过 openTimeout 进 HalfOpen 并放行一个探测请求。
- HalfOpen：只允许一个探测请求通过（halfOpenProbe 标记位）；探测成功转 Closed，探测失败回 Open。

可改进点：

- HalfOpen 阶段只放过 1 个请求，串行化排队的强度很大。grpc-go 的常见实现是 HalfOpen 时按一定速率/窗口放行 N 个请求。
- 没有暴露 State 变化的事件回调，监控接入困难。

### 3.7 限流 limiter

文件：internal/limiter/token_bucket.go

最小实现：每秒整桶填满 `rate` 个 token，Allow 取一个。

- 不是平滑令牌桶（不会按 rate / 时间增量补充），是"满秒重置"。在 burst 场景下表现不如标准实现。
- Stop 用 sync.Once 保证后台 goroutine 只被停一次。

### 3.8 负载均衡 load_balance

文件：internal/load_balance/round_robin.go、random.go、weighted_rr.go

- RoundRobin：`atomic.AddUint64(&idx, 1) - 1`，无锁无状态。
- Random：用 sync.Mutex 保护一个 `rand.Rand`，每秒系统调用一次。可以考虑用 math/rand/v2 的 PCG（Go 1.22+）取消加锁。
- WeightedRR：Nginx 风格的"平滑加权轮询"，正确性 OK，但权重列表与传入的实例列表是按下标对齐的，意味着 instances 的顺序必须稳定。当前 `registry.copyInstances` 是对 map 做 range，每次顺序都可能不同，把 WeightedRR 和 registry 直接组合会出问题。WeightedRR 没人这样组合，但作为公共 API 暴露给业务时这是个坑。

### 3.9 服务注册 registry

文件：internal/registry/registry.go

基于 etcd v3：

- Register：Grant 一个 TTL lease，Put 写入键 `<prefix><service>/<addr>` 并附带 lease，KeepAlive 后台续约。
- Discover：第一次访问触发 initService，先全量拉一次，再起一个 watch goroutine 监听后续变更。
- watch：监听 Put / Delete 事件并更新本地 map，watch 退出时通过 `time.After(1s)` 后重连，对 etcd 抖动有一定鲁棒性。

可改进点：

- Register 内部 KeepAlive 的回包 goroutine 只是简单丢弃，若 lease 过期会失去续约能力，但调用方完全不知道。应该在 lease 失效时尝试重新 Grant + Put，或者至少把错误透出来。
- copyInstances 持读锁后 range map，输出顺序不稳定（前面提到 WeightedRR 的隐患来源）。
- prefix 固定为 `/github.com/hangtiancheng/lark-go/lark_rpc/services/`，多租户/多环境场景下没法隔离，建议加入 namespace 参数。
- `Instance` 结构只保留 `Addr`，没有携带 weight、metadata、health 等扩展字段；与 WeightedRR 的权重数组解耦也使得"按 etcd 数据动态计算权重"变得不可能。

## 四、并发与正确性专项

1. Server.Handle 的循环里，conn.Read 出错只是 return（关闭连接），没有区分 EOF 与其他错误，日志缺失。这在排查"客户端怎么突然断了"时不够友好。
2. TCPClient.readLoop 拿到响应后从 pending 删除并 Done，全程无锁，正确依赖 sync.Map 的原子语义。
3. transport.ClientStreamConn 用单个 channel 串行 push，readLoop 在 push 时若 channel 满会阻塞，这会拖累同一 TCPClient 上其它 RPC。建议要么扩大 buffer，要么 push 时尝试 select default 丢弃旧帧或断开 stream。
4. ConnectionPool.Acquire 全程持锁 + 在锁内 Dial，是潜在的瓶颈点。
5. Server.limiter 是全 Server 共享单桶，限的是"全服务总 QPS"，与"每客户端/每方法限流"语义不同，注释里没有澄清。
6. CircuitBreaker.RecordSuccess 在 Closed 态会调 `resetClosedWindowIfReady`，逻辑里又判断了 rate >= threshold 并 toOpen，看似在"成功"路径里也能把状态推到 Open，但因为只在 total 达到 windowSize 时才检查，且 failureCount 此时不可能 >= threshold \* total（否则上一次 RecordFailure 就已经 toOpen），其实是安全的。但这个对称写法略冗余，可读性受影响。

## 五、可观测性与可运维性

- 全程几乎没有日志，唯一的两处 log.Println 在 GracefulStop 与 Stop 收尾。请求链路、熔断状态变化、限流拒绝、连接建立/断开都是黑盒。
- 没有 Metrics（计数器、Histogram）。
- 没有 Tracing 接入点（OpenTelemetry）。
- 没有 Interceptor / Middleware 机制，业务侧也无法自己注入埋点。

这对一个还在起步阶段的框架是可以接受的，但要进生产至少需要 Interceptor + Metrics 两件套。

## 六、测试覆盖

- internal/server：handler 反射分发的三种签名（grpc-go / net-rpc / 错误签名）、Process 错误响应、Server.Handle + Stop、Serve + Stop、并发 Register + Option 校验。
- internal/client：默认 codec、Option 校验、缺 registry 时的 InvokeAsync 报错、Timeout 设置生效。
- pkg/rpc：端到端的服务端流测试（正常流、中途出错、流过程中不影响 unary、context 取消），是整个仓库里最有价值的集成测试。
- 还有 breaker、limiter、load_balance、registry、protocol、codec、transport 的单元测试（按 \*\_test.go 文件存在性判断）。

整体测试覆盖足以保证主路径不出回归，但"双向并发请求 + 熔断/限流触发"这类组合场景尚未覆盖。

## 七、与 grpc-go API 对齐度

对齐良好的部分：

- NewServer + Option 模式、Serve(net.Listener)、GracefulStop / Stop。
- Dial(target, opts...) 返回 ClientConn，ClientConn.Invoke(ctx, method, args, reply) + ClientConn.NewStream(ctx, ...) + Close。
- 错误以 error 返回，超时通过 ctx 传递。
- 服务端方法的 grpc-go 签名 `(ctx, *T) (*R, error)`。

差距：

- 没有 protoc-gen-go-grpc 这种代码生成器，客户端只能通过字符串传 service/method，类型安全完全靠业务自己保证。
- 没有 Interceptor（Unary / Stream）。
- 流式只支持 server-streaming，缺少 client-streaming / bidi-streaming。
- 缺少 ServiceDesc / MethodDesc，反射调度需要每次走 reflect。
- 没有 Metadata（grpc-go 的 metadata.MD），无法传 trace id、auth token 等带外信息。

## 八、改进建议优先级

短期（低成本高收益）：

1. 把 protocol 内部反复 `codec.New(codec.JSON)` 改成包级缓存。
2. ConnectionPool.Acquire 把 Dial 移出锁外（用 sync.Once 或独立 dial mutex）。
3. 给 Compression 加个"小于 N 字节不压缩"的阈值，避免小消息反而变大。
4. ConnectionPool 删除未使用的 maxIdle 参数，或者实现它。
5. 给 registry.copyInstances 输出排序，避免与 WeightedRR 组合时翻车。

中期：

1. 引入 Interceptor 接口，让限流、熔断、日志、Metrics、Tracing 都通过它接入，砍掉 server 与 client 内部对 limiter 的硬编码。
2. 把熔断、限流参数暴露为 Option。
3. 完善 client-streaming / bidi-streaming，并相应扩展 StreamFlag。
4. Future / Stream 加超时清理，避免极端情况下的内存泄漏。

长期：

1. 提供代码生成器（基于 .proto 文件），生成强类型的 Client/Server stub。
2. 接入 OpenTelemetry，至少有 RED Metrics（Request / Error / Duration）。
3. registry 抽象出接口，让 etcd 不是唯一选项（Nacos、Consul、Zookeeper、Kubernetes Service）。

## 九、结论

lark_rpc 已经实现了一个"麻雀虽小五脏俱全"的 RPC 框架：协议层、传输层、编解码、连接池、熔断、限流、负载均衡、服务发现、流式调用悉数到位，公共 API 风格与 grpc-go 一致，新人上手成本很低。代码结构清晰，internal 与 pkg 分层合理，模块间耦合度可控。

主要不足集中在三方面：一是缺少 Interceptor 这种横切扩展点，导致很多能力被硬编码进 internal/client 与 internal/server；二是若干运维向能力缺失（日志、Metrics、Tracing、可调参数）；三是流式只支持 server-streaming。在把这三块补齐之后，lark_rpc 完全有能力作为业务侧自研服务间通信的基础设施使用。
