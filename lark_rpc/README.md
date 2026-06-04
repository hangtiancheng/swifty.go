# lark_rpc

lark_rpc 是一个用 Go 编写的轻量级 RPC 框架，对外 API 风格对齐 [grpc-go](https://github.com/grpc/grpc-go)，底层走自定义 TCP 二进制协议，开箱即用地提供请求多路复用、服务端流、连接池、熔断、限流、负载均衡和基于 etcd 的服务发现等能力。

模块路径：`github.com/hangtiancheng/lark-go/lark_rpc`

## 特性

- API 对齐 grpc-go：`NewServer` / `Register` / `Serve` / `GracefulStop`、`Dial` / `Invoke` / `NewStream` / `Close`，迁移成本低
- 支持两种调用模式：静态地址直连，与基于 etcd 的注册中心模式
- 同一条 TCP 连接上多路复用任意数量的并发请求，单连接低开销
- 内置可插拔的 Codec：JSON（默认）与 Protobuf，并对 Body 支持 Gzip 压缩
- 服务端流（server-streaming）：用 `Send` / `Recv` 即可优雅地推送任意数量的帧
- 内置熔断器（三态：Closed / Open / HalfOpen），按 `service|addr` 维度独立隔离
- 内置令牌桶限流，服务端与客户端各一份
- 内置负载均衡：RoundRobin、Random、平滑加权 WeightedRR，可自定义
- 基于 etcd v3 的服务注册与发现，自动 Watch 实时感知实例变更
- 反射分发服务方法，同时兼容 grpc-go 风格与 net/rpc 风格的方法签名

## 安装

```bash
go get github.com/hangtiancheng/lark-go/lark_rpc
```

要求 Go 1.21 及以上。若使用注册中心模式还需要可访问的 etcd v3 集群。

## 快速开始

### 定义服务

服务端方法支持两种签名，框架通过反射自动识别：

```go
package api

import "context"

type Args struct {
    A int
    B int
}

type Reply struct {
    Result int
}

type Arith struct{}

// grpc-go 风格（推荐）
func (a *Arith) Add(_ context.Context, args *Args) (*Reply, error) {
    return &Reply{Result: args.A + args.B}, nil
}

// net/rpc 兼容风格
func (a *Arith) Mul(args *Args, reply *Reply) error {
    reply.Result = args.A * args.B
    return nil
}
```

### 启动服务端

```go
package main

import (
    "log"
    "net"

    "github.com/hangtiancheng/lark-go/lark_rpc/pkg/api"
    "github.com/hangtiancheng/lark-go/lark_rpc/pkg/rpc"
)

func main() {
    server := rpc.NewServer()
    server.Register("Arith", &api.Arith{})

    lis, err := net.Listen("tcp", ":8080")
    if err != nil {
        log.Fatal(err)
    }

    log.Println("lark_rpc server listening on :8080")
    if err := server.Serve(lis); err != nil {
        log.Fatal(err)
    }
}
```

优雅停机：

```go
server.GracefulStop() // 等待存量请求处理完
// 或
server.Stop()         // 立即关闭所有连接
```

### 调用客户端（静态直连）

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/hangtiancheng/lark-go/lark_rpc/pkg/api"
    "github.com/hangtiancheng/lark-go/lark_rpc/pkg/rpc"
)

func main() {
    conn, err := rpc.Dial("127.0.0.1:8080", rpc.WithTimeout(3*time.Second))
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    var reply api.Reply
    if err := conn.Invoke(context.Background(), "Arith", "Add", &api.Args{A: 1, B: 2}, &reply); err != nil {
        log.Fatal(err)
    }
    log.Println("1 + 2 =", reply.Result)
}
```

## 服务端流（Server Streaming）

服务端方法签名为 `Method(req *T, stream rpc.ServerStream) error`，逐帧 `Send`，正常 return 时框架自动发送结束帧，error 返回时自动发送错误帧。

服务端：

```go
type Chunk struct {
    Index   int
    Content string
}

type FeedRequest struct {
    Count int
}

type FeedService struct{}

func (s *FeedService) Generate(req *FeedRequest, stream rpc.ServerStream) error {
    for i := 0; i < req.Count; i++ {
        if err := stream.Send(&Chunk{Index: i, Content: fmt.Sprintf("chunk-%d", i)}); err != nil {
            return err
        }
    }
    return nil
}
```

客户端：

```go
stream, err := conn.NewStream(ctx, "Feed", "Generate", &FeedRequest{Count: 5})
if err != nil {
    log.Fatal(err)
}

for {
    var c Chunk
    if err := stream.Recv(&c); err != nil {
        if err == io.EOF {
            break
        }
        log.Fatal(err)
    }
    log.Println("recv:", c)
}
```

服务端流在同一条 TCP 连接上以独立 RequestID 多路复用，与其它 unary 调用互不阻塞。

## 注册中心模式（etcd）

服务端注册到 etcd：

```go
reg, err := rpc.NewRegistry([]string{"127.0.0.1:2379"})
if err != nil {
    log.Fatal(err)
}
// 注：服务端注册接口在 internal/registry 中，暴露后即可写：
// reg.Register("Arith", registry.Instance{Addr: "127.0.0.1:8080"}, 10)
```

客户端通过注册中心发现并调用：

```go
reg, _ := rpc.NewRegistry([]string{"127.0.0.1:2379"})

conn, err := rpc.Dial("",
    rpc.WithRegistry(reg),
    rpc.WithTimeout(3*time.Second),
    // 可选：自定义负载均衡策略，默认 RoundRobin
    // rpc.WithLoadBalancer(load_balance.NewRandom()),
)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

var reply api.Reply
err = conn.Invoke(context.Background(), "Arith", "Add", &api.Args{A: 3, B: 4}, &reply)
```

注册中心模式下，target 参数被忽略，地址由 LoadBalancer 在 Discover 返回的实例集合中实时选择。

## 编解码

默认使用 JSON。如需切换到 Protobuf，请在服务端与客户端都显式声明：

```go
server := rpc.NewServer(rpc.WithCodec(rpc.CodecProto))

conn, _ := rpc.Dial(addr, rpc.WithDialCodec(rpc.CodecProto))
```

Protobuf 模式下，所有 args / reply 必须实现 `proto.Message`。

Body 默认会做 Gzip 压缩，对中大体积的消息节省带宽明显；小消息体场景如有需要可在后续版本中通过参数关闭。

## 配置选项

服务端 Option：

- `rpc.WithCodec(rpc.CodecJSON | rpc.CodecProto)`：设置序列化协议

客户端 Dial Option：

- `rpc.WithTimeout(d time.Duration)`：单次 RPC 超时，默认 5 秒
- `rpc.WithDialCodec(t rpc.CodecType)`：序列化协议
- `rpc.WithRegistry(reg *rpc.Registry)`：启用注册中心模式
- `rpc.WithLoadBalancer(lb rpc.LoadBalancer)`：注册中心模式下的负载均衡策略

## 架构

```
┌──────────────────────────────────────────────────────────┐
│                     pkg/rpc (公共 API)                    │
│   Server  ClientConn  ServerStream  ClientStream         │
└─────────┬──────────────────────────────────────┬─────────┘
          │                                      │
┌─────────▼─────────┐                  ┌─────────▼─────────┐
│ internal/server   │                  │ internal/client   │
│  反射分发 / 流帧   │                  │ 熔断/限流/LB/连接池│
└─────────┬─────────┘                  └─────────┬─────────┘
          │                                      │
          └────────────────┬─────────────────────┘
                           │
                  ┌────────▼────────┐
                  │ internal/transport│
                  │ TCPClient / Pool │
                  │ Future / Stream  │
                  └────────┬─────────┘
                           │
                  ┌────────▼────────┐
                  │ internal/protocol │
                  │  Header + Message│
                  └────────┬─────────┘
                           │
                  ┌────────▼────────┐
                  │  internal/codec  │
                  │  JSON / Proto    │
                  │  Gzip Compress   │
                  └──────────────────┘
```

线协议（每帧）：

```
+--------+-----------+---------+----------------+--------------+
| Magic  | HeaderLen | BodyLen |   Header(JSON) | Body(bytes)  |
| 2 byte | 4 byte    | 4 byte  |     N byte     |    M byte    |
+--------+-----------+---------+----------------+--------------+
```

Header 中携带 RequestID、ServiceName、MethodName、CodecType、Compression、StreamFlag、Error 等控制信息。RequestID 用于在同一连接上做请求/响应多路复用，StreamFlag 区分普通响应（None）、流数据帧（Data）、流结束（End）、流错误（Error）。

## 项目结构

```
lark_rpc/
├── pkg/
│   ├── rpc/                公共 API（对齐 grpc-go 风格）
│   │   ├── rpc.go          类型别名 / NewRegistry / Codec 常量
│   │   ├── server.go       Server、NewServer、Serve、GracefulStop、Stop
│   │   ├── client.go       ClientConn、Dial、Invoke、NewStream
│   │   └── stream.go       ServerStream / ClientStream 类型别名
│   └── api/                示例服务（Arith）与示例消息类型
└── internal/
    ├── server/             TCP 服务端 + 反射分发 + 流支持
    ├── client/             注册中心模式客户端 + 熔断/限流/LB
    ├── transport/          TCPConnection、ConnectionPool、TCPClient、Future、Stream
    ├── protocol/           Header、Message、编码解码
    ├── codec/              Codec 接口与 JSON / Protobuf / Gzip 实现
    ├── breaker/            三态熔断器
    ├── limiter/            令牌桶限流
    ├── load_balance/       RoundRobin / Random / WeightedRR
    ├── registry/           基于 etcd v3 的服务注册与发现
    └── stream/             ServerStream / ClientStream 接口定义
```

## 服务方法签名

`Register(name, service)` 之后，框架通过反射识别 `service` 上的导出方法，支持以下三种签名：

```go
// 1) grpc-go 风格 unary（推荐）
func (s *Svc) Method(ctx context.Context, req *T) (*R, error)

// 2) net/rpc 风格 unary（向后兼容）
func (s *Svc) Method(req *T, reply *R) error

// 3) 服务端流
func (s *Svc) Method(req *T, stream rpc.ServerStream) error
```

不匹配上述任意签名的方法会被忽略，调用时返回 `unsupported method signature` 错误。

## 错误处理

业务方法返回的 error 会被序列化到 Header.Error 字段，客户端 `Invoke` 直接以 `errors.New(headerError)` 形式返回。框架层错误的常见取值：

- `service not found: <Service>`：未注册的服务名
- `method not found: <Service>.<Method>`：方法在 service 实例上不存在
- `unsupported method signature: <Service>.<Method>`：方法签名不被识别
- `rate limit exceeded`：被令牌桶限流
- `circuit breaker open`：被熔断器拦截
- `no instance available`：注册中心未发现可用实例
- `connection closed`：发送时连接已关闭

## 运行测试

```bash
cd lark_rpc
go test ./...
```

端到端的服务端流集成测试位于 `pkg/rpc/stream_test.go`，覆盖了正常流、中途出错、流与 unary 混合调用、context 取消等场景。

## 路线图

- 拦截器（Unary / Stream Interceptor），把限流、熔断、日志、Metrics、Tracing 接入点统一
- 客户端流与双向流
- 把熔断 / 限流参数外露为 Option
- 接入 OpenTelemetry 与 Prometheus
- 基于 .proto 的代码生成器，提供强类型 stub
- 注册中心抽象，支持 Nacos / Consul / Kubernetes Service 等

## 依赖

- go.etcd.io/etcd/client/v3 —— 服务发现
- google.golang.org/protobuf —— Protobuf 编解码

## 许可证

详见仓库根目录 LICENSE 文件。
