---
name: lark-rpc
description: >
  TCP-based RPC framework (lark_rpc module) with a grpc-go-style public API. Use this
  skill when working on RPC servers, clients, streaming, codec implementations, connection
  pools, circuit breakers, rate limiters, load balancers, etcd service registry, or any
  code that imports github.com/hangtiancheng/lark-go/lark_rpc. Also use it when the user
  asks about the wire protocol, request multiplexing, or lark_rpc's internal architecture.
---

# lark_rpc

A TCP-based RPC framework with a grpc-go-style public API, request multiplexing,
streaming, pluggable codecs (JSON / Protobuf / Gzip compression), connection pooling,
circuit breakers, token bucket rate limiting, load balancing, and etcd service discovery.

Module path: `github.com/hangtiancheng/lark-go/lark_rpc`

Source root: `lark_rpc/`

## Architecture overview

```
pkg/rpc (public API -- grpc-go style)
  |-- Server     (NewServer, Register, Serve, GracefulStop, Stop)
  |-- ClientConn (Dial, Invoke, NewStream, Close)
  |-- ServerStream / ClientStream type aliases

internal/
  |-- server/     Server, Handler (reflection-based dispatch), stream support
  |-- client/     Client with registry, load balancer, circuit breaker, rate limiter
  |-- transport/  TCPConnection, ConnectionPool, TCPClient, Future (async result)
  |-- protocol/   Header + Message wire format (length-delimited binary)
  |-- codec/      Codec interface + JSON, Protobuf, Gzip implementations
  |-- breaker/    Three-state circuit breaker (Closed/Open/HalfOpen)
  |-- limiter/    Token bucket rate limiter
  |-- load_balance/  LoadBalancer interface + RoundRobin, Random, WeightedRR
  |-- registry/   etcd-based service registry with watch
  |-- stream/     ServerStream / ClientStream interfaces
```

## Public API (pkg/rpc)

### Server

The public Server follows grpc-go conventions: the constructor takes options (not an
address), the caller creates the `net.Listener` and passes it to `Serve`, and shutdown
is split into `GracefulStop` (waits for active connections) and `Stop` (immediate).

```go
func NewServer(opts ...ServerOption) *Server

func (s *Server) Register(name string, service interface{})
func (s *Server) Serve(lis net.Listener) error
func (s *Server) GracefulStop()
func (s *Server) Stop()
```

Server options:

| Option         | Description            |
| -------------- | ---------------------- |
| `WithCodec(t)` | Set codec (JSON/Proto) |

`Register` binds a service struct by name. The handler uses reflection to dispatch
RPCs to exported methods matching one of these signatures:

- grpc-go style: `Method(ctx context.Context, req *T) (*R, error)` -- unary
- net/rpc style: `Method(req *T, reply *R) error` -- unary (backward compat)
- streaming: `Method(req *T, stream ServerStream) error` -- server-streaming

The handler distinguishes grpc-go vs net/rpc by output count (2 vs 1).

### ClientConn

Unified client connection, supporting two modes:

- Static mode (default): direct connection pool to a single target address
- Registry mode: etcd-based service discovery via `WithRegistry` option

```go
func Dial(target string, opts ...DialOption) (*ClientConn, error)

func (cc *ClientConn) Invoke(ctx context.Context, service, method string, args, reply interface{}) error
func (cc *ClientConn) NewStream(ctx context.Context, service, method string, args interface{}) (ClientStream, error)
func (cc *ClientConn) Close() error
```

Dial options:

| Option                 | Description                             |
| ---------------------- | --------------------------------------- |
| `WithTimeout(d)`       | RPC timeout (default: 5s)               |
| `WithDialCodec(t)`     | Set codec type (JSON or Protobuf)       |
| `WithRegistry(reg)`    | Enable registry mode (etcd discovery)   |
| `WithLoadBalancer(lb)` | Load balancing strategy (registry mode) |

When `WithRegistry` is provided, `Dial` ignores the target address and uses the
registry for service discovery. Otherwise it creates a direct connection pool to
the target.

### Streaming interfaces

```go
type ServerStream interface {
    Send(msg interface{}) error
    Context() context.Context
}

type ClientStream interface {
    Recv(msg interface{}) error
    Context() context.Context
}
```

### Re-exported types

```go
type CodecType = codec.Type
type Registry = registry.Registry
type Instance = registry.Instance
type LoadBalancer = load_balance.LoadBalancer

var CodecJSON = codec.JSON
var CodecProto = codec.PROTO

func NewRegistry(endpoints []string) (*Registry, error)
```

## Internal components

### Wire protocol (internal/protocol)

Binary length-delimited format: `[Magic(2)][HeaderLen(4)][BodyLen(4)][Header][Body]`

Magic: `0x1234`

```go
type Header struct {
    RequestID   uint64
    ServiceName string
    MethodName  string
    Error       string
    CodecType   CodecType
    Compression codec.CompressionType
    StreamFlag  StreamFlag   // StreamNone=0, StreamData=1, StreamEnd=2, StreamError=3
}
```

Headers are JSON-serialized. Body is optionally Gzip-compressed.

### Codec (internal/codec)

```go
type Codec interface {
    Marshal(v interface{}) ([]byte, error)
    Unmarshal(data []byte, v interface{}) error
}

type Type byte
const (
    JSON  Type = 0x01
    PROTO Type = 0x02
)

type CompressionType byte
const (
    CompressionNone CompressionType = 0
    CompressionGzip CompressionType = 1
)
```

### Transport (internal/transport)

- `TCPConnection` -- wraps `net.Conn` with `PacketBuffer` for frame extraction;
  `Read() (*Message, error)` and `Write(msg *Message) error`
- `TCPClient` -- manages a connection with async send/receive; multiplexes requests
  via `RequestID`; `SendAsync`/`SendAsyncWithCodec` return `*Future`;
  `SendStream` returns `*ClientStreamConn`
- `ConnectionPool` -- pool of `TCPClient` instances to a single address;
  `Acquire(ctx) (*TCPClient, error)` returns a client, creating if needed
- `Future` -- async result container with `GetResult(reply)`,
  `GetResultWithContext(ctx, reply)`, `DoneChan()`, `OnComplete(callback)`
- `ClientStreamConn` -- client-side stream with `Push(body)`, `End()`, `Error(err)`,
  `Recv(msg) error`; backed by a buffered channel (cap 64)

The `TCPClient.readLoop` demultiplexes responses by `RequestID` and `StreamFlag`:
`StreamData` pushes to the stream, `StreamEnd` closes it, `StreamError` reports an
error, and the default case resolves a pending `Future`.

### Circuit breaker (internal/breaker)

Three-state circuit breaker: Closed -> Open -> HalfOpen -> Closed.

```go
func NewCircuitBreaker(windowSize int, failureThreshold float64, openTimeout time.Duration) *CircuitBreaker
func (cb *CircuitBreaker) Allow() bool
func (cb *CircuitBreaker) RecordSuccess()
func (cb *CircuitBreaker) RecordFailure()
func (cb *CircuitBreaker) State() State
```

### Rate limiter (internal/limiter)

Token bucket implementation.

```go
func NewTokenBucket(rate int) *TokenBucket
func (tb *TokenBucket) Allow() bool
func (tb *TokenBucket) Stop()
```

### Load balancer (internal/load_balance)

```go
type LoadBalancer interface {
    Select([]registry.Instance) registry.Instance
}
```

Built-in: `RoundRobin`, `Random`, `WeightedRoundRobin`.

### Service registry (internal/registry)

etcd-based service registration and discovery.

```go
func NewRegistry(endpoints []string) (*Registry, error)
func (r *Registry) Register(ctx context.Context, name, addr string) error
func (r *Registry) Discover(ctx context.Context, name string) ([]Instance, error)
func (r *Registry) Watch(ctx context.Context, name string) (<-chan []Instance, error)
func (r *Registry) Close() error

type Instance struct {
    Name string
    Addr string
}
```

### Handler (internal/server)

Reflection-based RPC method dispatch.

```go
func NewHandler(s interface{}, opts ...HandleOption) (*Handler, error)
func (h *Handler) Process(conn *TCPConnection, msg *Message, service interface{})
```

The `invoke` method checks method signatures in order:

1. grpc-go style: `numIn==2, numOut==2, in[0]=context.Context, in[1]=*T, out[0]=*R, out[1]=error`
2. net/rpc streaming: `numIn==2, numOut==1, in[1] implements ServerStream`
3. net/rpc unary: `numIn==2, numOut==1, in[1]=*R (reply pointer)`

For streaming methods, `invoke` creates a `serverStream` that sends frames with
`StreamFlag=StreamData` and ends with `StreamEnd` or `StreamError`.

## Typical usage

Server:

```go
type MathService struct{}

func (s *MathService) Add(ctx context.Context, args *Args) (*Reply, error) {
    return &Reply{Result: args.A + args.B}, nil
}

server := rpc.NewServer()
server.Register("Math", &MathService{})

lis, _ := net.Listen("tcp", ":8080")
go server.Serve(lis)
defer server.GracefulStop()
```

Client (static):

```go
conn, _ := rpc.Dial(":8080", rpc.WithTimeout(5*time.Second))
defer conn.Close()

var reply Reply
err := conn.Invoke(ctx, "Math", "Add", &Args{A: 1, B: 2}, &reply)
```

Client (registry):

```go
reg, _ := rpc.NewRegistry([]string{"localhost:2379"})
conn, _ := rpc.Dial("", rpc.WithRegistry(reg), rpc.WithTimeout(3*time.Second))
defer conn.Close()

var reply Reply
err := conn.Invoke(ctx, "Math", "Add", &Args{A: 1, B: 2}, &reply)
```

Streaming:

```go
// Server side
func (s *FeedService) Subscribe(req *SubArgs, stream rpc.ServerStream) error {
    for msg := range feed {
        if err := stream.Send(msg); err != nil {
            return err
        }
    }
    return nil
}

// Client side
stream, _ := conn.NewStream(ctx, "Feed", "Subscribe", &SubArgs{Topic: "news"})
for {
    var msg Message
    if err := stream.Recv(&msg); err != nil {
        break
    }
    process(msg)
}
```

## File map

| Directory                | Files                                                                                    | Purpose                                                         |
| ------------------------ | ---------------------------------------------------------------------------------------- | --------------------------------------------------------------- |
| `pkg/rpc/`               | `server.go`, `client.go`, `rpc.go`, `stream.go`                                          | Public API: Server, ClientConn, Dial, type aliases              |
| `pkg/api/`               | `arith.go`                                                                               | Example Arith service (grpc-go style signatures)                |
| `internal/server/`       | `server.go`, `handler.go`, `options.go`, `stream.go`                                     | TCP server, reflection dispatch, server-side streaming          |
| `internal/client/`       | `client.go`, `invoke.go`, `option.go`, `resolver.go`                                     | Client with breaker/limiter/LB, Invoke/InvokeAsync/InvokeStream |
| `internal/transport/`    | `tcp_connection.go`, `tcp_connection_pool.go`, `tcp_client.go`, `future.go`, `stream.go` | TCP transport, connection pool, async futures, client streams   |
| `internal/protocol/`     | `header.go`, `message.go`                                                                | Wire format: header encoding, message framing                   |
| `internal/codec/`        | `codec.go`, `json.go`, `protobuf.go`, `compress.go`                                      | Codec interface, JSON/Protobuf impls, Gzip compression          |
| `internal/breaker/`      | `breaker.go`                                                                             | Three-state circuit breaker                                     |
| `internal/limiter/`      | `token_bucket.go`                                                                        | Token bucket rate limiter                                       |
| `internal/load_balance/` | `balancer.go`, `round_robin.go`, `random.go`, `weighted_rr.go`                           | Load balancing strategies                                       |
| `internal/registry/`     | `registry.go`                                                                            | etcd service registry                                           |
| `internal/stream/`       | `stream.go`                                                                              | ServerStream / ClientStream interfaces                          |

## Dependencies

- `go.etcd.io/etcd/client/v3` -- service discovery
- `google.golang.org/protobuf` -- protobuf codec
