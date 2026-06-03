---
name: lark-rpc
description: >
  TCP-based RPC framework (lark_rpc module). Use this skill when working on RPC
  servers, clients, streaming, codec implementations, connection pools, circuit
  breakers, rate limiters, load balancers, etcd service registry, or any code that
  imports github.com/hangtiancheng/lark_rpc. Also use it when the user asks about
  the wire protocol, request multiplexing, or lark_rpc's internal architecture.
---

# lark_rpc

A TCP-based RPC framework with request multiplexing, streaming, pluggable codecs
(JSON / Protobuf / Gzip compression), connection pooling, circuit breakers, token
bucket rate limiting, load balancing, and etcd service discovery.

Module path: `github.com/hangtiancheng/lark_rpc`

Source root: `lark_rpc/`

## Architecture overview

```
pkg/rpc (public API)
  |-- Server   -> internal/server.Server
  |-- StaticClient (direct address)
  |-- RegistryClient = internal/client.Client (etcd-backed)
  |-- ServerStream / ClientStream = internal/stream interfaces

internal/
  |-- server/     Server, Handler (reflection-based dispatch), stream support
  |-- client/     Client with registry, load balancer, circuit breaker, rate limiter
  |-- transport/  TCPConnection, ConnectionPool, Future (async result)
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

```go
func NewServer(addr string) (*Server, error)
func (s *Server) Register(name string, service interface{})
func (s *Server) Start() error
func (s *Server) Addr() string
func (s *Server) Shutdown()
```

`Register` binds a service struct by name. The handler uses reflection to dispatch
RPCs to exported methods matching one of these signatures:

- `Method(args ArgType, reply *ReplyType) error` -- unary
- `Method(args ArgType, stream ServerStream) error` -- server-streaming

### StaticClient

Direct-address client with connection pool.

```go
func NewStaticClient(addr string, timeout time.Duration) (*StaticClient, error)
func (c *StaticClient) Invoke(ctx context.Context, service, method string, args, reply interface{}) error
func (c *StaticClient) InvokeStream(ctx context.Context, service, method string, args interface{}) (ClientStream, error)
func (c *StaticClient) Close()
```

Default timeout: 5 seconds. Uses JSON codec with Gzip compression.

### RegistryClient (etcd-backed)

```go
type RegistryClient = internal/client.Client

func NewRegistry(endpoints []string) (*Registry, error)
func NewClient(reg *Registry, opts ...ClientOption) (*RegistryClient, error)
```

Client options:

| Option                        | Description                  |
| ----------------------------- | ---------------------------- |
| `WithClientCodec(codec.Type)` | Set codec (JSON or Protobuf) |
| `WithClientTimeout(d)`        | RPC timeout                  |
| `WithClientLoadBalancer(lb)`  | Load balancing strategy      |

RegistryClient methods:

| Method                                                     | Description                |
| ---------------------------------------------------------- | -------------------------- |
| `Invoke(ctx, service, method, args, reply)`                | Synchronous unary RPC      |
| `InvokeAsync(ctx, service, method, args) -> *Future`       | Asynchronous unary RPC     |
| `InvokeStream(ctx, service, method, args) -> ClientStream` | Server-streaming RPC       |
| `Close()`                                                  | Close all connection pools |

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

## Internal components

### Wire protocol (internal/protocol)

Binary length-delimited format:

```
Header:
  RequestID    uint64
  ServiceName  string
  MethodName   string
  Compression  byte (0=none, 1=gzip)
  IsStream     bool
  IsStreamEnd  bool
  Error        string

Message = Header + Body ([]byte)
```

### Codec (internal/codec)

```go
type Codec interface {
    Marshal(v interface{}) ([]byte, error)
    Unmarshal(data []byte, v interface{}) error
}

type Type byte
const (
    JSON     Type = 0x01
    Protobuf Type = 0x02
)
```

Codecs are registered via `codec.Register(type, factory)` and created via `codec.New(type)`.
Gzip compression is applied at the transport layer based on the header's `Compression` field.

### Transport (internal/transport)

- `TCPConnection` -- wraps `net.Conn` with length-delimited read/write, supports
  async sends via `SendAsync` / `SendAsyncWithCodec` returning a `Future`
- `ConnectionPool` -- maintains a pool of `TCPConnection`s to a single address;
  `Acquire(ctx)` returns a connection, creating one if needed
- `Future` -- async result container with `GetResult(reply)`,
  `GetResultWithContext(ctx, reply)`, `DoneChan()`, `OnComplete(callback)`

### Circuit breaker (internal/breaker)

Three-state circuit breaker: Closed -> Open -> HalfOpen -> Closed.

```go
func NewCircuitBreaker(windowSize int, failureThreshold float64, openTimeout time.Duration) *CircuitBreaker
func (cb *CircuitBreaker) Allow() bool
func (cb *CircuitBreaker) RecordSuccess()
func (cb *CircuitBreaker) RecordFailure()
func (cb *CircuitBreaker) State() State
```

Transitions: when failure rate exceeds `failureThreshold` within a window of
`windowSize` calls, the breaker opens. After `openTimeout`, it moves to half-open
and allows one probe request.

### Rate limiter (internal/limiter)

Token bucket implementation.

```go
func NewTokenBucket(rate int) *TokenBucket
func (tb *TokenBucket) Allow() bool
func (tb *TokenBucket) Stop()
```

Refills at `rate` tokens/second up to a maximum of `rate` tokens.

### Load balancer (internal/load_balance)

```go
type LoadBalancer interface {
    Select([]registry.Instance) registry.Instance
}
```

Built-in strategies:

- `RoundRobin` -- sequential rotation
- `Random` -- uniform random selection
- `WeightedRoundRobin` -- weighted rotation based on instance weight

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
func NewHandler(services map[string]interface{}, opts ...HandlerOption) (*Handler, error)
func (h *Handler) Process(conn *TCPConnection, msg *Message, service interface{})
```

Handler options: `WithHandlerCodec(codec.Type)`.

The handler inspects the service struct for methods matching the expected signatures,
dispatches the call, serializes the result, and writes it back on the connection.
For streaming methods, it creates a `serverStreamImpl` that sends frames back
over the same connection.

## Typical usage

Server:

```go
type MathService struct{}

func (s *MathService) Add(args *AddArgs, reply *int) error {
    *reply = args.A + args.B
    return nil
}

srv, _ := rpc.NewServer(":0")
srv.Register("Math", &MathService{})
go srv.Start()
```

Static client:

```go
client, _ := rpc.NewStaticClient(srv.Addr(), 5*time.Second)
defer client.Close()

var result int
err := client.Invoke(ctx, "Math", "Add", &AddArgs{A: 1, B: 2}, &result)
```

Registry client:

```go
reg, _ := rpc.NewRegistry([]string{"localhost:2379"})
client, _ := rpc.NewClient(reg,
    client.WithClientTimeout(3*time.Second),
    client.WithClientLoadBalancer(&load_balance.WeightedRoundRobin{}),
)
defer client.Close()

var result int
err := client.Invoke(ctx, "Math", "Add", &AddArgs{A: 1, B: 2}, &result)
```

Streaming:

```go
// Server side
func (s *FeedService) Subscribe(args *SubArgs, stream rpc.ServerStream) error {
    for msg := range feed {
        if err := stream.Send(msg); err != nil {
            return err
        }
    }
    return nil
}

// Client side
stream, _ := client.InvokeStream(ctx, "Feed", "Subscribe", &SubArgs{Topic: "news"})
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
| `pkg/rpc/`               | `rpc.go`, `stream.go`                                                                    | Public API: Server, StaticClient, type aliases                  |
| `internal/server/`       | `server.go`, `handler.go`, `options.go`, `stream.go`                                     | TCP server, reflection dispatch, streaming                      |
| `internal/client/`       | `client.go`, `invoke.go`, `option.go`, `resolver.go`                                     | Client with breaker/limiter/LB, Invoke/InvokeAsync/InvokeStream |
| `internal/transport/`    | `tcp_connection.go`, `tcp_connection_pool.go`, `tcp_client.go`, `future.go`, `stream.go` | TCP transport, connection pool, async futures                   |
| `internal/protocol/`     | `header.go`, `message.go`                                                                | Wire format: header encoding, message framing                   |
| `internal/codec/`        | `codec.go`, `json.go`, `protobuf.go`, `compress.go`                                      | Codec interface, JSON/Protobuf impls, Gzip compression          |
| `internal/breaker/`      | `breaker.go`                                                                             | Three-state circuit breaker                                     |
| `internal/limiter/`      | `token_bucket.go`                                                                        | Token bucket rate limiter                                       |
| `internal/load_balance/` | `balancer.go`, `round_robin.go`, `random.go`, `weighted_rr.go`                           | Load balancing strategies                                       |
| `internal/registry/`     | `registry.go`                                                                            | etcd service registry                                           |
| `internal/stream/`       | `stream.go`                                                                              | ServerStream / ClientStream interfaces                          |
| `cmd/`                   | `server1/`, `server2/`, `bench1/`, `bench2/`                                             | Example servers and benchmarks                                  |
| `pkg/api/`               | `arith.go`                                                                               | Example Arith service for demos                                 |

## Dependencies

- `go.etcd.io/etcd/client/v3` -- service discovery
- `google.golang.org/protobuf` -- protobuf codec
