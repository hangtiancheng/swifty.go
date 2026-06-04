---
name: lark-rpc
description: >
  TCP-based RPC framework for Go (lark_rpc module,
  github.com/hangtiancheng/lark-go/lark_rpc) with a grpc-go-style public API,
  request multiplexing over a single connection, server-streaming, pluggable
  codecs (JSON / Protobuf) with mandatory Gzip body compression, connection
  pooling, three-state circuit breaking, token-bucket rate limiting,
  RoundRobin / Random / smooth-weighted load balancing, and etcd v3 service
  discovery. Use when writing or modifying Go code that calls rpc.NewServer,
  server.Register, server.Serve / GracefulStop / Stop, rpc.Dial,
  ClientConn.Invoke / NewStream / Close, rpc.WithCodec / WithTimeout /
  WithDialCodec / WithRegistry / WithLoadBalancer, rpc.NewRegistry, the
  ServerStream / ClientStream interfaces, or any import of
  github.com/hangtiancheng/lark-go/lark_rpc. Also use when the user asks about
  the wire protocol (Magic + HeaderLen + BodyLen + Header + Body framing),
  Header.StreamFlag (StreamNone / StreamData / StreamEnd / StreamError),
  multiplexing by RequestID, the reflection-based service dispatcher (grpc-go
  vs net/rpc method signatures), server-streaming handlers, registry mode vs
  static mode behavioural differences, circuit-breaker tuning, or migrating
  net/rpc / grpc-go code to lark_rpc. Do not use for grpc-go itself, gRPC
  gateways, Connect-Go, Twirp, net/rpc code that does not import lark_rpc, or
  for HTTP / SSE work (use lark_http instead).
---

# lark_rpc

A TCP-based RPC framework with a grpc-go-style public API. Provides request
multiplexing on a single connection, server-streaming, pluggable JSON / Protobuf
codecs with mandatory Gzip body compression, connection pooling, three-state
circuit breaking, token-bucket rate limiting, load balancing, and etcd v3
service registration / discovery.

- Module path: `github.com/hangtiancheng/lark-go/lark_rpc`
- Source root: `lark_rpc/`
- Go toolchain: 1.26+
- Public API lives in `pkg/rpc`; everything else is `internal/` and not
  importable by downstream code.

## Architecture overview

```
pkg/
  rpc/      Public API (grpc-go style)
    Server     -- NewServer, Register, Serve, GracefulStop, Stop
    ClientConn -- Dial, Invoke, NewStream, Close
    ServerStream / ClientStream  (type aliases of internal/stream)
  api/      Example service (Arith) used by tests and demos

internal/
  server/         TCP server loop, reflection-based dispatch, server-streaming
  client/         Registry-mode client with breaker / limiter / LB / pool
  transport/      TCPConnection, ConnectionPool, TCPClient, Future, ClientStreamConn
  protocol/       Header + Message framing (length-delimited binary)
  codec/          Codec interface + JSON / Protobuf + Gzip compressor
  breaker/        Three-state circuit breaker (Closed -> Open -> HalfOpen)
  limiter/        Token-bucket rate limiter (per-second refill)
  load_balance/   LoadBalancer interface + RoundRobin, Random, WeightedRR
  registry/       etcd v3 service registration, discovery, watch
  stream/         ServerStream / ClientStream interface definitions
```

Two strict boundaries to keep in mind:

1. `pkg/rpc` re-exports a minimal set of types from `internal/codec`,
   `internal/registry`, and `internal/load_balance` via `type alias`. Downstream
   code must never import `internal/*` directly.
2. The `internal/stream` package exists only to break a potential cyclic
   dependency between `internal/server` and `internal/transport`. Both packages
   refer to `stream.ServerStream` / `stream.ClientStream`, never to each other.

## Public API (pkg/rpc)

### Server

```go
func NewServer(opts ...ServerOption) *Server

func (s *Server) Register(name string, service interface{})
func (s *Server) Serve(lis net.Listener) error
func (s *Server) GracefulStop()
func (s *Server) Stop()
```

Conventions match grpc-go: the constructor takes options (not an address), the
caller supplies the `net.Listener`, and shutdown is split into `GracefulStop`
(closes the listener, waits for in-flight goroutines, then stops the limiter)
and `Stop` (same plus forcibly closes every live connection). Both are
idempotent (guarded by `sync.Once`).

Available `ServerOption`:

| Option         | Description                           |
| -------------- | ------------------------------------- |
| `WithCodec(t)` | Set the body codec (JSON or Protobuf) |

Notes:

- `NewServer` panics if an option is invalid; option errors are treated as
  programmer errors, not runtime errors.
- Every server instance installs a fixed-rate token bucket of 10 000 RPS
  (`limiter.NewTokenBucket(10000)`), shared across all connections and
  methods. There is no option to tune it; requests over the limit receive a
  response with `Header.Error = "rate limit exceeded"`.
- `Register` may be called concurrently with `Serve`; it takes the server
  mutex.

### ClientConn

```go
func Dial(target string, opts ...DialOption) (*ClientConn, error)

func (cc *ClientConn) Invoke(ctx context.Context, service, method string, args, reply interface{}) error
func (cc *ClientConn) NewStream(ctx context.Context, service, method string, args interface{}) (ClientStream, error)
func (cc *ClientConn) Close() error
```

`ClientConn` supports two distinct modes selected at `Dial` time:

- Static mode (default): direct `ConnectionPool` to a single target address.
- Registry mode (`WithRegistry(reg)`): the `target` argument is ignored;
  addresses are resolved via etcd and selected by the configured
  `LoadBalancer`.

Available `DialOption`:

| Option                 | Description                                           |
| ---------------------- | ----------------------------------------------------- |
| `WithTimeout(d)`       | Per-call timeout, default 5s, applied via context     |
| `WithDialCodec(t)`     | Codec for request and reply bodies (JSON or Protobuf) |
| `WithRegistry(reg)`    | Enable registry mode (etcd v3 service discovery)      |
| `WithLoadBalancer(lb)` | Load-balancing strategy for registry mode             |

Important behavioural cliff between the two modes:

| Capability                 | Static mode  | Registry mode                                       |
| -------------------------- | ------------ | --------------------------------------------------- |
| Codec / compression        | yes          | yes                                                 |
| Request multiplexing       | yes          | yes                                                 |
| Per-call timeout           | yes          | yes                                                 |
| Connection pool            | yes (1 conn) | yes (1 conn per addr)                               |
| Service discovery          | no           | etcd v3                                             |
| Load balancing             | no           | yes                                                 |
| Token-bucket rate limiting | no           | yes (10 000 RPS, hard-coded)                        |
| Circuit breaker            | no           | yes (10-window, 60% threshold, 5s open, hard-coded) |

Static mode goes through `pkg/rpc/client.go::invokeStatic`, which talks to a
`transport.ConnectionPool` directly and bypasses `internal/client.Client`
entirely. Registry mode delegates to `internal/client.Client.Invoke` /
`InvokeStream`.

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

Only server-streaming is implemented. Client-streaming and bidirectional
streaming are not supported by the current `StreamFlag` set; the protocol has
no client-originated stream frame after the initial request.

### Re-exported types and constants

```go
type CodecType    = codec.Type
type Registry     = registry.Registry
type Instance     = registry.Instance
type LoadBalancer = load_balance.LoadBalancer

var CodecJSON  = codec.JSON   // value 1
var CodecProto = codec.PROTO  // value 2

func NewRegistry(endpoints []string) (*Registry, error)
```

`pkg/rpc` does not re-export a server-side helper that registers the local
listener address into etcd. Today the only ways to publish a service instance
are to write a thin wrapper package or to add a re-export in `pkg/rpc`;
importing `internal/registry` directly is forbidden by Go.

## Internal components

### Wire protocol (internal/protocol)

Binary, length-delimited, big-endian:

```
+--------+-----------+---------+----------------+--------------+
| Magic  | HeaderLen | BodyLen |  Header(JSON)  | Body(bytes)  |
| 2 byte | 4 byte    | 4 byte  |    N byte      |    M byte    |
+--------+-----------+---------+----------------+--------------+

Magic = 0x1234
```

`PacketBuffer.Read` walks the byte stream looking for the magic; on mismatch it
advances by one byte and retries, which is a simple resync mechanism.
`Decode` rejects packets whose `10 + headerLen + bodyLen` exceeds `math.MaxInt`.

Header layout (declared in `internal/protocol/header.go`):

```go
type Header struct {
    RequestID   uint64
    ServiceName string
    MethodName  string
    Error       string
    CodecType   CodecType                  // protocol.CodecType, not codec.Type
    Compression codec.CompressionType
    StreamFlag  StreamFlag `json:",omitempty"`
}

type StreamFlag byte
const (
    StreamNone  StreamFlag = 0
    StreamData  StreamFlag = 1
    StreamEnd   StreamFlag = 2
    StreamError StreamFlag = 3
)
```

Two type-system facts that bite if missed:

- `protocol.CodecType` and `codec.Type` are two independent types that happen
  to share the same numeric encoding (1 = JSON, 2 = Proto). Headers carry the
  protocol-local one.
- `StreamFlag` is JSON-encoded with `omitempty`, so `StreamNone` does not
  appear in serialised headers. When debugging packet dumps, a missing
  `StreamFlag` key means "normal unary response", not "malformed packet".

The header is always JSON-encoded on the wire (regardless of the body codec).
The body is always Gzip-compressed: every send site in the repo hard-codes
`Compression: codec.CompressionGzip` and there is no option to disable it.

### Codec (internal/codec)

```go
type Codec interface {
    Marshal(v interface{}) ([]byte, error)
    Unmarshal(data []byte, v interface{}) error
}

type Type byte
const (
    JSON  Type = 1
    PROTO Type = 2
)

type CompressionType byte
const (
    CompressionNone CompressionType = 0
    CompressionGzip CompressionType = 1
)
```

Codecs are registered through an `init`-time factory map keyed by `Type`. The
Protobuf codec requires `v` to implement `proto.Message`; sending a plain
struct under `CodecProto` fails at Marshal time with
`proto codec: not proto.Message`.

### Transport (internal/transport)

- `TCPConnection` wraps a `net.Conn` with a `PacketBuffer` for frame
  extraction. `Read() (*protocol.Message, error)` and
  `Write(msg *protocol.Message) error`. `Close` calls `SetLinger(0)` so closing
  produces a TCP RST rather than FIN.
- `TCPClient` owns one `TCPConnection` plus two `sync.Map` registries:
  `pending` (RequestID -> *Future) for unary calls, and `streams`
  (RequestID -> *ClientStreamConn) for server-streams. `nextSeq` uses
  `atomic.AddUint64` starting at 1; RequestID 0 is never produced by the
  client. `SendAsync` / `SendAsyncWithCodec` return a `*Future`; `SendStream`
  returns a `*ClientStreamConn`. A single `readLoop` goroutine demultiplexes
  every response.
- `ConnectionPool` is a pool of `*TCPClient` keyed by address. Constructed as
  `NewConnectionPool(addr, maxIdle, maxActive)`, but `maxIdle` is silently
  dropped (never stored) and every call site passes `maxActive = 1`. In
  practice, one pool = one TCP connection multiplexed across all callers.
  `Acquire` holds the pool mutex across `net.DialTimeout` (5s), so concurrent
  callers serialise behind cold-start dials.
- `Future` exposes both raw-byte and decoded-reply consumers:
  `Wait() ([]byte, error)`, `WaitWithContext(ctx)`, `WaitWithTimeout(d)`,
  `GetResult(reply)`, `GetResultWithContext(ctx, reply)`, `DoneChan()`,
  `OnComplete(fn)`, `IsDone()`, and the producer-side `Done(res, err)`.
  `OnComplete` invokes the callback immediately if the future is already
  complete; otherwise it stores the callback for `Done` to invoke.
  `internal/client/invoke.go::InvokeAsync` uses `DoneChan()` plus a manual
  `future.Done(nil, context.DeadlineExceeded)` race to implement async
  timeout.
- `ClientStreamConn` is backed by a buffered channel of cap 64. `Recv`
  decodes the next frame; `Push` is called by `readLoop` for `StreamData`;
  `End` and `Error` are guarded by `sync.Once` so they fire exactly one
  terminal frame.

`TCPClient.readLoop` switches on `Header.StreamFlag`:

- `StreamData` -> `streams[seq].Push(body)`.
- `StreamEnd` -> `streams[seq].End()` and remove.
- `StreamError` -> `streams[seq].Error(err)` and remove.
- default (`StreamNone`) -> `pending[seq].Done(body, err)` and remove.

When the TCP read fails, `fail(err)` is invoked exactly once
(`atomic.CompareAndSwapInt32`) and resolves every outstanding Future and
stream with the error, preventing goroutine leaks.

### Server (internal/server)

`Server.Serve` is the canonical accept loop:

```
for { conn := lis.Accept(); go s.Handle(transport.NewTCPConnection(conn)) }
```

Each `Handle` invocation: read a message, check the limiter, look up the
service by `Header.ServiceName`, delegate to `Handler.Process`. Rate-limit
rejections are written back as `Header.Error = "rate limit exceeded"` with an
empty body.

`Handler.invoke` dispatches by reflecting on the registered service value.
There are two outer branches:

1. `numIn == 2 && numOut == 2` and `In(0).Implements(contextType)` and
   `In(1).Kind() == Ptr` and `Out(1).Implements(errorType)` -> grpc-go
   unary: `Method(ctx context.Context, req *T) (*R, error)`. The `contextType`
   check uses `Implements`, not equality, so any type satisfying the context
   interface qualifies.
2. `numIn == 2 && numOut == 1` and `Out(0).Implements(errorType)` ->
   either streaming or net/rpc unary, distinguished by an inner check on
   `In(1)`:
   - `In(1).Implements(stream.ServerStream)` -> streaming:
     `Method(req *T, stream ServerStream) error`. `invoke` constructs a
     `serverStream`, calls the method, and emits a `StreamEnd` (success) or
     `StreamError` (error) terminal frame automatically.
   - `In(1).Kind() == Ptr` -> net/rpc unary:
     `Method(req *T, reply *R) error`. The reply is allocated via
     `reflect.New(In(1).Elem())`.

Any other shape returns `unsupported method signature: <Service>.<Method>`.

Streaming success path: business method returns nil -> framework emits
`StreamEnd`. Streaming error path: business method returns non-nil ->
framework emits `StreamError` with `Header.Error = err.Error()`.

### Client (internal/client)

`internal/client.Client` is the registry-mode implementation. It composes:

- A `*registry.Registry` for discovery.
- A `load_balance.LoadBalancer` (default `&load_balance.RoundRobin{}`).
- A `limiter.TokenBucket(10 000)` shared across all addresses.
- `pools sync.Map` (addr -> \*ConnectionPool).
- `breaker sync.Map` (key `service|addr` -> \*CircuitBreaker, created lazily
  with `windowSize=10, threshold=0.6, openTimeout=5s`).

`Invoke` wraps the supplied context with `context.WithTimeout(ctx, c.timeout)`,
then delegates to `invokeAsync` and waits with
`future.GetResultWithContext(callCtx, reply)`. The async path registers an
`OnComplete` callback that calls `RecordSuccess` or `RecordFailure` on the
breaker. `InvokeStream` only records a failure at send time; mid-stream errors
are not currently fed back into the breaker.

`getAddr` calls `registry.Discover(service)`. The first call triggers
`initService` (full Get + start a Watch goroutine); subsequent calls hit the
in-memory cache.

### Circuit breaker (internal/breaker)

```go
func NewCircuitBreaker(windowSize int, failureThreshold float64, openTimeout time.Duration) *CircuitBreaker
func (cb *CircuitBreaker) Allow() bool
func (cb *CircuitBreaker) RecordSuccess()
func (cb *CircuitBreaker) RecordFailure()
func (cb *CircuitBreaker) State() State

type State int
const (
    Closed   State = 0
    Open     State = 1
    HalfOpen State = 2
)
```

Behaviour:

- Closed: count successes and failures. When `success + failure >=
windowSize`, compute `failure / total`; if it meets `failureThreshold`,
  transition to Open and reset counters; otherwise reset and start a fresh
  window.
- Open: reject all `Allow()` calls until `openTimeout` has elapsed since the
  last state change. After that, the next `Allow()` returns true (single
  probe) and transitions to HalfOpen.
- HalfOpen: only one in-flight probe is permitted at a time
  (`halfOpenProbe` flag). A successful probe -> Closed; a failed probe -> Open.

Parameters are not exposed via `pkg/rpc`. The client always uses
`(10, 0.6, 5s)`.

### Rate limiter (internal/limiter)

```go
func NewTokenBucket(rate int) *TokenBucket
func (tb *TokenBucket) Allow() bool
func (tb *TokenBucket) Stop()
```

The bucket is refilled to `rate` once per second by a background goroutine
(not a continuous leaky-bucket rate). Burst capacity equals `rate`. `Stop`
shuts the goroutine down idempotently via `sync.Once`.

### Load balancer (internal/load_balance)

```go
type LoadBalancer interface {
    Select([]registry.Instance) registry.Instance
}
```

Built-ins:

- `RoundRobin{}` -- lock-free, `atomic.AddUint64` on an internal counter.
- `Random` (use `NewRandom()`) -- `math/rand` guarded by a mutex.
- `WeightedRR` (use `NewWeightedRR(weights)`) -- Nginx-style smooth weighted
  round-robin. The weight slice is indexed positionally against the
  `[]Instance` argument. Combined with `registry.copyInstances` (which iterates
  a `map[string]Instance` and therefore returns instances in unspecified
  order), `WeightedRR` cannot be safely composed with the etcd registry
  without an extra stable-sorting layer.

### Service registry (internal/registry)

```go
func NewRegistry(endpoints []string) (*Registry, error)
func (r *Registry) Register(service string, ins Instance, ttl int64) error
func (r *Registry) Discover(service string) ([]Instance, error)
func (r *Registry) Close() error

type Instance struct {
    Addr string
}
```

- `Register` grants a lease of `ttl` seconds, writes
  `<prefix><service>/<addr> = <addr>` under that lease, and starts a
  KeepAlive goroutine that drains the lease channel but does nothing on
  failure. Lease expiry is not surfaced to the caller.
- `Discover` lazily initialises a per-service cache and a background watch
  goroutine. Subsequent calls return a copy of the cached map values, in
  unspecified order.
- There is no exported `Watch` method; consumers cannot subscribe to instance
  changes externally.
- The etcd key prefix is hard-coded to
  `/github.com/hangtiancheng/lark-go/lark_rpc/services/`. There is no
  namespace / environment parameter.

### Stream interfaces (internal/stream)

Exists solely to break the `server` <-> `transport` import cycle.

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

`pkg/rpc/stream.go` re-exports both via `type alias`.

## Typical usage

### Server (grpc-go style)

```go
type MathService struct{}

func (s *MathService) Add(ctx context.Context, args *Args) (*Reply, error) {
    return &Reply{Result: args.A + args.B}, nil
}

server := rpc.NewServer()
server.Register("Math", &MathService{})

lis, err := net.Listen("tcp", ":8080")
if err != nil {
    log.Fatal(err)
}

errCh := make(chan error, 1)
go func() { errCh <- server.Serve(lis) }()

// Wait on signals or whatever else, then:
server.GracefulStop()
if err := <-errCh; err != nil && !errors.Is(err, net.ErrClosed) {
    log.Fatal(err)
}
```

### Client (static)

```go
conn, err := rpc.Dial(":8080", rpc.WithTimeout(5*time.Second))
if err != nil { log.Fatal(err) }
defer conn.Close()

var reply Reply
if err := conn.Invoke(ctx, "Math", "Add", &Args{A: 1, B: 2}, &reply); err != nil {
    log.Fatal(err)
}
```

### Client (registry mode)

```go
reg, err := rpc.NewRegistry([]string{"localhost:2379"})
if err != nil { log.Fatal(err) }

conn, err := rpc.Dial(
    "",                              // target is ignored in registry mode
    rpc.WithRegistry(reg),
    rpc.WithTimeout(3*time.Second),
    // rpc.WithLoadBalancer(load_balance.NewRandom()), // optional
)
if err != nil { log.Fatal(err) }
defer conn.Close()

var reply Reply
err = conn.Invoke(ctx, "Math", "Add", &Args{A: 1, B: 2}, &reply)
```

### Server-streaming

```go
// Server side
func (s *FeedService) Subscribe(req *SubArgs, stream rpc.ServerStream) error {
    for msg := range feed {
        if err := stream.Send(msg); err != nil {
            return err // framework emits StreamError automatically
        }
    }
    return nil // framework emits StreamEnd automatically
}

// Client side
stream, err := conn.NewStream(ctx, "Feed", "Subscribe", &SubArgs{Topic: "news"})
if err != nil { log.Fatal(err) }

for {
    var msg Message
    if err := stream.Recv(&msg); err != nil {
        if errors.Is(err, io.EOF) {
            break
        }
        log.Fatal(err)
    }
    process(msg)
}
```

## Method signature cheat sheet

`Register(name, svc)` reflects on every exported method of `svc`. Three shapes
are recognised; everything else is rejected at call time.

| Shape                                             | Style     | Notes                                                           |
| ------------------------------------------------- | --------- | --------------------------------------------------------------- |
| `Method(ctx context.Context, req *T) (*R, error)` | grpc-go   | Recommended. `*R == nil` returns an empty body.                 |
| `Method(req *T, reply *R) error`                  | net/rpc   | Backward compatible. Reply is allocated by the framework.       |
| `Method(req *T, stream rpc.ServerStream) error`   | streaming | Server-streaming only. Terminal frame is emitted automatically. |

## Framework-level error strings

These travel in `Header.Error` and are returned verbatim by `Invoke` /
`NewStream` (wrapped as `errors.New(headerError)`):

- `service not found: <Service>`
- `method not found: <Service>.<Method>`
- `unsupported method signature: <Service>.<Method>`
- `rate limit exceeded` (server-side bucket exhausted)
- `circuit breaker open` (registry-mode client breaker tripped)
- `no instance available` (registry returned an empty set)
- `connection closed` (TCPClient already failed)
- `registry not configured` (client.Invoke without a registry)

## Pitfalls

- Body Gzip compression is mandatory. Every internal send site sets
  `Compression: codec.CompressionGzip`. For small payloads the 18-byte gzip
  header can dwarf the body. There is currently no option to disable it.
- Server-wide hard cap of 10 000 RPS via `limiter.NewTokenBucket(10000)`,
  not exposed as a `ServerOption`. Identical cap on the registry-mode client.
- Circuit-breaker parameters `(10, 0.6, 5s)` are hard-coded in
  `internal/client/resolver.go::getBreaker`. There is no public way to tune
  them.
- Static-mode `ClientConn` bypasses `internal/client.Client`, so it has
  neither rate limiting, breaker, nor load balancing. Do not promise those
  capabilities to a caller who has not also passed `WithRegistry`.
- `ConnectionPool` is single-connection (`maxActive = 1` at every call site).
  All requests on a `ClientConn` to a given address share one TCP socket and
  one `readLoop`. A slow stream consumer that fills its 64-frame buffer
  blocks `readLoop`, which stalls every other future and stream on that
  connection. Treat lark_rpc as "fast consumers only" until this is fixed.
- `Acquire` holds the pool mutex across `net.DialTimeout` (5s). Cold-start
  storms serialise.
- `WeightedRR.Select` requires the `[]Instance` argument and the configured
  weight slice to align positionally. `registry.copyInstances` returns
  instances in `map` iteration order, which is randomised. Combining
  `WeightedRR` with the etcd registry directly will misroute traffic.
- `NewServer` panics on invalid options; never feed it untrusted configuration
  without a recover.
- `TCPConnection.Close` calls `SetLinger(0)`, which produces a TCP RST. A
  client mid-Read on a graceful-stop server may see "connection reset by peer"
  instead of EOF.
- Only server-streaming exists. The `StreamFlag` enum has no codepoints for
  client-originated stream frames. Do not attempt bidirectional streaming
  with this framework as it stands.
- `pkg/rpc` does not expose a server-side `Register-into-etcd` helper. To
  publish a service instance, the only options today are an internal package
  wrapper or to add a re-export in `pkg/rpc`.
- `pending` Futures have no internal timeout. Always wait with
  `GetResultWithContext` (or use the timeout passed to `Dial`) or you risk
  leaking the entry until the connection dies.

## When not to use this skill

- Anything that imports `google.golang.org/grpc` directly. That is grpc-go,
  not lark_rpc; the surface looks similar but the wire is incompatible.
- Connect-Go, Twirp, net/rpc standard library, or HTTP-based RPC.
- HTTP / SSE / WebSocket work. Use the `lark-http` skill instead.
- Pure Protobuf schema / codegen questions unrelated to lark_rpc's reflection
  dispatcher.

## File map

| Directory                | Files                                                                                    | Purpose                                                               |
| ------------------------ | ---------------------------------------------------------------------------------------- | --------------------------------------------------------------------- |
| `pkg/rpc/`               | `rpc.go`, `server.go`, `client.go`, `stream.go`                                          | Public API: type aliases, Server, ClientConn, stream interfaces       |
| `pkg/api/`               | `arith.go`                                                                               | Example Arith service used by tests                                   |
| `internal/server/`       | `server.go`, `handler.go`, `options.go`, `stream.go`                                     | TCP server loop, reflection dispatch, server-side stream emitter      |
| `internal/client/`       | `client.go`, `invoke.go`, `option.go`, `resolver.go`                                     | Registry-mode client, Invoke / InvokeAsync / InvokeStream, breaker/LB |
| `internal/transport/`    | `tcp_connection.go`, `tcp_connection_pool.go`, `tcp_client.go`, `future.go`, `stream.go` | TCP framing, pool, multiplexed client, Future, ClientStreamConn       |
| `internal/protocol/`     | `header.go`, `message.go`                                                                | Wire format: Header struct, Magic, Encode / Decode                    |
| `internal/codec/`        | `codec.go`, `json.go`, `protobuf.go`, `compress.go`                                      | Codec registry, JSON / Protobuf codecs, Gzip compressor               |
| `internal/breaker/`      | `breaker.go`                                                                             | Three-state circuit breaker                                           |
| `internal/limiter/`      | `token_bucket.go`                                                                        | Per-second refill token bucket                                        |
| `internal/load_balance/` | `balancer.go`, `round_robin.go`, `random.go`, `weighted_rr.go`                           | LoadBalancer interface and three built-in strategies                  |
| `internal/registry/`     | `registry.go`                                                                            | etcd v3 register / discover / watch                                   |
| `internal/stream/`       | `stream.go`                                                                              | ServerStream / ClientStream interfaces (cycle-breaker)                |

## Dependencies

- `go.etcd.io/etcd/client/v3` -- service discovery and lease keepalive.
- `google.golang.org/protobuf` -- Protobuf codec.
- Standard library only for everything else (TCP, gzip, reflection, JSON).
