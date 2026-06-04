---
name: lark-cache
description: >
  Distributed cache framework (lark_cache module), groupcache-style read-through with
  write propagation. Use this skill when working on cache namespaces (Group), Getter
  callbacks, two-level bucketed LRU storage, ByteView semantics, single-flight
  deduplication, consistent hashing with auto-rebalancing, gRPC cache servers and
  clients, etcd-based service registration and discovery, peer Set/Delete forwarding,
  or cache statistics. Also use it for any code that imports
  github.com/hangtiancheng/lark-go/lark_cache, or when the user asks about cache
  topology, eventual-consistency guarantees, peer synchronization, or shutdown
  ordering. SKIP for plain in-process caches that do not need clustering, for cache
  libraries other than lark_cache (e.g., groupcache, bigcache, freecache), or for
  Redis client code.
---

# lark_cache

A distributed, groupcache-inspired caching framework with gRPC transport, etcd-based
service discovery, consistent hashing with automatic rebalancing, single-flight
request coalescing, and a bucketed two-level LRU eviction store. Unlike groupcache,
which is strictly read-through, lark_cache additionally propagates `Set` and `Delete`
to the owning peer with best-effort, eventually-consistent semantics.

Module path: `github.com/hangtiancheng/lark-go/lark_cache`

Source root: `lark_cache/`

Go toolchain: matches `lark_cache/go.mod` (currently `go 1.26.0`).

All types live in the `lark_cache` package (flat structure, with `pb/` as the only
sub-package for generated protobuf code).

## When to load adjacent skills

- For HTTP integration in `lark_demo`, also load the `lark-http` skill.
- For RPC integration in `lark_demo`, also load the `lark-rpc` skill.
- For MongoDB persistence in `lark_demo`, also load the `lark-orm` skill.

## Architecture overview

```
Group (namespace, registered globally by name)
  |-- mainCache (*Cache -> *lruStore)
  |-- loader   (*SingleFlightGroup)
  |-- peers    (PeerPicker -> *ClientPicker)
                 |-- consHash (*ConHashMap)
                 |-- clients  (map[addr]*Client, each is a gRPC Peer)
                 |-- etcdCli  (etcd watcher for /services/<svcName>/)

Server (gRPC, one per node)
  |-- pb.LarkCacheServer (Get / Set / Delete RPCs)
  |-- grpc health.Server (lameduck via SetServingStatus)
  |-- Register()  (etcd lease + keepalive on /services/<svcName>/<addr>)
```

Read path (`Group.Get`):

1. Return immediately if the group is closed or the key is empty.
2. Try `mainCache.Get`; on hit, increment `local_hits` and return.
3. Increment `local_misses` and call `Group.load`.
4. `load` uses `SingleFlightGroup.Do` to coalesce concurrent loads of the same key.
5. Inside the singleflight callback (`loadData`):
   - If a `PeerPicker` is set, `PickPeer(key)` selects the owning node.
     - If the owner is self, fall through to the local `Getter`.
     - If the owner is remote, call `peer.Get` (gRPC). On success, increment
       `peer_hits` and return. On failure, log it, increment `peer_misses`, and
       fall back to the local `Getter`.
   - Call `getter.Get(ctx, key)`. On success, increment `loader_hits` and return.
6. Store the resulting `ByteView` in `mainCache` (with TTL if configured) and return.

Write path (`Group.Set` and `Group.Delete`):

1. Reject if closed, missing key, or (for `Set`) empty value (`ErrValueRequired`).
2. Update the local cache.
3. If the request did not originate from a peer (`peerRequestContextKey` not set on
   the context), spawn a goroutine to forward the operation to the owning peer
   via the `PeerPicker`. The forwarded context carries the peer-request marker so
   the receiving `Server.Set` / `Server.Delete` will not re-broadcast and create a
   fan-out loop.

## Consistency model

lark_cache is best-effort and eventually consistent. Specifically:

- `Set` and `Delete` write the local cache synchronously and then fire-and-forget
  an asynchronous RPC to the owning peer. There is no acknowledgement, no retry,
  and no rollback. Failures are logged only.
- Non-owning peers do not learn of the change at all; their cached copies converge
  only via TTL expiry. To bound staleness, always configure `WithExpiration`.
- There is no read-repair: if the owner is unreachable, the caller falls back to
  its local `Getter` and caches the value locally, which may diverge from other
  peers until TTL.
- Topology changes (etcd put/delete events) repoint the hash ring; in-flight keys
  may briefly hash to the wrong owner.
- Do not use lark_cache for state that requires strong consistency.

## Core types

### Group

The cache namespace. Each Group owns a `Getter`, a local `Cache`, an optional
`PeerPicker`, and a `SingleFlightGroup`. Groups are registered in a process-global
`map[name]*Group`; creating a second group with the same name replaces the first
and logs a warning.

```go
type Getter interface {
    Get(ctx context.Context, key string) ([]byte, error)
}
type GetterFunc func(ctx context.Context, key string) ([]byte, error)
```

The `Getter` must not panic. `SingleFlightGroup.Do` has no panic recovery; a panic
inside the getter leaves waiting callers observing `(nil, nil)` and silently
suppresses the failure. Recover internally if the loader can panic.

Group options:

```go
type GroupOption func(*Group)

func WithExpiration(d time.Duration) GroupOption       // zero = no expiry
func WithPeers(peers PeerPicker) GroupOption           // alternative to RegisterPeers
func WithCacheOptions(opts CacheOptions) GroupOption   // override default CacheOptions
```

Constructor and registry:

```go
func NewGroup(name string, cacheBytes int64, getter Getter, opts ...GroupOption) *Group
func GetGroup(name string) *Group
func ListGroups() []string
func DestroyGroup(name string) bool
func DestroyAllGroups()
```

`NewGroup` panics if `getter == nil`. The `cacheBytes` argument is wired into
`CacheOptions.MaxBytes` but is not enforced as a byte cap (see the caveat under
`CacheOptions`).

Methods:

| Method        | Signature                                               | Notes                                                              |
| ------------- | ------------------------------------------------------- | ------------------------------------------------------------------ |
| Get           | `(ctx context.Context, key string) (ByteView, error)`   | Read-through; peer failure is not surfaced, only logged            |
| Set           | `(ctx context.Context, key string, value []byte) error` | Local write + async peer sync; rejects empty value                 |
| Delete        | `(ctx context.Context, key string) error`               | Local delete + async peer sync                                     |
| Clear         | `()`                                                    | Removes all local entries; does not propagate                      |
| Close         | `() error`                                              | Idempotent (atomic CAS guard); removes the group from the registry |
| RegisterPeers | `(PeerPicker)`                                          | Must be called at most once; panics on the second call             |
| Stats         | `() map[string]interface{}`                             | See key list below                                                 |

Sentinel errors: `ErrKeyRequired`, `ErrValueRequired`, `ErrGroupClosed`.

`Group.Stats` keys (always present unless noted):

```
name              string  - group name
closed            bool
expiration        time.Duration
loads             int64
local_hits        int64
local_misses      int64
peer_hits         int64
peer_misses       int64
loader_hits       int64
loader_errors     int64
hit_rate          float64  // only present if local_hits + local_misses > 0
avg_load_time_ms  float64  // only present if loads > 0
cache_<key>       any      // every key from Cache.Stats prefixed with "cache_"
```

### ByteView

Immutable wrapper for cached byte data. The internal `b []byte` is unexported;
`ByteSlice()` returns a defensive copy on every call.

```go
func (b ByteView) Len() int
func (b ByteView) ByteSlice() []byte   // defensive copy
func (b ByteView) String() string      // standard []byte->string conversion (copies)
```

Implements `Value` (`Len() int`). There is no exported constructor; `ByteView`
instances are produced inside the package from `cloneBytes(raw)`.

### Cache

Thin wrapper around a `Store` with lazy initialization, hit/miss counters, and
idempotent close.

```go
type CacheOptions struct {
    MaxBytes     int64                                  // see caveat below
    BucketCount  uint16                                 // rounded up to a power of 2; default 16
    CapPerBucket uint16                                 // L1 capacity per bucket; default 512
    Level2Cap    uint16                                 // L2 capacity per bucket; default 256
    CleanupTime  time.Duration                          // background expiry sweep; default 1m
    OnEvicted    func(key string, value Value)         // optional eviction callback
}

func DefaultCacheOptions() CacheOptions
func NewCache(opts CacheOptions) *Cache
```

Caveat: `MaxBytes` is accepted but not enforced. The underlying `lruStore` evicts
strictly by per-bucket entry count. To bound memory, size the cache with
`BucketCount * (CapPerBucket + Level2Cap) * avg_entry_size`. Do not rely on
`MaxBytes` for a byte ceiling.

Caveat: `NewStoreOptions()` (in `store.go`) defaults `MaxBytes` to 8192 (bytes),
which disagrees with `DefaultCacheOptions().MaxBytes = 8 MiB`. The discrepancy is
moot today because `MaxBytes` is unused; mention it if you ever wire it up.

Cache methods:

| Method            | Signature                                            | Notes                                                     |
| ----------------- | ---------------------------------------------------- | --------------------------------------------------------- |
| Add               | `(key string, value ByteView)`                       | No-op on a closed cache                                   |
| AddWithExpiration | `(key string, value ByteView, exp time.Time)`        | Silently drops entries whose `exp` is already in the past |
| Get               | `(ctx context.Context, key string) (ByteView, bool)` | Lazy-initializes the store on first hit                   |
| Delete            | `(key string) bool`                                  | Returns `false` if the cache is closed or uninitialized   |
| Clear             | `()`                                                 | Resets hits/misses; no-op if closed or uninitialized      |
| Len               | `() int`                                             | Walks both LRU levels                                     |
| Close             | `()`                                                 | Idempotent (atomic CAS); releases the store               |
| Stats             | `() map[string]any`                                  | See key list below                                        |

`Cache.Stats` keys: `initialized`, `closed`, `hits`, `misses`, plus `size` and
`hit_rate` once initialized.

### Store interface and lruStore

```go
type Value interface { Len() int }

type Store interface {
    Get(key string) (Value, bool)
    Set(key string, value Value) error
    SetWithExpiration(key string, value Value, expiration time.Duration) error
    Delete(key string) bool
    Clear()
    Len() int
    Close()
}

type StoreOptions struct {
    MaxBytes        int64
    BucketCount     uint16
    CapPerBucket    uint16
    Level2Cap       uint16
    CleanupInterval time.Duration
    OnEvicted       func(key string, value Value)
}

func NewStoreOptions() StoreOptions   // defaults (note MaxBytes=8192, see caveat)
func NewStore(opts StoreOptions) *lruStore
```

The `lruStore` implements a bucketed two-level LRU:

- Keys hash to one of `BucketCount` shards via `HashBKRD(key) & mask`, where `mask`
  is `MaskOfNextPowOf2(BucketCount)` (so `BucketCount` is effectively rounded up
  to the next power of two).
- Each shard holds two independent fixed-capacity LRUs: L1 (`CapPerBucket`, the
  entry point) and L2 (`Level2Cap`, populated on access).
- On `Set` / `SetWithExpiration`, the entry is inserted into L1; if L1 is full,
  the LRU tail of L1 is recycled and (when `OnEvicted != nil` and the tail had a
  non-zero `expireAt`) the eviction callback fires.
- On `Get`, L1 is checked first. If present, the L1 entry is logically retired
  (its `expireAt` is zeroed and it is moved to the L1 LRU tail; the slot stays
  registered in L1's `hashMap` until the slot is later recycled by a `put` that
  overflows L1) and a copy is inserted into L2. If L1 misses, L2 is queried; on
  L2 hit the node is moved to L2's MRU end.
- Expired entries (`Now() >= expireAt`) are filtered on read and are also swept
  by a background cleanup goroutine every `CleanupInterval` (default 1 minute).
- Time is read via `Now() int64`, a coarse atomic clock advanced every ~100 ms
  by a goroutine started in `init()`. TTLs below ~100 ms are not reliable.
- `Clear` and `Len` walk both levels and perform `O(N)` linear deduplication
  between L1 and L2; avoid calling them on large caches in hot paths.

Exported helpers (consider these package-internal even though they are exported):

```go
func HashBKRD(s string) int32             // BKDR hash; result can be negative
func MaskOfNextPowOf2(cap uint16) uint16  // next power-of-two minus one
func Now() int64                          // coarse atomic nanosecond clock
func Create(cap uint16) *cache            // raw L1/L2 list factory; do not use outside the package
```

## Distributed layer

### PeerPicker / Peer interfaces

```go
type PeerPicker interface {
    PickPeer(key string) (peer Peer, ok bool, self bool)
    Close() error
}

type Peer interface {
    Get(group, key string) ([]byte, error)
    Set(ctx context.Context, group, key string, value []byte) error
    Delete(group, key string) (bool, error)
    Close() error
}
```

`PickPeer` returns `(nil, false, false)` when the ring is empty,
`(peer, true, true)` when `key` hashes to the local node, and
`(peer, true, false)` for a remote owner. `Group` uses the `self` flag to skip
the network round-trip.

### ClientPicker (etcd-backed PeerPicker)

```go
type PickerOption func(*ClientPicker)
func WithServiceName(name string) PickerOption   // default: "lark_cache"

func NewClientPicker(addr string, opts ...PickerOption) (*ClientPicker, error)
func (p *ClientPicker) PickPeer(key string) (Peer, bool, bool)
func (p *ClientPicker) PrintPeers()
func (p *ClientPicker) Close() error
```

`NewClientPicker` connects to etcd using `DefaultRegisterConfig.Endpoints`
(`localhost:2379` by default), performs a one-shot `Get` of `/services/<svc>/`
with a 3-second timeout to seed the peer list, and starts a background watcher
on the same prefix. Put events spawn a new `Client`; delete events close and
remove it. Self-addressed entries are filtered out.

Caveat: the watcher handler runs `NewClient` while holding the picker's write
lock. Topology churn briefly blocks `PickPeer` calls; do not assume non-blocking
peer-list reads.

Caveat: `ConHashMap` (the ring used by `ClientPicker`) starts a per-second
rebalancer goroutine in `NewConHash` that has no shutdown path. `ClientPicker.Close`
closes etcd and gRPC connections but does not stop the rebalancer.

### Client (gRPC Peer)

```go
func NewClient(addr, svcName string, etcdCli *client_v3.Client) (*Client, error)
func (c *Client) Get(group, key string) ([]byte, error)
func (c *Client) Set(ctx context.Context, group, key string, value []byte) error
func (c *Client) Delete(group, key string) (bool, error)
func (c *Client) Close() error
```

Behavior notes:

- `Get` and `Delete` build their own `context.WithTimeout(context.Background(), 3*time.Second)`
  and discard any caller context. Only `Set` honors the caller `ctx`.
- The transport is `grpc.NewClient` with `insecure.NewCredentials()` and
  `WaitForReady(true)`. There is no TLS, no retry, no circuit breaker.
- If `etcdCli == nil`, `NewClient` lazily creates its own etcd client
  (`localhost:2379`, 5s dial timeout). `Client.Close` only closes the gRPC
  connection, never the auto-created etcd client. Always pass a shared etcd
  client to avoid this leak.

### Server (gRPC)

```go
type ServerOptions struct {
    EtcdEndpoints []string
    DialTimeout   time.Duration
    MaxMsgSize    int          // sets MaxRecvMsgSize only; MaxSendMsgSize is gRPC default
    TLS           bool
    CertFile      string
    KeyFile       string
}

var DefaultServerOptions = &ServerOptions{
    EtcdEndpoints: []string{"localhost:2379"},
    DialTimeout:   5 * time.Second,
    MaxMsgSize:    4 << 20,
}

type ServerOption func(*ServerOptions)
func WithEtcdEndpoints(endpoints []string) ServerOption
func WithDialTimeout(timeout time.Duration) ServerOption
func WithTLS(certFile, keyFile string) ServerOption

func NewServer(addr, svcName string, opts ...ServerOption) (*Server, error)
func (s *Server) Start() error    // blocks until grpcServer.Serve returns
func (s *Server) Stop()           // idempotent (sync.Once); GracefulStop + close etcd
```

The server registers `pb.LarkCacheServer` and a gRPC health server (initial
serving status `SERVING` under `svcName`). `Server.Stop` does not flip the health
status to `NOT_SERVING` before stopping; clients may still consider the node
healthy during shutdown.

gRPC handlers (`server.go`):

- `Get` looks up the group by name and returns `view.ByteSlice()`. The group
  name must be registered on the receiving node, otherwise the RPC errors.
- `Set` and `Delete` always inject `withPeerRequest(ctx)` before delegating to
  `Group.Set` / `Group.Delete`, ensuring the receiving peer does not re-broadcast.

Caveat: `DefaultServerOptions` is a mutable global pointer. Do not mutate it from
business code; copy from `*DefaultServerOptions` if you need a custom baseline.

## Consistent hashing

```go
type ConHashConfig struct {
    DefaultReplicas      int      // default 50
    MinReplicas          int      // default 10
    MaxReplicas          int      // default 200
    HashFunc             func(data []byte) uint32   // default crc32.ChecksumIEEE
    LoadBalanceThreshold float64  // default 0.25
}
var DefaultConHashConfig *ConHashConfig  // mutable global; treat as read-only

type ConHashOption func(*ConHashMap)
func WithConHashConfig(config *ConHashConfig) ConHashOption

func NewConHash(opts ...ConHashOption) *ConHashMap
func (m *ConHashMap) Add(nodes ...string) error
func (m *ConHashMap) Remove(node string) error
func (m *ConHashMap) Get(key string) string
func (m *ConHashMap) GetStats() map[string]float64
```

Behavior:

- `Add` inserts `DefaultReplicas` virtual nodes per physical node, keyed by
  `HashFunc([]byte(fmt.Sprintf("%s-%d", node, i)))`, then re-sorts the ring.
- `Get` does a binary search on the sorted ring; the request count for the
  selected node is incremented under the ring's write lock (`mu.Lock`), so `Get`
  is the hot-path serialization point.
- A goroutine started inside `NewConHash` runs every second. When the total
  request count exceeds 1000 and the maximum per-node deviation from average
  exceeds `LoadBalanceThreshold`, it rebalances replica counts per node, clamped
  to `[MinReplicas, MaxReplicas]`, and resets all counters. The rebalancer has
  no shutdown path; the goroutine leaks until process exit.

## Single flight

```go
type SingleFlightGroup struct{ /* sync.Map of in-flight calls */ }

func (g *SingleFlightGroup) Do(key string, fn func() (interface{}, error)) (interface{}, error)
```

Uses `sync.Map.LoadOrStore` for the lock-free fast path; duplicate callers block
on the leader's `sync.WaitGroup`. The leader stores `(val, err)` on the `call`
struct, then `wg.Done()` releases the followers, all of whom return the same
`(val, err)`. The call is removed from the map via `defer`.

Caveat: no panic recovery. A panic inside `fn` runs `defer wg.Done()` but skips
the assignment, so followers receive `(nil, nil)`. Wrap `fn` if it can panic.

## Service registration

```go
type RegisterConfig struct {
    Endpoints   []string
    DialTimeout time.Duration
}
var DefaultRegisterConfig = &RegisterConfig{
    Endpoints:   []string{"localhost:2379"},
    DialTimeout: 5 * time.Second,
}

func Register(svcName, addr string, stopCh <-chan error) error
```

`Server.Start` calls `Register` in a goroutine; you rarely need to invoke it
directly. Behavior:

- Creates a new etcd client (separate from `Server.etcdCli` and
  `ClientPicker.etcdCli`; this is a known duplication).
- If `addr` begins with `:` (e.g., `":9001"`), it is rewritten to
  `<firstNonLoopbackIPv4>:<port>` before being written to etcd. The
  `ClientPicker.selfAddr` you pass to `NewClientPicker` must match the rewritten
  form, otherwise the picker will treat the local node as remote and every local
  cache miss will round-trip through gRPC back to itself (the loop is broken by
  the peer-request marker, but you pay one extra round trip per miss).
- Grants a 10-second lease and writes `/services/<svcName>/<addr>` with that
  lease. `KeepAlive` renews indefinitely.
- On `stopCh` close, revokes the lease with a 3-second timeout.

## Utilities

```go
func ValidPeerAddr(addr string) bool
```

Weak validator: accepts `localhost:<port>` or dotted-quad IPv4 with any non-empty
port string. Does not validate port range or support IPv6. Not called from
`Register` / `ClientPicker` / `Server`; treat it as opt-in.

## Protobuf surface (`pb/`)

```protobuf
message Request          { string group = 1; string key = 2; bytes value = 3; }
message ResponseForGet   { bytes value = 1; }
message ResponseForDelete{ bool  value = 1; }

service LarkCache {
  rpc Get   (Request) returns (ResponseForGet);
  rpc Set   (Request) returns (ResponseForGet);   // reuses ResponseForGet
  rpc Delete(Request) returns (ResponseForDelete);
}
```

Conventions when extending:

- All three RPCs share `Request`; the `value` field is only populated for `Set`.
- `Set` reuses `ResponseForGet`. Mirror this if you must, but a new RPC should
  introduce a dedicated message rather than overloading existing ones.
- The protocol has no metadata fields (trace id, tenant, version). Add them
  through gRPC metadata interceptors rather than the message body.

## Typical usage

### Single-node, in-process

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
    g := cache.NewGroup("scores", 64<<20, cache.GetterFunc(
        func(ctx context.Context, key string) ([]byte, error) {
            return []byte("score-of-" + key), nil
        },
    ), cache.WithExpiration(5*time.Minute))
    defer g.Close()

    view, err := g.Get(ctx, "player:42")
    if err != nil {
        panic(err)
    }
    fmt.Println(view.String())
}
```

### Distributed, with etcd

Recommended start/stop ordering:

1. `NewServer` and `srv.Start()` in a goroutine. Wait briefly (or poll etcd) so
   the registration entry exists before constructing the picker.
2. `NewClientPicker(selfAddr)`; pass the same `addr` you handed to `NewServer`
   (already rewritten if it began with `:`).
3. `NewGroup(...)` followed by `g.RegisterPeers(picker)`.
4. On shutdown: `g.Close()` -> `picker.Close()` -> `srv.Stop()`.

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

    srv, err := cache.NewServer(
        ":9001", "lark_cache",
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

    // Allow Register to complete its first Put before the picker scans etcd.
    time.Sleep(200 * time.Millisecond)

    picker, err := cache.NewClientPicker(":9001", cache.WithServiceName("lark_cache"))
    if err != nil {
        log.Fatal(err)
    }
    defer picker.Close()

    g := cache.NewGroup("scores", 64<<20, cache.GetterFunc(
        func(ctx context.Context, key string) ([]byte, error) {
            return []byte("loaded-" + key), nil
        },
    ), cache.WithExpiration(time.Minute))
    defer g.Close()
    g.RegisterPeers(picker)

    view, err := g.Get(ctx, "player:42")
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("value=%s stats=%+v", view.String(), g.Stats())
}
```

## Testing patterns

Unit tests in this repo do not require a running etcd or gRPC server. Reuse the
existing helpers:

- `group_test.go` defines `fakePeerPicker` (implements `PeerPicker`) and
  `fakePeer` (implements `Peer` with per-method function injection). Use them to
  exercise read fallback, write propagation, and the `peerRequestContextKey`
  contract without touching the network.
- `client_test.go` defines `fakeLarkCacheClient` (implements
  `pb.LarkCacheClient`). Inject it into `&Client{grpcCli: ...}` to test
  `Client.Get` / `Set` / `Delete` happy paths and error paths.
- For end-to-end gRPC tests, use `google.golang.org/grpc/test/bufconn` to avoid
  binding to TCP ports.
- For etcd-backed flows, prefer `go.etcd.io/etcd/server/v3/embed` to run an
  embedded etcd in-process.

Always start tests with `DestroyAllGroups()` because `NewGroup` registers into a
process-global map; leftover registrations from earlier tests will leak across
table entries.

Run the suite from the module root:

```
cd lark_cache && go test ./...
```

## Troubleshooting

- "Every local Get takes ~3 seconds and finally returns the local value":
  topology has a stale entry pointing at a downed peer, or `selfAddr` does not
  match the rewritten registration address. Check `ClientPicker.PrintPeers()`
  and reconcile with `Register`'s address-rewrite rule.
- "`SingleFlightGroup` callers get `(nil, nil)`": the `Getter` panicked. Wrap
  the loader body with `defer recover` and surface the panic as an error.
- "etcd connections grow unbounded over time": `Client.NewClient` was called with
  `etcdCli == nil`; the auto-created etcd client is leaked on every `Client.Close`.
  Pass a shared `*client_v3.Client` instead.
- "Cache memory grows beyond `MaxBytes`": `MaxBytes` is not enforced. Reduce
  `BucketCount`, `CapPerBucket`, or `Level2Cap` to bound entries.
- "TTLs under 100 ms behave erratically": expected. The internal clock advances
  in ~100 ms increments.
- "Health check still reports SERVING during shutdown": `Server.Stop` does not
  call `healthServer.SetServingStatus(svcName, NOT_SERVING)`. Add it before
  `Stop` if you depend on lameduck behavior.

## File map

| File               | Purpose                                                                    |
| ------------------ | -------------------------------------------------------------------------- |
| `byte_view.go`     | Immutable byte slice value type and `cloneBytes`                           |
| `store.go`         | `Value` and `Store` interfaces, `StoreOptions`, `NewStoreOptions`          |
| `lru.go`           | Bucketed two-level LRU (`lruStore`), `Now`, `HashBKRD`, `MaskOfNextPowOf2` |
| `cache.go`         | `Cache` wrapper with lazy init, stats, and idempotent close                |
| `group.go`         | `Group` type, read-through / write-propagation pipelines, registry         |
| `single_flight.go` | `SingleFlightGroup` (no panic recovery)                                    |
| `con_hash.go`      | Consistent hash ring with virtual nodes and auto-rebalancer goroutine      |
| `config.go`        | `ConHashConfig` and `DefaultConHashConfig`                                 |
| `peers.go`         | `PeerPicker` / `Peer` interfaces, `ClientPicker` with etcd discovery       |
| `server.go`        | gRPC server, health check registration, optional TLS                       |
| `client.go`        | gRPC `Peer` implementation                                                 |
| `register.go`      | etcd lease registration and keepalive; rewrites `:port` to `<ip>:port`     |
| `utils.go`         | `ValidPeerAddr` (opt-in helper, not used internally)                       |
| `pb/`              | `lark.proto` and generated gRPC code                                       |

## Dependencies

- `go.etcd.io/etcd/client/v3` for service discovery and registration
- `google.golang.org/grpc` and `google.golang.org/grpc/health` for transport and
  health checking
- `google.golang.org/protobuf` for message serialization
- Standard library only beyond the above (`hash/crc32`, `sync`, `sync/atomic`,
  `context`, `time`, `log`)

## Known limitations to surface in code review

These are not bugs the agent should silently work around; flag them in PR
reviews and propose fixes:

1. `CacheOptions.MaxBytes` is unenforced. Either implement byte accounting or
   rename to `MaxEntries`.
2. `lruStore.cache.del` does not delete the key from L1's `hashMap`, so a key
   can be promoted to L2 multiple times before the slot is recycled.
3. `Client.NewClient` leaks the auto-created etcd client on `Close`.
4. `ConHashMap.Get` takes a write lock for stats increment; the ring is the
   hottest path in the system.
5. `ConHashMap` has no `Close`; the rebalancer goroutine leaks.
6. `ClientPicker.handleWatchEvents` calls `NewClient` while holding a write
   lock; topology churn briefly stalls `PickPeer`.
7. `SingleFlightGroup` has no panic recovery and no `Forget` / `DoChan` API.
8. `Server.groups *sync.Map` field is declared but never used; remove it.
9. `Client.Set` logs every response at `log.Printf` level; remove the debug log.
10. The pb package overloads `Request` and `ResponseForGet` across RPCs; new
    RPCs should use dedicated messages.
