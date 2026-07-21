---
name: swifty-rpc
description: >
  TCP-based RPC framework for Go (module path:
  github.com/hangtiancheng/swifty.go/swifty_rpc). Key exported symbols:
  rpc.Dial, rpc.NewServer, rpc.NewRegistry, ClientConn (Invoke, InvokeAsync,
  NewStream, Close), Future, Server, ServerStream, ClientStream, DialOption,
  ServerOption, CodecType, CodecJSON, CodecProto, Instance, LoadBalancer,
  WithTimeout, WithDialCodec, WithRegistry, WithLoadBalancer, WithCodec. Use
  when writing or modifying Go code that imports swifty_rpc, calls
  Invoke/InvokeAsync/NewStream, registers services on rpc.NewServer, uses etcd
  discovery via rpc.NewRegistry, or asks about the binary wire protocol (Magic
  0x1234 framing), StreamFlag frames, per-header codec negotiation, RequestID
  multiplexing, reflection dispatch (grpc-go and net/rpc signatures),
  server-streaming, circuit breaker, rate limiting, load balancing, or
  graceful shutdown. Do NOT use for grpc-go itself, Connect-Go, Twirp,
  HTTP/SSE/WebSocket work (use swifty-http), or Protobuf schema questions
  unrelated to swifty_rpc.
---

# swifty_rpc

A TCP-based RPC framework with a grpc-go-style public API that provides request multiplexing on a single connection, asynchronous invocation through futures, server-streaming with per-stream goroutines, pluggable JSON and Protobuf codecs negotiated per request header, mandatory Gzip body compression, connection pooling, three-state circuit breaking, token-bucket rate limiting, three built-in load-balancing strategies (round-robin, random, smooth-weighted round-robin), and etcd v3 service registration and discovery. The module path is `github.com/hangtiancheng/swifty.go/swifty_rpc`, the public API lives in `pkg/rpc`, and all implementation details reside under `internal/`, which Go prevents external packages from importing.

## Architecture overview

```
                          +------------------+
                          |   pkg/rpc        |  Public API surface
                          |  (Server,        |
                          |   ClientConn,    |
                          |   Future, Dial)  |
                          +--------+---------+
                                   |
              +--------------------+--------------------+
              |                                         |
   +----------v-----------+              +-------------v-----------+
   | internal/server      |              | internal/client         |
   |  Server accept loop  |              |  Registry-mode client   |
   |  Handler (reflect,   |              |  Breaker + Limiter + LB |
   |   safeCall recover)  |              |  Invoke / InvokeAsync / |
   |  serverStream        |              |  InvokeStream           |
   |  (async, streamWg)   |              |  observedStream         |
   +----------+-----------+              +-------------+-----------+
              |                                         |
              |    +--------------------+               |
              +--->| internal/transport |<--------------+
              |    |  TCPConnection     |
              |    |  TCPClient (seq,   |
              |    |   pending, streams,|
              |    |   readLoop)        |
              |    |  ConnectionPool    |
              |    |  Future            |
              |    |  ClientStreamConn  |
              |    +--------+-----------+
              |             |
              v             v
   +----------+-------------+----------+
   | internal/protocol                 |
   |  Header, Message, Encode, Decode  |
   |  Magic = 0x1234, StreamFlag       |
   +----------+------------------------+
              |
              v
   +----------+------------------------+
   | internal/codec                    |
   |  Codec interface, JSON, Protobuf  |
   |  CompressionType, Gzip            |
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

1. `pkg/rpc` re-exports a minimal set of types from `internal/codec`, `internal/registry`, `internal/load_balance`, `internal/stream`, and `internal/transport` via type aliases. Downstream code must never import `internal/*` directly.
2. `internal/stream` exists solely to break a potential cyclic dependency between `internal/server` and `internal/transport`. Both packages reference `stream.ServerStream` / `stream.ClientStream`, never each other.

Ownership: a `ClientConn` in static mode owns one `ConnectionPool`; in registry mode it owns one `internal/client.Client`, which owns one pool per discovered address plus one circuit breaker per `service|addr` pair. Each `TCPClient` owns one TCP socket, one `readLoop` goroutine, a `pending` map (RequestID to Future), and a `streams` map (RequestID to ClientStreamConn). The server owns one goroutine per accepted connection plus one goroutine per in-flight streaming handler, tracked by a per-connection `streamWg`.

## Core types

### Package pkg/rpc -- type aliases

```go
type CodecType    = codec.Type              // byte; JSON = 1, PROTO = 2
type Registry     = registry.Registry
type Instance     = registry.Instance      // struct { Addr string }
type LoadBalancer = load_balance.LoadBalancer
type ServerStream = stream.ServerStream
type ClientStream = stream.ClientStream
type Future       = transport.Future       // async invocation handle
```

### Package pkg/rpc -- exported constants and variables

```go
var CodecJSON  = codec.JSON   // codec.Type value 1
var CodecProto = codec.PROTO  // codec.Type value 2
```

### Package pkg/rpc -- exported functions

```go
func NewRegistry(endpoints []string) (*Registry, error)

func NewServer(opts ...ServerOption) *Server   // panics on invalid options

func Dial(target string, opts ...DialOption) (*ClientConn, error)

func WithTimeout(d time.Duration) DialOption      // default 5s
func WithDialCodec(t CodecType) DialOption        // default CodecJSON
func WithRegistry(reg *Registry) DialOption       // switches to registry mode
func WithLoadBalancer(lb LoadBalancer) DialOption // registry mode only

func WithCodec(t CodecType) ServerOption          // server default codec
```

`Dial` never dials the network itself. It validates the codec, then either constructs a registry-mode client (when `WithRegistry` was supplied; `target` is ignored) or a lazy single-connection pool for `target`. The first `Invoke` establishes the TCP connection.

### Package pkg/rpc -- struct types and methods

```go
type DialOption func(*dialOptions)
type ServerOption func(*serverOptions)

type Server struct { /* wraps internal/server.Server */ }
func (s *Server) Register(name string, service interface{})
func (s *Server) Serve(lis net.Listener) error
func (s *Server) GracefulStop()
func (s *Server) Stop()

type ClientConn struct { /* unexported fields */ }
func (cc *ClientConn) Invoke(ctx context.Context, service, method string, args, reply interface{}) error
func (cc *ClientConn) InvokeAsync(ctx context.Context, service, method string, args interface{}) (*Future, error)
func (cc *ClientConn) NewStream(ctx context.Context, service, method string, args interface{}) (ClientStream, error)
func (cc *ClientConn) Close() error   // always returns nil
```

Method behavior:

- `Register` stores the service under a mutex; no signature validation happens at registration time. Method-shape errors surface at call time.
- `Serve` runs the accept loop until shutdown. If `Stop`/`GracefulStop` ran before `Serve`, `Serve` closes the listener and returns nil immediately instead of blocking in `Accept`.
- `GracefulStop` closes the listener, waits for the accept loop, interrupts idle connections via `SetReadDeadline(now)`, and waits for in-flight requests and streams to finish. It returns even when clients keep idle pooled connections open.
- `Stop` closes the listener and force-closes every connection. `Stop` remains effective after a prior `GracefulStop` call because only the listener-closing step is guarded by a shared `sync.Once`.
- `Invoke` wraps the caller context with the dial timeout, sends, and waits. On timeout/cancel it resolves the pending future with the context error so the breaker (registry mode) records exactly one failure and a late server response becomes a no-op.
- `InvokeAsync` sends without waiting and returns a `*Future`. A watchdog goroutine (`time.NewTimer`, stopped on completion) resolves the future with `context.DeadlineExceeded` when the dial timeout elapses first. Available in both static and registry modes.
- `NewStream` starts a server-stream. Pool acquisition is bounded by the dial timeout; the stream itself lives on the caller's context.
- `Close` closes the pool (static) or the registry client including its rate limiter (registry).

### Future (transport.Future, exported as rpc.Future)

```go
func (f *Future) Wait() ([]byte, error)                       // raw body, blocks forever
func (f *Future) WaitWithContext(ctx context.Context) ([]byte, error)
func (f *Future) WaitWithTimeout(d time.Duration) ([]byte, error)
func (f *Future) GetResult(reply interface{}) error           // decode with future codec, blocks forever
func (f *Future) GetResultWithContext(ctx context.Context, reply interface{}) error
func (f *Future) DoneChan() <-chan struct{}
func (f *Future) IsDone() bool
func (f *Future) OnComplete(fn func(error))                   // fires immediately if already done
func (f *Future) Done(res []byte, err error)                  // producer side; idempotent
```

`Done` is idempotent: the second and later calls are no-ops, so double resolution (late response after timeout) is safe. `OnComplete` runs at most once; it is how the registry-mode client records breaker success/failure. The future decodes replies with the codec captured at send time (`NewFutureWithCodec`), defaulting to JSON.

### Stream interfaces (internal/stream, exported via aliases)

```go
type ServerStream interface {
    Send(msg interface{}) error
    Context() context.Context
}

type ClientStream interface {
    Recv(msg interface{}) error   // io.EOF on clean end, error on StreamError
    Context() context.Context
}
```

### Sentinel errors and error strings

- `transport.ErrPoolClosed` (`"connection pool closed"`) -- returned by `Acquire` after `Close`.
- `transport.ErrStreamNotFound` -- declared but not referenced anywhere in the codebase.
- Framework errors travel as strings in `Header.Error` and come back from `Invoke`/`NewStream` as `errors.New(...)`; match by substring. Full list: `service not found: <S>`, `method not found: <S>.<M>`, `unsupported method signature: <S>.<M>`, `handler panic: <value>`, `rate limit exceeded` (server and client side), `circuit breaker open`, `no instance available`, `load balancer returned empty address`, `registry not configured`, `connection closed`, `codec: type <n> not registered`.
- Client option validation errors: `client timeout must be positive`, `client load balancer must not be nil`.

### Package pkg/api -- example service types

```go
type Args struct { A, B int }
type Args1 struct { A, B, C int }
type Reply struct { Result int }

type Arith struct{}   // Add / Mul (ctx, *Args) (*Reply, error)
type Arith2 struct{}  // Add / Mul (ctx, *Args1) (*Reply, error)
```

## Internal implementation details affecting correctness

### Wire protocol (internal/protocol)

Binary, length-delimited, big-endian framing:

```
+--------+-----------+---------+----------------+--------------+
| Magic  | HeaderLen | BodyLen |  Header(JSON)  | Body(bytes)  |
| 2 byte | 4 byte    | 4 byte  |    N byte      |    M byte    |
+--------+-----------+---------+----------------+--------------+

Magic = 0x1234
```

The header is always JSON-encoded on the wire regardless of the body codec. The body is always Gzip-compressed: every send site hard-codes `Compression: codec.CompressionGzip`. `Decode` rejects packets whose computed `10 + headerLen + bodyLen` exceeds `math.MaxInt`.

`PacketBuffer.Read` resynchronizes on corruption by looping: it skips leading bytes until the buffer starts with the magic number, then extracts a frame if complete. This is a loop, so a valid frame behind garbage is recovered in the same call.

Header struct:

```go
type Header struct {
    RequestID   uint64
    ServiceName string
    MethodName  string
    Error       string
    CodecType   CodecType             // protocol-local type; carries codec negotiation
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

`StreamFlag` uses JSON `omitempty`, so `StreamNone` (0) does not appear in serialized headers; a missing key means unary. `protocol.CodecType` and `codec.Type` are distinct Go types sharing the numeric encoding (1 = JSON, 2 = Proto); clients convert with `protocol.CodecType(codecType)` when building headers.

### Codec negotiation

Every client send path (static and registry, unary and stream) writes the client's codec type into `Header.CodecType`. The server handler honors it: `Process` instantiates the announced codec for both request decoding and response encoding, falling back to the server's configured codec (default JSON) when the header carries 0. A `WithDialCodec(CodecProto)` client therefore works against a default-JSON server, provided the request and reply types implement `proto.Message`.

### Codec system (internal/codec)

```go
type Codec interface {
    Marshal(v interface{}) ([]byte, error)
    Unmarshal(data []byte, v interface{}) error
}
type Type byte           // JSON = 1, PROTO = 2
type CompressionType byte // CompressionNone = 0, CompressionGzip = 1
```

Codecs register through an `init`-time factory map. `Register` panics on nil factory or duplicate type. The Protobuf codec requires `proto.Message`; a plain struct fails with `proto codec: not proto.Message` at Marshal/Unmarshal time. Only the Gzip compressor is registered by default; `RegisterCompressor` can add others.

### Transport layer (internal/transport)

`TCPConnection` wraps a `net.Conn` with a 4096-byte `bufio.Reader` and a `PacketBuffer`. Writes are serialized by a write mutex and loop until the full frame is flushed. `Close` calls `SetLinger(0)` on TCP connections, producing an RST rather than FIN. `SetReadDeadline` exists so the server can interrupt blocked reads during graceful shutdown.

`TCPClient` multiplexes one socket. `nextSeq` uses `atomic.AddUint64` starting from 1; RequestID 0 is never produced. The single `readLoop` goroutine demultiplexes by `Header.StreamFlag`:

- `StreamData` -> `streams[seq].Push(body)`
- `StreamEnd` -> `streams[seq].End()` and delete entry
- `StreamError` -> `streams[seq].Error(err)` and delete entry
- default (`StreamNone`) -> `pending[seq].Done(body, err)` and delete entry

Teardown is unified in `shutdown(err)`, guarded by `atomic.CompareAndSwapInt32`: it closes the socket, resolves every pending future with the error, and terminates every stream. Both a read failure (`fail`) and an explicit `Close` (error `connection closed`) take this path, so callers blocked in bare `Wait()`/`GetResult()` are always woken. `SendAsyncWithCodec` and `SendStream` re-check the `closed` flag after storing their map entry to close the race window against a concurrent `shutdown` sweep.

`ConnectionPool` holds a slice of `*TCPClient` with `maxActive` (always 1 at every call site). `Acquire` holds the pool mutex across `net.DialTimeout` (5s), so concurrent callers serialize behind cold-start dials; the context is checked only on entry and does not bound the dial. Dead connections are evicted on access with corrected slice indexing.

`ClientStreamConn` buffers up to 64 data frames in a channel. Terminal state is delivered out-of-band via `termCh` guarded by `sync.Once`, so `End`/`Error` never block `readLoop` even when the data buffer is full. `Recv` drains buffered data frames before reporting the terminal state (no frame received prior to `StreamEnd`/`StreamError` is lost), returning `io.EOF` on clean end, the terminal error otherwise, or `ctx.Err()` on context cancellation. `Push` (data frames) can still block `readLoop` when the buffer is full and the stream is neither terminated nor cancelled. `Cancel()` cancels the stream's derived context.

### Reflection-based method dispatch (internal/server/handler.go)

`Handler.invoke` matches one of three signatures on the registered service value:

1. grpc-go style: `Method(ctx context.Context, req *T) (*R, error)` -- requires `numIn == 2`, `numOut == 2`, `In(0)` implements `context.Context`, `In(1)` pointer, `Out(0)` pointer, `Out(1)` implements `error`. A non-pointer first return value is rejected as `unsupported method signature` (it does not panic). A nil `*R` yields an empty response body.
2. net/rpc style: `Method(req *T, reply *R) error` -- `numIn == 2`, `numOut == 1`, `Out(0)` error, both params pointers. The framework allocates the reply via `reflect.New`.
3. Streaming style: `Method(req *T, stream ServerStream) error` -- same outer shape as net/rpc but the second parameter implements `ServerStream`. The framework constructs a `serverStream` and runs the handler in its own goroutine (tracked by the per-connection `streamWg`), so a long-lived stream does not block other requests multiplexed on the connection. On return, the framework emits `StreamEnd` (nil error) or `StreamError`.

Every reflective call goes through `safeCall`, which recovers panics and converts them to `handler panic: <value>` error frames; a misbehaving handler cannot crash the server process. The handler receives `context.Background()`-derived contexts; caller deadlines are not propagated to the server side.

Unary requests on a connection are processed serially by the per-connection `Handle` loop; only streaming handlers run concurrently.

### Server lifecycle (internal/server/server.go)

`Serve` registers the accept loop in `serveWg` and each connection handler in `wg`. `beginShutdown` (shared `sync.Once`) closes the `closing` channel and the listener. `GracefulStop` then waits for the accept loop, sweeps `SetReadDeadline(now)` over all connections to interrupt idle reads (in-flight requests and streams complete first), waits on `wg`, and stops the limiter. `Stop` waits for the accept loop, force-closes every connection, and stops the limiter without waiting for handlers. The server enforces a hard-coded 10,000 RPS token bucket; over-limit requests receive an error response carrying the original RequestID.

### Circuit breaker (internal/breaker)

Three states: `Closed` (0), `Open` (1), `HalfOpen` (2).

- Closed: counts successes and failures; both contribute to the rolling window. When total reaches `windowSize`, computes the failure rate; at or above `failureThreshold` transitions to Open, otherwise resets the window.
- Open: rejects `Allow()` until `openTimeout` elapses; the next `Allow()` transitions to HalfOpen and admits a single probe.
- HalfOpen: exactly one in-flight probe. Success closes the breaker; failure reopens it.

The registry-mode client creates breakers with hard-coded parameters `(windowSize=10, failureThreshold=0.6, openTimeout=5s)`, keyed `service|addr`. Breaker accounting is complete on the unary path: send failures, response errors, and timeouts (via forced `future.Done`) all record. On the stream path, `observedStream` records success on `io.EOF`, failure on any other terminal error, and deliberately ignores `context.Canceled` (caller-initiated cancellation is not a service failure); at most one record per stream.

### Rate limiter (internal/limiter)

Token bucket refilled to `rate` once per second by a ticker goroutine; burst capacity equals `rate`. Negative rates are clamped to zero (always reject). `Stop` is idempotent via `sync.Once`. Both the server and the registry-mode client stop their limiter on shutdown/close.

### Load balancing (internal/load_balance)

- `RoundRobin` (zero value usable; `NewRR()` also provided) -- lock-free atomic counter; first selection is index 0. Empty list yields empty `Instance`.
- `Random` (construct with `NewRandom()`) -- `math/rand` behind a mutex.
- `WeightedRR` (construct with `NewWeightedRR(weights)`) -- Nginx-style smooth weighted round-robin. Negative weights are clamped to 0. Returns empty `Instance` when the weight slice length differs from the instance list length or total weight is not positive.

An empty selection propagates as the client error `load balancer returned empty address`.

### Service registry (internal/registry)

- `Register(service, ins, ttl)` grants an etcd lease of `ttl` seconds, writes `<prefix><service>/<addr> = <addr>` under the lease, and starts a KeepAlive goroutine that only drains the channel; lease expiry (etcd restart, partition) is not detected and the instance silently disappears without re-registration.
- `Discover(service)` lazily initializes a per-service cache plus a background watch goroutine, then returns a defensive copy of cached instances in map-iteration (randomized) order.
- The etcd key prefix is hard-coded: `/github.com/hangtiancheng/swifty.go/swifty_rpc/services/`.
- `Close()` cancels the background context and closes the etcd client; both steps are nil-safe.

`pkg/rpc` does not wrap `Register`, but because `rpc.Registry` is a type alias, servers call `reg.Register("Math", rpc.Instance{Addr: addr}, ttl)` directly on the value returned by `rpc.NewRegistry`.

### Registry-mode client (internal/client)

Composes the registry, a `LoadBalancer` (default zero-value `RoundRobin`), a `TokenBucket(10000)`, a `sync.Map` of pools (addr to `ConnectionPool`), and a `sync.Map` of breakers. `Invoke`/`InvokeAsync`/`InvokeStream` each run the pipeline: limiter -> discover + select address -> breaker gate -> pool acquire (bounded by client timeout) -> marshal -> send. `Close` stops the limiter and closes every pool. Options: `WithClientCodec`, `WithClientTimeout` (must be positive), `WithClientLoadBalancer` (must be non-nil); each returns an error consumed by `NewClient`.

## Typical usage

### Unary server (grpc-go style)

```go
package main

import (
    "context"
    "log"
    "net"

    "github.com/hangtiancheng/swifty.go/swifty_rpc/pkg/rpc"
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

    "github.com/hangtiancheng/swifty.go/swifty_rpc/pkg/rpc"
)

type Args struct{ A, B int }
type Reply struct{ Result int }

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

### Asynchronous invocation

```go
future, err := conn.InvokeAsync(context.Background(), "Math", "Add", &Args{A: 1, B: 2})
if err != nil {
    log.Fatal(err)
}

// ... do other work; optionally select on future.DoneChan() ...

var reply Reply
if err := future.GetResultWithContext(context.Background(), &reply); err != nil {
    log.Fatal(err)
}
```

The future resolves with the reply, a server error, or `context.DeadlineExceeded` once the dial timeout elapses, so waiting on it never hangs forever.

### Registry mode: registration and discovery

```go
// Server side -- publish the instance into etcd.
reg, err := rpc.NewRegistry([]string{"localhost:2379"})
if err != nil {
    log.Fatal(err)
}
if err := reg.Register("Math", rpc.Instance{Addr: "10.0.0.5:8080"}, 10); err != nil {
    log.Fatal(err)
}

// Client side -- discover through the same registry.
conn, err := rpc.Dial(
    "", // target ignored in registry mode
    rpc.WithRegistry(reg),
    rpc.WithTimeout(3*time.Second),
)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

var reply Reply
if err := conn.Invoke(context.Background(), "Math", "Add", &Args{A: 1, B: 2}, &reply); err != nil {
    log.Fatal(err)
}
```

### Server-streaming

```go
// Server side -- streaming handler; runs in its own goroutine.
func (s *FeedService) Subscribe(req *SubArgs, stream rpc.ServerStream) error {
    for i := 0; i < req.Count; i++ {
        if err := stream.Send(&Event{Index: i}); err != nil {
            return err
        }
    }
    return nil // framework emits StreamEnd automatically
}

// Client side -- consuming the stream.
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

| Signature                                          | Style     | Notes                                                          |
| -------------------------------------------------- | --------- | -------------------------------------------------------------- |
| `Method(ctx context.Context, req *T) (*R, error)`  | grpc-go   | Recommended. nil `*R` yields empty body. `*R` must be pointer.  |
| `Method(req *T, reply *R) error`                   | net/rpc   | Reply allocated by framework.                                  |
| `Method(req *T, stream rpc.ServerStream) error`    | streaming | Runs in own goroutine. Terminal frame emitted by framework.    |

## Pitfalls / known limitations

1. Body Gzip compression is mandatory. Every send site hard-codes `Compression: codec.CompressionGzip`; for small payloads the gzip overhead dwarfs the body and there is no option to disable it.

2. Hard-coded 10,000 RPS caps on both the server and the registry-mode client (`limiter.NewTokenBucket(10000)`), not exposed as options. Circuit-breaker parameters `(10, 0.6, 5s)` in `internal/client/resolver.go` are equally fixed.

3. Every `ConnectionPool` is single-connection (`maxActive = 1` at all call sites). All unary calls and streams to one address share one socket and one `readLoop`. A slow stream consumer that fills its 64-frame buffer blocks `readLoop` on the next `Push`, stalling every other future and stream on that connection. Terminal frames (`End`/`Error`) never block, but data frames do. Consume streams promptly or cancel their context.

4. `ConnectionPool.Acquire` holds the pool mutex across a 5s `net.DialTimeout`, and the acquire context does not bound the dial itself. Concurrent callers serialize behind cold-start dials to unreachable addresses.

5. Static-mode `ClientConn` bypasses `internal/client.Client` entirely: no rate limiting, no circuit breaking, no load balancing. Those capabilities require `WithRegistry`.

6. `WeightedRR.Select` matches weights to instances positionally, but `registry.copyInstances` returns instances in randomized map order. Combining `WeightedRR` with the etcd registry misroutes traffic unless a stable-ordering layer is added; use it only with static, ordered instance lists.

7. `NewServer` panics on invalid options (for example an unregistered codec type) instead of returning an error. Do not feed it untrusted configuration without `recover`.

8. `TCPConnection.Close` calls `SetLinger(0)`, sending a TCP RST. Peers mid-read during a hard `Stop` observe "connection reset by peer" rather than EOF.

9. Only server-streaming exists. `StreamFlag` has no codepoints for client-originated stream frames; client-streaming and bidirectional streaming are impossible at the protocol level.

10. The registry KeepAlive goroutine only drains the keepalive channel. If the lease expires (etcd restart, network partition), the instance silently vanishes from discovery and is never re-registered. Add external re-registration if availability matters.

11. The etcd key prefix `/github.com/hangtiancheng/swifty.go/swifty_rpc/services/` is hard-coded with no namespace parameter. Environments sharing an etcd cluster collide.

12. Method signature validation happens at call time, not registration time. Registering a service with unsupported method shapes succeeds silently; clients see `unsupported method signature` when they call.

13. Server handlers receive contexts derived from `context.Background()`; the client's deadline is not transmitted. Long-running handlers keep executing after the client times out (the client-side future resolves with the deadline error; the late response is discarded by the idempotent `Done`).

14. Unary requests on a single connection are handled serially by the per-connection loop. Only streaming handlers run concurrently. A slow unary handler delays subsequent unary calls multiplexed on the same connection.

15. Errors cross the wire as strings in `Header.Error` and come back as fresh `errors.New` values. `errors.Is`/`errors.As` against typed errors does not work across the RPC boundary; match on substrings.

16. `NewHandler`'s first parameter (`s interface{}`) is unused; the service is passed per-request through `Process`. Do not expect handler-level service binding.

## File map

| File                                          | Purpose                                                                 |
| --------------------------------------------- | ----------------------------------------------------------------------- |
| `pkg/rpc/rpc.go`                              | Type aliases (CodecType, Registry, Instance, LoadBalancer, Future), CodecJSON/CodecProto, NewRegistry |
| `pkg/rpc/client.go`                           | DialOption set, Dial, ClientConn (Invoke, InvokeAsync, NewStream, Close), static-mode send paths |
| `pkg/rpc/server.go`                           | ServerOption set, NewServer (panics on bad options), Server facade      |
| `pkg/rpc/stream.go`                           | ServerStream / ClientStream aliases                                      |
| `pkg/rpc/stream_test.go`                      | End-to-end streaming tests: basic, mid-stream error, unary coexistence, cancellation |
| `pkg/rpc/fixes_test.go`                       | Regression tests: GracefulStop with idle connection, stream not blocking unary, panic/bad-signature resilience, public InvokeAsync |
| `pkg/api/arith.go`, `pkg/api/arith_test.go`   | Example Arith/Arith2 services                                            |
| `internal/server/server.go`                   | Accept loop, per-connection Handle loop, GracefulStop/Stop, serveWg/wg tracking, rate limit |
| `internal/server/handler.go`                  | Reflection dispatch (3 signatures), per-header codec selection, safeCall panic recovery, async stream launch |
| `internal/server/options.go`                  | HandleOption / ServerOption, WithHandlerCodec, WithServerCodec           |
| `internal/server/stream.go`                   | serverStream: Send (StreamData), end (StreamEnd), sendError (StreamError) |
| `internal/server/server_test.go`              | Handler invoke tests, Process response tests, serve/stop lifecycle       |
| `internal/client/client.go`                   | Registry-mode Client struct, NewClient defaults, Close (stops limiter and pools) |
| `internal/client/invoke.go`                   | Invoke (timeout feeds breaker), InvokeAsync (timer watchdog), InvokeStream, observedStream |
| `internal/client/option.go`                   | ClientOption: codec, timeout, load balancer (validated)                  |
| `internal/client/resolver.go`                 | getPool, getAddr (discover + select), getBreaker (hard-coded params)     |
| `internal/client/client_test.go`              | Default codec, option validation, registry-less errors                   |
| `internal/transport/tcp_connection.go`        | TCPConnection, PacketBuffer with looping magic resync, SetReadDeadline, SetLinger(0) close |
| `internal/transport/tcp_client.go`            | TCPClient: seq allocation, pending/streams maps, readLoop demux, unified shutdown |
| `internal/transport/tcp_connection_pool.go`   | ConnectionPool (Acquire with dead-conn eviction, Close), ErrPoolClosed   |
| `internal/transport/future.go`                | Future: idempotent Done, OnComplete, Wait/GetResult variants             |
| `internal/transport/stream.go`                | ClientStreamConn: 64-frame buffer, out-of-band terminal state, drain-before-EOF Recv, ErrStreamNotFound |
| `internal/transport/transport_test.go`        | Future semantics, packet buffer, SendAsync round trip, Close unblocking pending, non-blocking terminal |
| `internal/protocol/header.go`                 | Header struct, CodecType, StreamFlag constants                            |
| `internal/protocol/message.go`                | Magic, Encode/Decode with compression and overflow checks                 |
| `internal/protocol/message_test.go`           | Round trip and malformed-frame tests                                      |
| `internal/codec/codec.go`                     | Codec interface, factory registry (panics on misuse)                     |
| `internal/codec/json.go`, `internal/codec/protobuf.go` | JSON and Protobuf codec implementations                           |
| `internal/codec/compress.go`                  | CompressionType, Gzip compressor, compressor registry                    |
| `internal/codec/codec_test.go`                | Codec round trips, registry panics, compression errors                    |
| `internal/breaker/breaker.go`                 | Three-state circuit breaker with rolling window                           |
| `internal/breaker/breaker_test.go`            | State transition tests including single half-open probe                   |
| `internal/limiter/token_bucket.go`            | Per-second refill token bucket with idempotent Stop                       |
| `internal/limiter/token_bucket_test.go`       | Refill, zero/negative rate, concurrent Allow/Stop                          |
| `internal/load_balance/balancer.go`           | LoadBalancer interface                                                     |
| `internal/load_balance/round_robin.go`        | RoundRobin (atomic, index 0 first)                                         |
| `internal/load_balance/random.go`             | Random (seeded, mutex-guarded)                                             |
| `internal/load_balance/weighted_rr.go`        | Smooth weighted round-robin with weight clamping                           |
| `internal/load_balance/load_balance_test.go`  | Distribution and concurrency tests                                          |
| `internal/registry/registry.go`               | etcd v3 register (lease + keepalive drain), discover cache, watch loop     |
| `internal/registry/registry_test.go`          | Defensive-copy and Close tests                                              |
| `internal/stream/stream.go`                   | ServerStream / ClientStream interfaces (dependency cycle breaker)           |

## Dependencies

- `go.etcd.io/etcd/client/v3 v3.6.7` -- etcd v3 client for service discovery and lease keepalive.
- `google.golang.org/protobuf v1.36.11` -- Protobuf codec implementation.
- Standard library for everything else: `net`, `compress/gzip`, `reflect`, `encoding/json`, `encoding/binary`, `sync`, `sync/atomic`, `context`, `time`, `bufio`, `io`, `math/rand`, `fmt`, `log`.
- Go toolchain requirement: 1.26.0. The `go.mod` contains `replace` directives pointing sibling modules (`swifty_cache`, `swifty_http`, `swifty_orm`) at local paths.

## Cross-references

- `swifty-cache` -- In-process/distributed caching. swifty_rpc has no built-in cache; combine the two for client-side response memoization or for cache-backed service handlers.
- `swifty-http` -- HTTP servers, REST, SSE, WebSocket. swifty_rpc is TCP-only with a custom binary protocol; the two are complementary and non-overlapping.
- `swifty-orm` -- MongoDB access inside RPC service handlers; service methods registered with swifty_rpc commonly use swifty-orm for persistence.
