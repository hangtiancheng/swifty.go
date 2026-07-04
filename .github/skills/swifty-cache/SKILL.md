---
name: swifty-cache
description: >
  Distributed cache framework (github.com/hangtiancheng/swifty.go/swifty_cache).
  Groupcache-style read-through with write propagation. Use when working with
  Group, Getter, GetterFunc, ByteView, Cache, CacheOptions, Store, lruStore,
  StoreOptions, Entry, SingleFlightGroup, ConHashMap, ConHashConfig, PeerPicker,
  Peer, ClientPicker, Client, Server, ServerOptions, Register, RegisterConfig,
  DashboardHandler, StartDashboard, NewGroup, GetGroup, ListGroups, GetAllGroups,
  DestroyGroup, DestroyAllGroups, NewCache, NewStore, NewConHash, NewClientPicker,
  NewClient, NewServer, WithExpiration, WithPeers, WithCacheOptions,
  WithServiceName, WithConHashConfig, WithEtcdEndpoints, WithDialTimeout, WithTLS,
  ErrKeyRequired, ErrValueRequired, ErrGroupClosed, HashBKRD, MaskOfNextPowOf2,
  Now, ValidPeerAddr, or DefaultCacheOptions, DefaultConHashConfig,
  DefaultRegisterConfig, DefaultServerOptions. Also use for cache topology,
  eventual-consistency guarantees, peer synchronization, etcd service discovery,
  consistent hashing with auto-rebalancing, single-flight deduplication, two-level
  bucketed LRU, gRPC cache servers/clients, or dashboard WebSocket endpoints.
  SKIP for plain in-process caches that do not need clustering, for cache libraries
  other than swifty_cache (e.g., groupcache, bigcache, freecache, ristretto), or for
  Redis/Memcached client code.
---

# swifty_cache

A distributed, groupcache-inspired caching framework with gRPC transport, etcd-based
service discovery, consistent hashing with automatic rebalancing, single-flight
request coalescing, a bucketed two-level LRU eviction store, and an optional
real-time WebSocket dashboard. Unlike groupcache, which is strictly read-through,
swifty_cache additionally propagates `Set` and `Delete` to the owning peer with
best-effort, eventually-consistent semantics.

Module path: `github.com/hangtiancheng/swifty.go/swifty_cache`

Source root: `swifty_cache/`

Go toolchain: matches `swifty_cache/go.mod` (currently `go 1.26.0`).

All types live in the `swifty_cache` package (flat structure, with `pb/` as the only
sub-package for generated protobuf code).

## When to load adjacent skills

- For HTTP integration (dashboard, middleware), also load the `swifty-http` skill.
- For RPC integration patterns, also load the `swifty-rpc` skill.
- For MongoDB persistence layers backing a Getter, also load the `swifty-orm` skill.

## Architecture overview

```
Group (namespace, registered globally by name)
  |-- mainCache (*Cache -> *lruStore)
  |     |-- [BucketCount] shards, each with L1 + L2 fixed-cap LRU
  |-- loader   (*SingleFlightGroup)
  |-- peers    (PeerPicker -> *ClientPicker)
                 |-- consHash (*ConHashMap, auto-rebalancer goroutine)
                 |-- clients  (map[addr]*Client, each is a gRPC Peer)
                 |-- etcdCli  (etcd watcher for /services/<svcName>/)

Server (gRPC, one per node)
  |-- pb.SwiftyCacheServer (Get / Set / Delete RPCs)
  |-- grpc health.Server (serves status under svcName)
  |-- Register()  (etcd lease + keepalive on /services/<svcName>/<addr>)

Dashboard (optional, WebSocket)
  |-- StartDashboard(addr) -> swifty_http server on /dashboard/ws
  |-- DashboardHandler() -> mountable middleware for existing swifty_http apps
  |-- pushes groupSnapshot every 2s; accepts {"action":"delete"} commands
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

swifty_cache is best-effort and eventually consistent. Specifically:

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
- Do not use swifty_cache for state that requires strong consistency.

## Core types

### Entry

Exported data structure representing a single cache entry, returned by `Cache.Entries`
and `Group.Entries`, and passed to `Store.Walk` callbacks.

```go
type Entry struct {
    Key      string
    Size     int    // value.Len()
    ExpireAt int64  // nanosecond timestamp; maxExpireAt = math.MaxInt64 for no-expire
    Level    int    // 0 = L1, 1 = L2
}
```

### Value interface

```go
type Value interface {
    Len() int
}
```

All values stored in a `Store` must implement `Value`. `ByteView` satisfies this.

### ByteView

Immutable wrapper for cached byte data. The internal `b []byte` is unexported;
`ByteSlice()` returns a defensive copy on every call.

```go
type ByteView struct {
    b []byte  // unexported
}

func (b ByteView) Len() int         // implements Value
func (b ByteView) ByteSlice() []byte // defensive copy
func (b ByteView) String() string    // standard []byte->string conversion (copies)
```

There is no exported constructor; `ByteView` instances are produced inside the
package from `cloneBytes(raw)`.

### Getter and GetterFunc

```go
type Getter interface {
    Get(ctx context.Context, key string) ([]byte, error)
}

type GetterFunc func(ctx context.Context, key string) ([]byte, error)

func (f GetterFunc) Get(ctx context.Context, key string) ([]byte, error)
```

`GetterFunc` adapts a plain function to the `Getter` interface.

### Group

The cache namespace. Each Group owns a `Getter`, a local `Cache`, an optional
`PeerPicker`, and a `SingleFlightGroup`. Groups are registered in a process-global
`map[name]*Group`; creating a second group with the same name replaces the first
and logs a warning.

Sentinel errors:

```go
var ErrKeyRequired   = errors.New("key is required")
var ErrValueRequired = errors.New("value is required")
var ErrGroupClosed   = errors.New("cache group is closed")
```

Group options:

```go
type GroupOption func(*Group)

func WithExpiration(d time.Duration) GroupOption     // zero = no expiry
func WithPeers(peers PeerPicker) GroupOption         // alternative to RegisterPeers
func WithCacheOptions(opts CacheOptions) GroupOption // override default CacheOptions
```

Constructor and registry functions:

```go
func NewGroup(name string, cacheBytes int64, getter Getter, opts ...GroupOption) *Group
func GetGroup(name string) *Group
func GetAllGroups() map[string]*Group
func ListGroups() []string
func DestroyGroup(name string) bool
func DestroyAllGroups()
```

`NewGroup` panics if `getter == nil`. The `cacheBytes` argument is wired into
`CacheOptions.MaxBytes` but is not enforced as a byte cap (see the caveat under
`CacheOptions`).

Methods:

| Method           | Signature                                               | Notes                                                              |
| ---------------- | ------------------------------------------------------- | ------------------------------------------------------------------ |
| Get              | `(ctx context.Context, key string) (ByteView, error)`   | Read-through; peer failure is not surfaced, only logged            |
| Set              | `(ctx context.Context, key string, value []byte) error` | Local write + async peer sync; rejects empty value                 |
| Delete           | `(ctx context.Context, key string) error`               | Local delete + async peer sync                                     |
| Clear            | `()`                                                    | Removes all local entries; does not propagate                      |
| Close            | `() error`                                              | Idempotent (atomic CAS guard); removes the group from the registry |
| RegisterPeers    | `(PeerPicker)`                                          | Must be called at most once; panics on the second call             |
| Stats            | `() map[string]interface{}`                             | See key list below                                                 |
| DashboardEnabled | `() bool`                                               | True when mainCache has a non-empty DashboardAddr                  |
| Entries          | `() []Entry`                                            | Returns all live entries from the local cache                      |

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

### Cache

Thin wrapper around a `Store` with lazy initialization, hit/miss counters, and
idempotent close.

```go
type CacheOptions struct {
    MaxBytes      int64                            // see caveat below
    BucketCount   uint16                           // rounded up to power of 2; default 16
    CapPerBucket  uint16                           // L1 capacity per bucket; default 512
    Level2Cap     uint16                           // L2 capacity per bucket; default 256
    CleanupTime   time.Duration                    // background expiry sweep; default 1m
    OnEvicted     func(key string, value Value)   // optional eviction callback
    DashboardAddr string                           // if non-empty, starts the dashboard server
}

func DefaultCacheOptions() CacheOptions
func NewCache(opts CacheOptions) *Cache
```

Caveat: `MaxBytes` is accepted but not enforced. The underlying `lruStore` evicts
strictly by per-bucket entry count. To bound memory, size the cache with
`BucketCount * (CapPerBucket + Level2Cap) * avg_entry_size`. Do not rely on
`MaxBytes` for a byte ceiling.

Cache methods:

| Method            | Signature                                            | Notes                                                     |
| ----------------- | ---------------------------------------------------- | --------------------------------------------------------- |
| Add               | `(key string, value ByteView)`                       | No-op on a closed cache                                   |
| AddWithExpiration | `(key string, value ByteView, exp time.Time)`        | Silently drops entries whose `exp` is already in the past |
| Get               | `(ctx context.Context, key string) (ByteView, bool)` | Lazy-initializes the store on first Add                   |
| Delete            | `(key string) bool`                                  | Returns `false` if the cache is closed or uninitialized   |
| Clear             | `()`                                                 | Resets hits/misses; no-op if closed or uninitialized      |
| Len               | `() int`                                             | Walks both LRU levels                                     |
| Close             | `()`                                                 | Idempotent (atomic CAS); releases the store               |
| DashboardEnabled  | `() bool`                                            | True when `DashboardAddr != ""`                           |
| Entries           | `() []Entry`                                         | Returns all live entries via Store.Walk                   |
| Stats             | `() map[string]any`                                  | See key list below                                        |

`Cache.Stats` keys: `initialized`, `closed`, `hits`, `misses`, plus `size` and
`hit_rate` once initialized.

### Store interface and lruStore

```go
type Store interface {
    Get(key string) (Value, bool)
    Set(key string, value Value) error
    SetWithExpiration(key string, value Value, expiration time.Duration) error
    Delete(key string) bool
    Clear()
    Len() int
    Walk(fn func(Entry) bool)
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
- `Walk` iterates L2 then L1, deduplicating via a `seen` map. The callback
  receives `Entry` structs with `Level` set to 0 for L1 entries and 1 for L2.
  Return `false` from the callback to stop iteration early.

Exported helpers (consider these package-internal even though they are exported):

```go
func HashBKRD(s string) int32             // BKDR hash; result can be negative
func MaskOfNextPowOf2(cap uint16) uint16  // next power-of-two minus one
func Now() int64                          // coarse atomic nanosecond clock
func Create(cap uint16) *cache            // raw L1/L2 list factory; do not use outside the package
```

### SingleFlightGroup

```go
type SingleFlightGroup struct {
    m sync.Map  // unexported
}

func (g *SingleFlightGroup) Do(key string, fn func() (interface{}, error)) (interface{}, error)
```

Uses `sync.Map.LoadOrStore` for the lock-free fast path; duplicate callers block
on the leader's `sync.WaitGroup`. The leader stores `(val, err)` on the `call`
struct, then `wg.Done()` releases the followers, all of whom return the same
`(val, err)`. The call is removed from the map via `defer`.

Caveat: no panic recovery. A panic inside `fn` runs `defer wg.Done()` but skips
the assignment, so followers receive `(nil, nil)`. Wrap `fn` if it can panic.

## Distributed layer

### PeerPicker / Peer interfaces

```go
type PeerPicker interface {
    PickPeer(key string) (peer Peer, ok bool, self bool)
    Close() error
}

type Peer interface {
    Get(group string, key string) ([]byte, error)
    Set(ctx context.Context, group string, key string, value []byte) error
    Delete(group string, key string) (bool, error)
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

func WithServiceName(name string) PickerOption   // default: "swifty_cache"

func NewClientPicker(addr string, opts ...PickerOption) (*ClientPicker, error)
func (p *ClientPicker) PickPeer(key string) (Peer, bool, bool)
func (p *ClientPicker) PrintPeers()
func (p *ClientPicker) Close() error
```

`NewClientPicker` connects to etcd using `DefaultRegisterConfig.Endpoints`
(`localhost:2379` by default), performs a one-shot `Get` of `/services/<srv>/`
with a 3-second timeout to seed the peer list, and starts a background watcher
on the same prefix. Put events spawn a new `Client`; delete events close and
remove it. Self-addressed entries are filtered out.

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

The server registers `pb.SwiftyCacheServer` and a gRPC health server (initial
serving status `SERVING` under `svcName`). `Server.Stop` does not flip the health
status to `NOT_SERVING` before stopping; clients may still consider the node
healthy during shutdown.

gRPC handlers:

- `Get(ctx, *pb.Request) (*pb.ResponseForGet, error)` looks up the group by name
  and returns `view.ByteSlice()`. The group name must be registered on the
  receiving node, otherwise the RPC errors.
- `Set(ctx, *pb.Request) (*pb.ResponseForGet, error)` injects `withPeerRequest(ctx)`
  before delegating to `Group.Set`, ensuring the receiving peer does not re-broadcast.
- `Delete(ctx, *pb.Request) (*pb.ResponseForDelete, error)` injects
  `withPeerRequest(ctx)` before delegating to `Group.Delete`.

## Consistent hashing

```go
type ConHashConfig struct {
    DefaultReplicas      int                     // default 50
    MinReplicas          int                     // default 10
    MaxReplicas          int                     // default 200
    HashFunc             func(data []byte) uint32 // default crc32.ChecksumIEEE
    LoadBalanceThreshold float64                 // default 0.25
}

var DefaultConHashConfig = &ConHashConfig{
    DefaultReplicas:      50,
    MinReplicas:          10,
    MaxReplicas:          200,
    HashFunc:             crc32.ChecksumIEEE,
    LoadBalanceThreshold: 0.25,
}

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
  Returns an error if no nodes are provided. Empty-string nodes are silently
  skipped.
- `Remove` returns an error if the node is empty or not found in the ring.
- `Get` does a binary search on the sorted ring; the request count for the
  selected node is incremented under the ring's write lock (`mu.Lock`), so `Get`
  is the hot-path serialization point. Returns `""` if the key is empty or the
  ring is empty.
- `GetStats` returns the traffic share per node as a `map[node]fraction`.
- A goroutine started inside `NewConHash` runs every second. When the total
  request count exceeds 1000 and the maximum per-node deviation from average
  exceeds `LoadBalanceThreshold`, it rebalances replica counts per node, clamped
  to `[MinReplicas, MaxReplicas]`, and resets all counters. The rebalancer has
  no shutdown path; the goroutine leaks until process exit.

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

## Dashboard

The optional real-time dashboard streams cache state over WebSocket.

```go
func DashboardHandler() func(ctx *swifty_http.Context, next func())
func StartDashboard(addr string)
```

`StartDashboard` is guarded by `sync.Once`; only the first invocation takes
effect. It creates a `swifty_http` application listening on `addr` with a single
route `GET /dashboard/ws`. It is called automatically by `NewGroup` when
`CacheOptions.DashboardAddr` is non-empty.

`DashboardHandler` returns a middleware-compatible handler that upgrades the
connection to WebSocket and serves the dashboard protocol. Mount it on an existing
`swifty_http` app if you prefer to share a port.

Protocol:

- Every 2 seconds, the server pushes a `dashboardSnapshot` JSON message containing
  all groups that have `DashboardEnabled() == true`. Each group entry includes its
  `Stats()` map and a list of `entrySnapshot` objects (`Key`, `Size`, `ExpireAt`,
  `Level`).
- A 30-second WebSocket heartbeat (ping) is active.
- The client may send `{"action":"delete","group":"<name>","key":"<key>"}` to
  delete a specific entry from the named group. Unknown actions are logged and
  ignored.

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

service SwiftyCache {
  rpc Get   (Request) returns (ResponseForGet);
  rpc Set   (Request) returns (ResponseForGet);
  rpc Delete(Request) returns (ResponseForDelete);
}
```

Generated constants:

```go
const SwiftyCache_Get_FullMethodName    = "/pb.SwiftyCache/Get"
const SwiftyCache_Set_FullMethodName    = "/pb.SwiftyCache/Set"
const SwiftyCache_Delete_FullMethodName = "/pb.SwiftyCache/Delete"
```

Conventions when extending:

- All three RPCs share `Request`; the `value` field is only populated for `Set`.
- `Set` reuses `ResponseForGet`. A new RPC should introduce a dedicated message
  rather than overloading existing ones.
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

    cache "github.com/hangtiancheng/swifty.go/swifty_cache"
)

func main() {
    ctx := context.Background()
    g := cache.NewGroup("scores", 64<<20, cache.GetterFunc(
        func(ctx context.Context, key string) ([]byte, error) {
            // Load from database, remote API, etc.
            return []byte("score-of-" + key), nil
        },
    ), cache.WithExpiration(5*time.Minute))
    defer g.Close()

    view, err := g.Get(ctx, "player:42")
    if err != nil {
        panic(err)
    }
    fmt.Println(view.String()) // "score-of-player:42"
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

    cache "github.com/hangtiancheng/swifty.go/swifty_cache"
)

func main() {
    ctx := context.Background()

    srv, err := cache.NewServer(
        ":9001", "swifty_cache",
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

    picker, err := cache.NewClientPicker(":9001", cache.WithServiceName("swifty_cache"))
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

### With dashboard enabled

```go
package main

import (
    "context"
    "log"
    "time"

    cache "github.com/hangtiancheng/swifty.go/swifty_cache"
)

func main() {
    opts := cache.DefaultCacheOptions()
    opts.DashboardAddr = ":8080"

    g := cache.NewGroup("inventory", 32<<20, cache.GetterFunc(
        func(ctx context.Context, key string) ([]byte, error) {
            return []byte("item-" + key), nil
        },
    ), cache.WithCacheOptions(opts), cache.WithExpiration(10*time.Minute))
    defer g.Close()

    // Dashboard is now serving at ws://localhost:8080/dashboard/ws
    log.Printf("dashboard enabled: %v", g.DashboardEnabled())

    view, _ := g.Get(context.Background(), "sku:123")
    log.Println(view.String())

    select {} // keep running
}
```

## Testing patterns

Unit tests in this repo do not require a running etcd or gRPC server. Reuse the
existing helpers:

- `group_test.go` defines `fakePeerPicker` (implements `PeerPicker`) and
  `fakePeer` (implements `Peer` with per-method function injection). Use them to
  exercise read fallback, write propagation, and the `peerRequestContextKey`
  contract without touching the network.
- `client_test.go` defines `fakeSwiftyCacheClient` (implements
  `pb.SwiftyCacheClient`). Inject it into `&Client{grpcCli: ...}` to test
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
cd swifty_cache && go test ./...
```

## Pitfalls / known limitations

1. `CacheOptions.MaxBytes` is accepted but not enforced. The underlying `lruStore`
   evicts strictly by per-bucket entry count. Either implement byte accounting or
   rename to `MaxEntries`.
2. `lruStore.cache.del` does not delete the key from L1's `hashMap`, so a key
   can be promoted to L2 multiple times before the slot is recycled.
3. `Client.NewClient` with `etcdCli == nil` leaks the auto-created etcd client on
   `Close`. Always pass a shared `*client_v3.Client`.
4. `ConHashMap.Get` takes a write lock (`mu.Lock`) to increment the stats counter;
   this serializes the hottest path in the system. Consider atomic per-node
   counters.
5. `ConHashMap` has no `Close` method; the rebalancer goroutine leaks until process
   exit.
6. `ClientPicker.handleWatchEvents` calls `NewClient` while holding a write lock;
   topology churn briefly stalls `PickPeer` reads.
7. `SingleFlightGroup` has no panic recovery and no `Forget` / `DoChan` API.
   A panic leaves followers observing `(nil, nil)`.
8. `Server.groups *sync.Map` field is declared but never used; it should be
   removed.
9. `Client.Set` logs every successful response at `log.Printf` level; remove the
   debug log in production builds.
10. The protobuf protocol overloads `Request` and `ResponseForGet` across RPCs;
    new RPCs should introduce dedicated messages.
11. TTLs below ~100 ms are unreliable because the internal clock (`Now()`) is
    advanced in ~100 ms increments by a goroutine in `init()`.
12. `Server.Stop` does not flip the gRPC health status to `NOT_SERVING` before
    calling `GracefulStop`; clients may still route to the node during shutdown.
13. `NewStoreOptions()` defaults `MaxBytes` to 8192 (bytes), disagreeing with
    `DefaultCacheOptions().MaxBytes = 8 MiB`. The discrepancy is moot because
    `MaxBytes` is unused, but confuses readers.
14. `Register` creates its own etcd client, separate from the one inside `Server`;
    three distinct etcd connections exist per node (Register, Server, ClientPicker).
15. `lruStore.Clear()` and `lruStore.Len()` perform `O(N)` linear deduplication
    by walking both LRU levels; avoid them in hot paths for large caches.

## File map

| File                   | Purpose                                                                                                |
| ---------------------- | ------------------------------------------------------------------------------------------------------ |
| `byte_view.go`         | Immutable byte slice value type and `cloneBytes`                                                       |
| `store.go`             | `Value` and `Store` interfaces, `StoreOptions`, `NewStoreOptions`                                      |
| `lru.go`               | Bucketed two-level LRU (`lruStore`), `Entry`, `Now`, `HashBKRD`, `MaskOfNextPowOf2`, cleanup goroutine |
| `cache.go`             | `Cache` wrapper with lazy init, stats, entries, and idempotent close                                   |
| `group.go`             | `Group` type, read-through / write-propagation pipelines, global registry                              |
| `single_flight.go`     | `SingleFlightGroup` (no panic recovery)                                                                |
| `con_hash.go`          | Consistent hash ring with virtual nodes and auto-rebalancer goroutine                                  |
| `config.go`            | `ConHashConfig` and `DefaultConHashConfig`                                                             |
| `peers.go`             | `PeerPicker` / `Peer` interfaces, `ClientPicker` with etcd discovery                                   |
| `server.go`            | gRPC server, health check registration, optional TLS                                                   |
| `client.go`            | gRPC `Peer` implementation                                                                             |
| `register.go`          | `RegisterConfig`, `Register` function, etcd lease + keepalive, IP rewrite                              |
| `dashboard.go`         | WebSocket dashboard: `StartDashboard`, `DashboardHandler`, snapshot push                               |
| `utils.go`             | `ValidPeerAddr` (opt-in helper, not used internally)                                                   |
| `pb/swifty.pb.go`      | Generated protobuf message types                                                                       |
| `pb/swifty_grpc.pb.go` | Generated gRPC client/server stubs and service descriptor                                              |

## Dependencies

- `github.com/hangtiancheng/swifty.go/swifty_http` for the dashboard WebSocket server
- `go.etcd.io/etcd/client/v3` for service discovery and registration
- `google.golang.org/grpc` and `google.golang.org/grpc/health` for transport and
  health checking
- `google.golang.org/protobuf` for message serialization
- Standard library only beyond the above (`hash/crc32`, `math`, `sync`,
  `sync/atomic`, `context`, `time`, `log`, `net`, `crypto/tls`, `sort`,
  `strings`, `errors`, `fmt`)

## Cross-references

- `swifty-http`: The dashboard feature depends on `swifty_http.Context`, `ctx.Upgrade`,
  `swifty_http.UpgradeOptions`, `swifty_http.WSConn`, and `swifty_http.New()` / `app.Listen`.
  Load the swifty-http skill when working on dashboard code or mounting
  `DashboardHandler` on a shared application.
- `swifty-rpc`: For higher-level service patterns that compose swifty_cache groups with
  RPC service definitions.
- `swifty-orm`: For database-backed `Getter` implementations that load cache misses
  from MongoDB via the ORM layer.
