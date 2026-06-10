---
name: lark-rpc
description: >
  TCP-based RPC framework for Go (module path:
  github.com/hangtiancheng/lark-go/lark_rpc). Key exported symbols: rpc.Dial,
  rpc.NewServer, rpc.NewRegistry, ClientConn, Server, ServerStream,
  ClientStream, DialOption, ServerOption, CodecType, CodecJSON, CodecProto,
  Instance, LoadBalancer, WithTimeout, WithDialCodec, WithRegistry,
  WithLoadBalancer, WithCodec. Use when writing or modifying Go code that
  imports github.com/hangtiancheng/lark-go/lark_rpc, uses rpc.NewServer /
  Register / Serve / GracefulStop / Stop, rpc.Dial / ClientConn.Invoke /
  ClientConn.NewStream / ClientConn.Close, service discovery via
  rpc.NewRegistry, the ServerStream / ClientStream interfaces, or questions
  about the binary wire protocol (Magic 0x1234, HeaderLen, BodyLen framing),
  StreamFlag (StreamNone / StreamData / StreamEnd / StreamError), request
  multiplexing by RequestID, the reflection-based method dispatcher (grpc-go
  and net/rpc signatures), server-streaming handlers, circuit-breaker /
  rate-limiting / load-balancing internals, registry vs static mode, or
  migrating net/rpc or grpc-go code to lark_rpc. Skip for grpc-go itself,
  Connect-Go, Twirp, net/rpc code that does not import lark_rpc, HTTP / SSE /
  WebSocket work (use lark-http instead), or pure Protobuf schema questions
  unrelated to lark_rpc dispatch.
---

# lark_rpc

A TCP-based RPC framework with a grpc-go-style public API that provides request multiplexing on a single connection, server-streaming, pluggable JSON and Protobuf codecs with mandatory Gzip body compression, connection pooling, three-state circuit breaking, token-bucket rate limiting, three built-in load-balancing strategies (round-robin, random, smooth-weighted round-robin), and etcd v3 service registration and discovery. The module path is `github.com/hangtiancheng/lark-go/lark_rpc`, the public API lives in `pkg/rpc`, and all implementation details reside under `internal/` which Go prevents external packages from importing.

## Architecture overview

```
                          +------------------+
                          |   pkg/rpc        |  Public API surface
                          |  (Server,        |
                          |   ClientConn,    |
                          |   Dial, types)   |
                          +--------+---------+
                                   |
              +--------------------+--------------------+
              |                                         |
   +----------v-----------+              +-------------v-----------+
   | internal/server      |              | internal/client         |
   |  Server loop         |              |  Registry-mode client   |
   |  Handler (reflect)   |              |  Breaker + Limiter + LB |
   |  serverStream        |              |  Invoke / InvokeStream  |
   +----------+-----------+              +-------------+-----------+
              |                                         |
              |    +-------------------+                |
              +--->| internal/transport |<--------------+
              |    |  TCPConnection    |
              |    |  TCPClient        |
              |    |  ConnectionPool   |
              |    |  Future           |
              |    |  ClientStreamConn |
              |    +--------+----------+
              |             |
              v             v
   +----------+-------------+----------+
   | internal/protocol                 |
   |  Header, Message, Encode, Decode  |
   |  Magic = 0x1234                   |
   +----------+------------------------+
              |
              v
   +----------+------------------------+
   | internal/codec                    |
   |  Codec interface, JSON, Protobuf  |
   |  CompressionType, Gzip           |
   +-----------------------------------+

   +---------------------+    +---------------------+    +-------------------------+
   | internal/breaker    |    | internal/limiter    |    | internal/load_balance   |
   |  CircuitBreaker     |    |  TokenBucket        |    |  RoundRobin, Random,    |
   |  (Closed/Open/Half) |    |  (per-second refill)|    |  WeightedRR             |
   +---------------------+    +---------------------+    +-------------------------+

   +---------------------+    +---------------------+
   | internal/registry   |    | internal/stream     |
   |  etcd v3 register/  |    |  ServerStream iface |
   |  discover/watch     |    |  ClientStream iface |
   +---------------------+    +---------------------+
```

Two strict structural boundaries:

1. `pkg/rpc` re-exports a minimal set of types from `internal/codec`, `internal/registry`, and `internal/load_balance` via type aliases. Downstream code must never import `internal/*` directly.
2. `internal/stream` exists solely to break a potential cyclic dependency between `internal/server` and `internal/transport`. Both packages reference `stream.ServerStream` / `stream.ClientStream`, never each other.

## Core types

### Package pkg/rpc -- exported types

```go
// Type aliases re-exported from internal packages.
type CodecType    = codec.Type
type Registry     = registry.Registry
type Instance     = registry.Instance
type LoadBalancer = load_balance.LoadBalancer
type ServerStream = stream.ServerStream
type ClientStream = stream.ClientStream
```

### Package pkg/rpc -- exported constants and variables

```go
var CodecJSON  = codec.JSON   // codec.Type value 1
var CodecProto = codec.PROTO  // codec.Type value 2
```

### Package pkg/rpc -- exported functions

```go
func NewRegistry(endpoints []string) (*Registry, error)

func NewServer(opts ...ServerOption) *Server

func Dial(target string, opts ...DialOption) (*ClientConn, error)

func WithTimeout(d time.Duration) DialOption
func WithDialCodec(t CodecType) DialOption
func WithRegistry(reg *Registry) DialOption
func WithLoadBalancer(lb LoadBalancer) DialOption

func WithCodec(t CodecType) ServerOption
```

### Package pkg/rpc -- exported struct types and methods

```go
type DialOption func(*dialOptions)

type ServerOption func(*serverOptions)

type Server struct { /* unexported fields */ }
func (s *Server) Register(name string, service interface{})
func (s *Server) Serve(lis net.Listener) error
func (s *Server) GracefulStop()
func (s *Server) Stop()

type ClientConn struct { /* unexported fields */ }
func (cc *ClientConn) Invoke(ctx context.Context, service, method string, args, reply interface{}) error
func (cc *ClientConn) NewStream(ctx context.Context, service, method string, args interface{}) (ClientStream, error)
func (cc *ClientConn) Close() error
```

### Package pkg/rpc -- exported interfaces (via type alias)

```go
// stream.ServerStream
type ServerStream interface {
    Send(msg interface{}) error
    Context() context.Context
}

// stream.ClientStream
type ClientStream interface {
    Recv(msg interface{}) error
    Context() context.Context
}

// load_balance.LoadBalancer
type LoadBalancer interface {
    Select([]registry.Instance) registry.Instance
}
```

### Package pkg/rpc -- exported struct fields (via type alias)

```go
// registry.Instance
type Instance struct {
    Addr string
}
```

### Package pkg/api -- example service types

```go
type Args struct { A int; B int }
type Args1 struct { A int; B int; C int }
type Reply struct { Result int }

type Arith struct{}
func (a *Arith) Add(_ context.Context, args *Args) (*Reply, error)
func (a *Arith) Mul(_ context.Context, args *Args) (*Reply, error)

type Arith2 struct{}
func (a *Arith2) Add(_ context.Context, args *Args1) (*Reply, error)
func (a *Arith2) Mul(_ context.Context, args *Args1) (*Reply, error)
```

## Internal implementation details affecting correctness

### Wire protocol (internal/protocol)

Binary, length-delimited, big-endian framing:

```
+--------+-----------+---------+----------------+--------------+
| Magic  | HeaderLen | BodyLen |  Header(JSON)  | Body(bytes)  |
| 2 byte | 4 byte   | 4 byte  |    N byte      |    M byte    |
+--------+-----------+---------+----------------+--------------+

Magic = 0x1234
```

`PacketBuffer.Read` scans for the magic number; on mismatch it advances by one byte and retries, providing a simple resync mechanism for corrupted streams. `Decode` rejects packets whose computed `10 + headerLen + bodyLen` exceeds `math.MaxInt`.

The header is always JSON-encoded on the wire, regardless of the body codec. The body is always Gzip-compressed: every send site in the codebase hard-codes `Compression: codec.CompressionGzip`.

Header struct:

```go
type Header struct {
    RequestID   uint64
    ServiceName string
    MethodName  string
    Error       string
    CodecType   CodecType                  // protocol-local type
    Compression codec.CompressionType
    StreamFlag  StreamFlag `json:",omitempty"`
}

type StreamFlag byte
const (
    StreamNone  StreamFlag = 0   // unary response
    StreamData  StreamFlag = 1   // stream data frame
    StreamEnd   StreamFlag = 2   // stream terminal success
    StreamError StreamFlag = 3   // stream terminal error
)
```

`StreamFlag` uses JSON `omitempty`, so `StreamNone` (value 0) does not appear in serialised headers. A missing `StreamFlag` key means a normal unary response.

`protocol.CodecType` and `codec.Type` are two independent types sharing the same numeric encoding (1 = JSON, 2 = Proto). Headers carry the protocol-local type.

### Codec system (internal/codec)

```go
type Codec interface {
    Marshal(v interface{}) ([]byte, error)
    Unmarshal(data []byte, v interface{}) error
}

type Type byte
const JSON  Type = 1
const PROTO Type = 2

type CompressionType byte
const CompressionNone CompressionType = 0
const CompressionGzip CompressionType = 1
```

Codecs are registered through an `init`-time factory map keyed by `Type`. The JSON codec uses `encoding/json`. The Protobuf codec requires `v` to implement `proto.Message`; passing a plain struct produces `proto codec: not proto.Message` at Marshal time. `Register` panics if the factory is nil or the type is already registered.

Compression functions `Compress(data, type)` and `Decompress(data, type)` delegate to a compressor registry. Only Gzip is registered by default.

### Transport layer (internal/transport)

`TCPConnection` wraps a `net.Conn` with a `bufio.Reader` (4096 buffer) and a `PacketBuffer` for frame extraction. `Close` calls `SetLinger(0)`, producing a TCP RST rather than FIN.

`TCPClient` owns one `TCPConnection` plus two `sync.Map` registries: `pending` (RequestID to Future) for unary calls, and `streams` (RequestID to ClientStreamConn) for server-streams. `nextSeq` uses `atomic.AddUint64` starting at 1; RequestID 0 is never produced. A single `readLoop` goroutine demultiplexes every response by examining `Header.StreamFlag`:

- `StreamData` -> `streams[seq].Push(body)`
- `StreamEnd` -> `streams[seq].End()` and delete entry
- `StreamError` -> `streams[seq].Error(err)` and delete entry
- default (`StreamNone`) -> `pending[seq].Done(body, err)` and delete entry

When the TCP read fails, `fail(err)` fires exactly once (`atomic.CompareAndSwapInt32`) and resolves every outstanding Future and stream with the error, preventing goroutine leaks.

`ConnectionPool` holds a slice of `*TCPClient` with a configured `maxActive` (always 1 at every call site). `Acquire` holds the pool mutex across `net.DialTimeout` (5s), serialising concurrent callers behind cold-start dials. Dead connections are evicted on access: the loop checks `conn.closed` and removes stale entries.

`Future` provides both raw-byte and decoded-reply consumers: `Wait`, `WaitWithContext`, `WaitWithTimeout`, `GetResult`, `GetResultWithContext`, `DoneChan`, `OnComplete`, `IsDone`, and the producer-side `Done`. Calling `Done` multiple times is safe (second call is a no-op). `OnComplete` invokes the callback immediately if the future is already complete; otherwise stores it for `Done` to invoke.

`ClientStreamConn` is backed by a buffered channel of capacity 64. `Push`, `End`, and `Error` are the producer methods called by `readLoop`. `End` and `Error` are guarded by `sync.Once` so they fire exactly one terminal frame. `Recv` blocks on the channel or the context, returning `io.EOF` on end, the error on error, or decoded data on data frames.

### Reflection-based method dispatch (internal/server/handler.go)

`Handler.invoke` reflects on the registered service value and matches one of three method signatures:

1. grpc-go style: `Method(ctx context.Context, req *T) (*R, error)` -- detected when `numIn == 2`, `numOut == 2`, `In(0).Implements(contextType)`, `In(1).Kind() == Ptr`, and `Out(1).Implements(errorType)`. A nil `*R` returns an empty body.
2. net/rpc style: `Method(req *T, reply *R) error` -- detected when `numIn == 2`, `numOut == 1`, `Out(0).Implements(errorType)`, and `In(1).Kind() == Ptr`. Reply is allocated by the framework via `reflect.New`.
3. Streaming style: `Method(req *T, stream ServerStream) error` -- same outer shape as net/rpc but `In(1).Implements(serverStreamType)`. The framework constructs a `serverStream`, invokes the method, then emits `StreamEnd` on success or `StreamError` on error.

Any other signature produces `unsupported method signature: <Service>.<Method>` at call time (not at registration time).

### Circuit breaker (internal/breaker)

Three states: `Closed` (0), `Open` (1), `HalfOpen` (2).

- Closed: accumulates successes and failures. When total reaches `windowSize`, computes `failure/total`; if it meets `failureThreshold`, transitions to Open; otherwise resets counters for a fresh window.
- Open: rejects all `Allow()` calls until `openTimeout` elapses from the last state change. The next `Allow()` returns true (single probe) and transitions to HalfOpen.
- HalfOpen: one in-flight probe permitted (`halfOpenProbe` flag). Successful probe transitions to Closed; failed probe returns to Open.

The client always creates breakers with parameters `(windowSize=10, failureThreshold=0.6, openTimeout=5s)`, keyed by `service|addr`.

### Rate limiter (internal/limiter)

Token bucket refilled to `rate` once per second by a background goroutine. Burst capacity equals `rate`. `Stop` shuts the goroutine down idempotently via `sync.Once`. Negative or zero rates produce a bucket that always rejects.

### Load balancing (internal/load_balance)

- `RoundRobin` -- lock-free via `atomic.AddUint64`. Returns empty `Instance` for nil/empty lists.
- `Random` (construct with `NewRandom()`) -- `math/rand` guarded by a mutex.
- `WeightedRR` (construct with `NewWeightedRR(weights)`) -- Nginx-style smooth weighted round-robin. Returns empty `Instance` if the weight slice length does not match the instance list length, or if total weight is zero or negative.

### Service registry (internal/registry)

- `Register(service, ins, ttl)` grants an etcd lease of `ttl` seconds, writes `<prefix><service>/<addr> = <addr>` under that lease, and starts a KeepAlive goroutine that drains the channel but does not handle failures.
- `Discover(service)` lazily initialises a per-service in-memory cache plus a background watch goroutine. Subsequent calls return a defensive copy of the cached instances in unspecified order.
- The etcd key prefix is hard-coded: `/github.com/hangtiancheng/lark-go/lark_rpc/services/`.
- `Close()` cancels the background context and closes the etcd client.

### Registry-mode client (internal/client)

Composes a `*registry.Registry`, a `LoadBalancer` (default `RoundRobin`), a `TokenBucket(10000)`, a `sync.Map` of connection pools (addr to ConnectionPool), and a `sync.Map` of circuit breakers (service|addr to CircuitBreaker). `Invoke` wraps the context with `context.WithTimeout`, delegates to `invokeAsync`, and waits with `GetResultWithContext`. The async path registers an `OnComplete` callback that records success/failure on the breaker. `InvokeStream` only records a failure at send time; mid-stream errors are not fed back into the breaker.

## Typical usage

### Unary server (grpc-go style)

```go
package main

import (
    "context"
    "log"
    "net"

    "github.com/hangtiancheng/lark-go/lark_rpc/pkg/rpc"
)

type Args struct {
    A int
    B int
}

type Reply struct {
    Result int
}

type MathService struct{}

func (s *MathService) Add(ctx context.Context, args *Args) (*Reply, error) {
    return &Reply{Result: args.A + args.B}, nil
}

func main() {
    server := rpc.NewServer()
    server.Register("Math", &MathService{})

    lis, err := net.Listen("tcp", ":8080")
    if err != nil {
        log.Fatal(err)
    }

    errCh := make(chan error, 1)
    go func() { errCh <- server.Serve(lis) }()

    // ... wait for signal ...

    server.GracefulStop()
}
```

### Unary client (static mode)

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/hangtiancheng/lark-go/lark_rpc/pkg/rpc"
)

type Args struct {
    A int
    B int
}

type Reply struct {
    Result int
}

func main() {
    conn, err := rpc.Dial("127.0.0.1:8080", rpc.WithTimeout(5*time.Second))
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    var reply Reply
    if err := conn.Invoke(context.Background(), "Math", "Add", &Args{A: 1, B: 2}, &reply); err != nil {
        log.Fatal(err)
    }
    log.Printf("Result: %d", reply.Result)
}
```

### Client with registry mode

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/hangtiancheng/lark-go/lark_rpc/pkg/rpc"
)

type Args struct{ A, B int }
type Reply struct{ Result int }

func main() {
    reg, err := rpc.NewRegistry([]string{"localhost:2379"})
    if err != nil {
        log.Fatal(err)
    }

    conn, err := rpc.Dial(
        "",
        rpc.WithRegistry(reg),
        rpc.WithTimeout(3*time.Second),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    var reply Reply
    err = conn.Invoke(context.Background(), "Math", "Add", &Args{A: 1, B: 2}, &reply)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Result: %d", reply.Result)
}
```

### Server-streaming

```go
// Server side -- handler method
func (s *FeedService) Subscribe(req *SubArgs, stream rpc.ServerStream) error {
    for i := 0; i < req.Count; i++ {
        if err := stream.Send(&Event{Index: i}); err != nil {
            return err
        }
    }
    return nil // framework emits StreamEnd automatically
}

// Client side -- consuming the stream
stream, err := conn.NewStream(ctx, "Feed", "Subscribe", &SubArgs{Count: 10})
if err != nil {
    log.Fatal(err)
}

for {
    var event Event
    if err := stream.Recv(&event); err != nil {
        if err == io.EOF {
            break
        }
        log.Fatal(err)
    }
    process(event)
}
```

### Method signature reference

| Signature                                         | Style     | Notes                                                       |
| ------------------------------------------------- | --------- | ----------------------------------------------------------- |
| `Method(ctx context.Context, req *T) (*R, error)` | grpc-go   | Recommended. nil \*R yields empty body.                     |
| `Method(req *T, reply *R) error`                  | net/rpc   | Reply allocated by framework.                               |
| `Method(req *T, stream rpc.ServerStream) error`   | streaming | Server-streaming only. Terminal frame emitted by framework. |

## Pitfalls / known limitations

1. Body Gzip compression is mandatory. Every internal send site sets `Compression: codec.CompressionGzip`. For small payloads the 18-byte gzip header can dwarf the body. There is no option to disable compression.

2. Server-wide hard cap of 10,000 RPS via `limiter.NewTokenBucket(10000)`, not exposed as a `ServerOption`. An identical 10,000 RPS cap exists on the registry-mode client.

3. Circuit-breaker parameters `(windowSize=10, failureThreshold=0.6, openTimeout=5s)` are hard-coded in `internal/client/resolver.go::getBreaker`. There is no public API to tune them.

4. Static-mode `ClientConn` bypasses `internal/client.Client` entirely, so it has neither rate limiting, circuit breaking, nor load balancing. Those capabilities require `WithRegistry`.

5. `ConnectionPool` is single-connection (`maxActive = 1` at every call site). All requests to a given address share one TCP socket and one `readLoop`. A slow stream consumer that fills its 64-frame buffer blocks `readLoop`, stalling every other future and stream on that connection.

6. `ConnectionPool.Acquire` holds the pool mutex across `net.DialTimeout` (5s). Concurrent callers serialise behind cold-start dials.

7. `WeightedRR.Select` requires the `[]Instance` argument and the configured weight slice to align positionally. `registry.copyInstances` returns instances in `map` iteration order (randomised per Go spec). Combining `WeightedRR` with the etcd registry directly misroutes traffic unless an extra stable-sorting layer is added.

8. `NewServer` panics on invalid options rather than returning an error. Never feed it untrusted configuration without a `recover`.

9. `TCPConnection.Close` calls `SetLinger(0)`, producing a TCP RST. A client mid-Read during a server graceful-stop may see "connection reset by peer" instead of EOF.

10. Only server-streaming exists. The `StreamFlag` enum has no codepoints for client-originated stream frames. Do not attempt client-streaming or bidirectional streaming.

11. `pkg/rpc` does not expose a server-side "register into etcd" helper. To publish a service instance into the registry, call `registry.Register` through an internal package wrapper or add a re-export to `pkg/rpc`.

12. Pending Futures have no internal timeout. Always wait with `GetResultWithContext` or rely on the timeout passed to `Dial`, or the entry leaks until the connection dies.

13. `InvokeStream` records a circuit-breaker failure only at send time. Mid-stream errors (server crashes, network partitions after the first frame) are not fed back into the breaker state.

14. The etcd key prefix `/github.com/hangtiancheng/lark-go/lark_rpc/services/` is hard-coded with no namespace or environment parameter. Multiple environments sharing one etcd cluster will collide.

15. Method signature validation is deferred to call time. Registering a service with unsupported method shapes succeeds silently; the error surfaces only when a client invokes that method.

16. Framework error strings travel in `Header.Error` and are returned verbatim by `Invoke` / `NewStream` as `errors.New(headerError)`. The full list: `service not found: <Service>`, `method not found: <Service>.<Method>`, `unsupported method signature: <Service>.<Method>`, `rate limit exceeded`, `circuit breaker open`, `no instance available`, `connection closed`, `registry not configured`, `rate limit exceeded` (client-side).

## File map

| Directory                | Files                                                                                                         | Purpose                                                             |
| ------------------------ | ------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------- |
| `pkg/rpc/`               | `rpc.go`, `server.go`, `client.go`, `stream.go`, `stream_test.go`                                             | Public API: type aliases, Server, ClientConn, stream interfaces     |
| `pkg/api/`               | `arith.go`, `arith_test.go`                                                                                   | Example Arith/Arith2 services used by tests and demos               |
| `internal/server/`       | `server.go`, `handler.go`, `options.go`, `stream.go`, `server_test.go`                                        | TCP server loop, reflection dispatch, server-side stream emitter    |
| `internal/client/`       | `client.go`, `invoke.go`, `option.go`, `resolver.go`, `client_test.go`                                        | Registry-mode client: Invoke, InvokeAsync, InvokeStream, breaker/LB |
| `internal/transport/`    | `tcp_connection.go`, `tcp_connection_pool.go`, `tcp_client.go`, `future.go`, `stream.go`, `transport_test.go` | TCP framing, pool, multiplexed client, Future, ClientStreamConn     |
| `internal/protocol/`     | `header.go`, `message.go`, `message_test.go`                                                                  | Wire format: Header struct, Magic, Encode / Decode                  |
| `internal/codec/`        | `codec.go`, `json.go`, `protobuf.go`, `compress.go`, `codec_test.go`                                          | Codec registry, JSON / Protobuf codecs, Gzip compressor             |
| `internal/breaker/`      | `breaker.go`, `breaker_test.go`                                                                               | Three-state circuit breaker                                         |
| `internal/limiter/`      | `token_bucket.go`, `token_bucket_test.go`                                                                     | Per-second refill token bucket                                      |
| `internal/load_balance/` | `balancer.go`, `round_robin.go`, `random.go`, `weighted_rr.go`, `load_balance_test.go`                        | LoadBalancer interface and three built-in strategies                |
| `internal/registry/`     | `registry.go`, `registry_test.go`                                                                             | etcd v3 register / discover / watch                                 |
| `internal/stream/`       | `stream.go`                                                                                                   | ServerStream / ClientStream interfaces (cycle-breaker)              |

## Dependencies

- `go.etcd.io/etcd/client/v3 v3.6.7` -- etcd v3 client for service discovery and lease keepalive.
- `google.golang.org/protobuf v1.36.11` -- Protobuf codec implementation.
- Go standard library only for everything else: `net`, `compress/gzip`, `reflect`, `encoding/json`, `encoding/binary`, `sync`, `sync/atomic`, `context`, `time`, `bufio`, `io`, `math/rand`, `fmt`, `log`.
- Go toolchain requirement: 1.26+.

## Cross-references

- `lark-cache` -- Use lark-cache for in-process caching layers. lark_rpc does not integrate a cache; combine the two when building services that need response memoisation on the client side.
- `lark-http` -- Use lark-http for HTTP servers, REST APIs, SSE, or WebSocket work. lark_rpc is TCP-only with a custom binary protocol; the two are complementary but non-overlapping.
- `lark-orm` -- Use lark-orm for database access within RPC service handlers. Service methods registered with lark_rpc often use lark-orm internally to query persistence.
