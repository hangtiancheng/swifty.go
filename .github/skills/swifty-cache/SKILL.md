---
name: swifty-cache
description: >
  Distributed groupcache-style cache for Go (module
  github.com/hangtiancheng/swifty.go/swifty_cache): read-through Group with a
  byte-budgeted two-level LRU store, singleflight deduplication, consistent
  hashing, etcd service discovery, gRPC transport, and a WebSocket dashboard.
  Use when working with Group, NewGroup, GetGroup, Getter, GetterFunc,
  ByteView, Cache, CacheOptions, Store, StoreOptions, NewStore, Entry,
  SingleFlightGroup, ConHashMap, ConHashConfig, PeerPicker, Peer,
  ClientPicker, NewClientPicker, Client, NewClient, Server, NewServer,
  ServerOptions, Register, RegisterConfig, DashboardHandler, StartDashboard,
  WithExpiration, WithPeers, WithCacheOptions, WithServiceName,
  WithConHashConfig, WithEtcdEndpoints, WithDialTimeout, ErrKeyRequired,
  ErrValueRequired, ErrGroupClosed, HashBKRD, MaskOfNextPowOf2, Now,
  ValidPeerAddr, cache topology, peer sync, or byte eviction. Do NOT use for
  groupcache, bigcache, ristretto, Redis/Memcached client code, or plain
  in-process caches that need no clustering.
---

# swifty_cache

A distributed, groupcache-aligned caching framework with gRPC transport,
etcd-based service discovery, consistent hashing with a stable (fixed-replica)
ring, single-flight request coalescing, a bucketed two-level LRU store with a
real byte budget, and an optional real-time WebSocket dashboard. Unlike
groupcache, which is strictly read-through, swifty_cache additionally
propagates `Set` and `Delete` to the owning peer with best-effort, eventually
consistent semantics.

Module path: `github.com/hangtiancheng/swifty.go/swifty_cache`

Source root: `swifty_cache/`

Go toolchain: matches `swifty_cache/go.mod` (currently `go 1.26.0`).

All types live in the `swifty_cache` package (flat structure, with `pb/` as
the only sub-package for generated protobuf code).

## When to load adjacent skills

- For HTTP integration (dashboard, middleware), also load the `swifty-http` skill.
- For RPC integration patterns, also load the `swifty-rpc` skill.
- For MongoDB persistence layers backing a Getter, also load the `swifty-orm` skill.

## Architecture overview

```
Group (namespace, registered globally by name; duplicate name panics)
  |-- mainCache (*Cache -> *lruStore)
  |     |-- [BucketCount] shards, each with L1 + L2 fixed-cap LRU
  |     |-- per-bucket byte budget = MaxBytes / bucketCount
  |-- loader   (*SingleFlightGroup, panic-recovering)
  |-- peers    (PeerPicker -> *ClientPicker)
                 |-- consHash (*ConHashMap, fixed replicas, no rebalancer)
                 |     `-- always contains selfAddr (self ownership works)
                 |-- clients  (map[addr]*Client, each is a gRPC Peer)
                 |-- etcdCli  (watcher on /services/<svcName>/ with
                               resync + revision continuation + backoff)

Server (gRPC, one per node)
  |-- pb.SwiftyCacheServer (Get / Set / Delete RPCs; all inject the
  |     peer-request marker so peer traffic is never re-forwarded)
  |-- grpc health.Server (SERVING under svcName; NOT_SERVING on Stop)
  |-- registerWithConfig()  (etcd lease + keepalive + auto re-register
        with exponential backoff on /services/<svcName>/<addr>)

Dashboard (optional, WebSocket)
  |-- StartDashboard(addr) -> swifty_http server on GET /dashboard/ws
  |-- DashboardHandler() -> mountable handler for existing swifty_http apps
  |-- pushes a snapshot every 2s; accepts {"action":"delete"} commands
```

Read path (`Group.Get`):

1. Return `ErrGroupClosed` if closed, `ErrKeyRequired` if the key is empty.
2. Increment `gets`. Try `mainCache.Get`; on hit, increment `local_hits` and return.
3. Increment `local_misses` and call `Group.load`.
4. `load` runs `SingleFlightGroup.Do(key, fn)`. Inside the callback (leader only):
   - Re-check `mainCache.Get` (double-check: two serial callers can both miss
     before the first one populates). On hit, return the cached view.
   - Increment `loads_deduped` and call `loadData`:
     - If a `PeerPicker` is set and the request did NOT originate from a peer
       (`peerRequestContextKey` absent), `PickPeer(key)` selects the owner.
       - Owner is self (`self == true`): fall through to the local `Getter`.
       - Owner is remote: call `peer.Get` (gRPC). On success, increment
         `peer_hits` and return; on failure, increment `peer_misses`, log,
         and fall back to the local `Getter`.
     - Call `getter.Get(ctx, key)`; on success, increment `loader_hits`.
   - Store the resulting `ByteView` in `mainCache` (with TTL when configured)
     inside the callback, so only the leader populates once.
5. `load` records the load duration and `loads`; on error it increments
   `loader_errors`. The result is type-asserted with an `ok` check; a
   non-`ByteView` result becomes an error, never a panic.

Write path (`Group.Set` and `Group.Delete`):

1. Reject if closed, missing key, or (for `Set`) empty value (`ErrValueRequired`).
2. Update the local cache synchronously (with TTL when `WithExpiration` is set).
3. If the request did not originate from a peer, spawn a goroutine
   (`syncToPeers`) that picks the owner; if the owner is remote and non-nil,
   forward the operation with a 3-second timeout (`peerSyncTimeout`) on a
   context carrying the peer-request marker, so the receiving node does not
   re-broadcast. Failures are logged only.

## Consistency model

swifty_cache is best-effort and eventually consistent:

- `Set` and `Delete` write locally, then fire-and-forget one RPC (3s timeout,
  no retry, no rollback) to the owning peer. Failures are only logged.
- Non-owning peers never learn of the change; their cached copies converge
  only via TTL expiry. To bound staleness, always configure `WithExpiration`.
- There is no read-repair: if the owner is unreachable, the caller falls back
  to its local `Getter` and caches the value locally.
- Values fetched from a remote peer are cached unconditionally in the local
  `mainCache` (there is no probabilistic hotCache as in groupcache), so
  non-owner replicas are common; TTL is the only convergence mechanism.
- Topology changes repoint the hash ring; in-flight keys may briefly hash to
  the wrong owner. Peer-originated requests are always answered locally,
  which prevents forwarding loops during ring divergence.
- Do not use swifty_cache for state that requires strong consistency.

## Core types

### Entry

Exported data structure representing a single cache entry, returned by
`Cache.Entries` and `Group.Entries`, and passed to `Store.Walk` callbacks.

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

All values stored in a `Store` must implement `Value`. `ByteView` satisfies it.

### ByteView

Immutable wrapper for cached byte data. The internal `b []byte` is unexported;
`ByteSlice()` returns a defensive copy on every call.

```go
type ByteView struct {
    b []byte  // unexported
}

func (b ByteView) Len() int          // implements Value
func (b ByteView) ByteSlice() []byte // defensive copy
func (b ByteView) String() string    // []byte -> string conversion (copies)
```

There is no exported constructor; `ByteView` instances are produced inside the
package from `cloneBytes(raw)`. External packages cannot construct a non-empty
`ByteView` directly (in-package tests use the struct literal).

### Getter and GetterFunc

```go
type Getter interface {
    Get(ctx context.Context, key string) ([]byte, error)
}

type GetterFunc func(ctx context.Context, key string) ([]byte, error)

func (f GetterFunc) Get(ctx context.Context, key string) ([]byte, error)
```

`GetterFunc` adapts a plain function to the `Getter` interface. A panic inside
the getter is recovered by the singleflight layer and surfaces as an error
from `Group.Get`.

### Group

The cache namespace. Each Group owns a `Getter`, a local `Cache`, an optional
`PeerPicker` (guarded by an RWMutex), and a `SingleFlightGroup`. Groups are
registered in a process-global `map[name]*Group`. Creating a second group with
the same name panics with `"duplicate registration of group <name>"`
(groupcache semantics).

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
func WithCacheOptions(opts CacheOptions) GroupOption // replaces the default Cache
```

Note: `WithCacheOptions` replaces the whole `mainCache`, discarding the
`MaxBytes = cacheBytes` value that `NewGroup` wired in; set
`CacheOptions.MaxBytes` yourself when using this option.

Constructor and registry functions:

```go
func NewGroup(name string, cacheBytes int64, getter Getter, opts ...GroupOption) *Group
func GetGroup(name string) *Group
func GetAllGroups() map[string]*Group
func ListGroups() []string
func DestroyGroup(name string) bool
func DestroyAllGroups()
```

`NewGroup` panics if `getter == nil` and panics on a duplicate name. The
`cacheBytes` argument becomes `CacheOptions.MaxBytes` and is enforced as a
byte budget by the store (split evenly across buckets). If
`CacheOptions.DashboardAddr` is non-empty, `NewGroup` calls
`StartDashboard(addr)`.

Methods:

| Method           | Signature                                               | Notes                                                             |
| ---------------- | ------------------------------------------------------- | ----------------------------------------------------------------- |
| Get              | `(ctx context.Context, key string) (ByteView, error)`   | Read-through; peer failure is not surfaced, only logged            |
| Set              | `(ctx context.Context, key string, value []byte) error` | Local write + async peer sync (3s timeout); rejects empty value    |
| Delete           | `(ctx context.Context, key string) error`               | Local delete + async peer sync (3s timeout)                        |
| Clear            | `()`                                                    | Removes all local entries; does not propagate                      |
| Close            | `() error`                                              | Idempotent (atomic CAS); removes the group from the registry       |
| RegisterPeers    | `(PeerPicker)`                                          | At most once; panics on the second call (also counts `WithPeers`)  |
| Stats            | `() map[string]interface{}`                             | See key list below                                                 |
| DashboardEnabled | `() bool`                                               | True when mainCache has a non-empty DashboardAddr                  |
| Entries          | `() []Entry`                                            | All live entries from the local cache; nil when closed             |

`Group.Stats` keys (always present unless noted):

```
name              string
closed            bool
expiration        time.Duration
gets              int64    - total Get calls (after validation)
loads             int64    - singleflight Do invocations
loads_deduped     int64    - loads that actually ran loadData (leader, cache-missed)
local_hits        int64
local_misses      int64
peer_hits         int64
peer_misses       int64
loader_hits       int64
loader_errors     int64
server_requests   int64    - gRPC Get requests received from peers (Server.Get)
hit_rate          float64  - only when local_hits + local_misses > 0
avg_load_time_ms  float64  - only when loads > 0
cache_<key>       any      - every key from Cache.Stats prefixed with "cache_"
```

### Cache

Thin wrapper around a `Store` with lazy initialization, hit/miss counters, and
idempotent close. All store accesses take the internal mutex and nil-check the
store, so racing `Add`/`Get`/`Delete` against `Close` is safe (covered by
`TestCacheConcurrentUseWithClose`).

```go
type CacheOptions struct {
    MaxBytes      int64                          // byte budget, split per bucket; default 8 MiB
    BucketCount   uint16                         // rounded up to power of 2; default 16
    CapPerBucket  uint16                         // L1 entry capacity per bucket; default 512
    Level2Cap     uint16                         // L2 entry capacity per bucket; default 256
    CleanupTime   time.Duration                  // background expiry sweep; default 1m
    OnEvicted     func(key string, value Value)  // optional eviction callback
    DashboardAddr string                         // if non-empty, NewGroup starts the dashboard
}

func DefaultCacheOptions() CacheOptions
func NewCache(opts CacheOptions) *Cache
```

Cache methods:

| Method            | Signature                                            | Notes                                                     |
| ----------------- | ---------------------------------------------------- | ---------------------------------------------------------- |
| Add               | `(key string, value ByteView)`                       | No-op on a closed cache; lazily initializes the store       |
| AddWithExpiration | `(key string, value ByteView, exp time.Time)`        | Silently drops entries whose `exp` is already in the past   |
| Get               | `(ctx context.Context, key string) (ByteView, bool)` | Counts a miss when uninitialized                            |
| Delete            | `(key string) bool`                                  | `false` if closed or uninitialized                          |
| Clear             | `()`                                                 | Resets hits/misses; no-op if closed or uninitialized        |
| Len               | `() int`                                             | Deduplicated count across both LRU levels                   |
| Close             | `()`                                                 | Idempotent (atomic CAS); nils out and closes the store      |
| DashboardEnabled  | `() bool`                                            | True when `DashboardAddr != ""`                             |
| Entries           | `() []Entry`                                         | All live entries via Store.Walk; nil when closed            |
| Stats             | `() map[string]any`                                  | See key list below                                          |

`Cache.Stats` keys: `initialized`, `closed`, `hits`, `misses`, plus `size`,
`hit_rate`, `bytes` (current live bytes, keys + values), and `evictions`
(cumulative capacity/byte-budget evictions) once initialized. `bytes` and
`evictions` are only emitted when the store is a `*lruStore` (the default).

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
    MaxBytes        int64          // byte budget; <= 0 disables byte eviction
    BucketCount     uint16
    CapPerBucket    uint16
    Level2Cap       uint16
    CleanupInterval time.Duration
    OnEvicted       func(key string, value Value)
}

func NewStoreOptions() StoreOptions   // defaults: 8 MiB, 16, 512, 256, 1m
func NewStore(opts StoreOptions) *lruStore
```

`NewStore` fills zero values with the same defaults (16 / 512 / 256 / 1m).
The `lruStore` (unexported type, returned concretely) also exposes:

```go
func (s *lruStore) Bytes() int64      // total live bytes (keys + values) across buckets
func (s *lruStore) Evictions() int64  // cumulative capacity/byte-budget evictions
```

Behavior:

- Keys hash to one of `BucketCount` shards via `HashBKRD(key) & mask`, where
  `mask = MaskOfNextPowOf2(BucketCount)` (`BucketCount` is effectively rounded
  up to the next power of two). Each shard is guarded by its own mutex.
- Each shard holds two independent fixed-capacity LRUs: L1 (`CapPerBucket`,
  the write entry point) and L2 (`Level2Cap`, populated on L1 read).
- Byte budget: when `MaxBytes > 0`, each bucket gets
  `MaxBytes / bucketCount` bytes (minimum 1). After every `Set` /
  `SetWithExpiration`, entries are evicted LRU-first from L1 and then from L2
  (`evictOldest`) until the bucket's `L1.bytes + L2.bytes` fits the budget.
  Every eviction increments the eviction counter and fires `OnEvicted`.
- `Set` / `SetWithExpiration` writes to L1 and then drops any stale copy of
  the key from L2. L1 is the single write authority; the old value can no
  longer resurface from L2 (covered by `TestLRUStoreSetInvalidatesL2Copy`).
- On `Get`, L1 is checked first. An L1 hit is logically retired from L1 (its
  `expireAt` is zeroed, the value reference released) and re-inserted into L2;
  the L1 slot stays registered in the hash map until recycled by a later
  overflowing `put`. If L1 misses, L2 is peeked; on a live L2 hit the node is
  moved to L2's MRU end.
- Expired entries are detected on read: an expired L1 or L2 hit deletes both
  copies and fires `OnEvicted` exactly once (covered by
  `TestLRUStoreExpiredReadFiresOnEvicted`). A background cleanup goroutine
  also sweeps expired keys every `CleanupInterval` (default 1 minute) and
  exits when `Close` is called (`closeCh` + `sync.Once`).
- `Clear`, `Len`, and `Walk` deduplicate keys between L1 and L2 with a map
  (O(N) per bucket, under the bucket lock). `Walk` iterates L2 first, then
  L1 entries not already seen; `Level` is 0 for L1 and 1 for L2. Return
  `false` from the callback to stop iteration within the current traversal.
- Time is read via `Now() int64`, a coarse atomic clock advanced every
  ~100 ms by a goroutine started in `init()`; it resynchronizes with the real
  clock roughly once per second. TTLs below ~200 ms are not reliable.

Exported helpers (treat as package-internal even though exported):

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

Uses `sync.Map.LoadOrStore` for the lock-free fast path; duplicate concurrent
callers block on the leader's `sync.WaitGroup` and receive the same
`(val, err)`. The call is removed from the map via `defer`, so serial callers
re-execute `fn` (which is why `Group.load` double-checks the cache inside the
callback).

A panic inside `fn` is recovered and converted into an error
(`"singleflight: panic during call: <value>"`) that every caller receives;
the key is released so subsequent calls work (covered by
`TestSingleFlightPanicIsRecovered`). There is no `Forget` or `DoChan` API.

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

`PickPeer` returns `(nil, false, false)` when the ring is empty or the owner
has no client, `(nil, true, true)` when `key` hashes to the local node, and
`(peer, true, false)` for a remote owner. `Group` uses the `self` flag to skip
the network round-trip and load locally.

### ClientPicker (etcd-backed PeerPicker)

```go
type PickerOption func(*ClientPicker)

func WithServiceName(name string) PickerOption   // default: "swifty_cache"

func NewClientPicker(addr string, opts ...PickerOption) (*ClientPicker, error)
func (p *ClientPicker) PickPeer(key string) (Peer, bool, bool)
func (p *ClientPicker) PrintPeers()
func (p *ClientPicker) Close() error
```

Behavior:

- `addr` starting with `:` (e.g. `":9001"`) is normalized to
  `<firstNonLoopbackIPv4>:<port>`, exactly matching the rewrite `Register`
  performs before writing to etcd, so the local node recognizes its own
  registration.
- The local address is always added to the hash ring at construction, so keys
  owned by the local node are loaded locally (`PickPeer` returns
  `(nil, true, true)`).
- Connects to etcd using `DefaultRegisterConfig` (`localhost:2379`, 5s dial
  timeout by default; there is no per-picker etcd endpoint option — mutate
  `DefaultRegisterConfig` before construction if needed).
- Startup performs a full `Get` of `/services/<svc>/` (3s timeout) to seed
  the peer list, then a background goroutine watches the prefix starting from
  the snapshot revision + 1 (no event gap). If the watch channel breaks
  (etcd restart, compaction), the picker waits 1 second, resyncs the full
  peer set (`fetchAllServices`, which also removes vanished peers and closes
  their clients), and re-establishes the watch from the new revision.
- Watch events derive the peer address from the etcd key
  (`addrFromEventKey`), never from the value, because DELETE events carry an
  empty value. Put events create a `Client` and add the address to the ring;
  delete events close the client and remove the address. Self-addressed
  entries are skipped (self is already permanently in the ring).
- `handleWatchEvents` and `fetchAllServices` hold the picker's write lock
  while creating clients; `grpc.NewClient` does not block on connectivity,
  so the stall window is short.
- `Close` cancels the watch context, closes every client and the etcd
  client, and aggregates errors.

### Client (gRPC Peer)

```go
func NewClient(addr, svcName string, etcdCli *client_v3.Client) (*Client, error)
func (c *Client) Get(group, key string) ([]byte, error)
func (c *Client) Set(ctx context.Context, group, key string, value []byte) error
func (c *Client) Delete(group, key string) (bool, error)
func (c *Client) Close() error
```

Behavior notes:

- `Get` and `Delete` build their own
  `context.WithTimeout(context.Background(), 3*time.Second)` and discard any
  caller context. Only `Set` honors the caller `ctx`; `Group.syncToPeers`
  passes a 3-second-timeout context, so writes are bounded too.
- The transport is `grpc.NewClient` with `insecure.NewCredentials()` and
  `WaitForReady(true)`. There is no TLS, no retry, no circuit breaker.
- The `etcdCli` parameter is stored but never used and never closed; passing
  `nil` is safe. `NewClient` no longer creates its own etcd client.
- `Close` closes only the gRPC connection.

### Server (gRPC)

```go
type ServerOptions struct {
    EtcdEndpoints []string
    DialTimeout   time.Duration
    MaxMsgSize    int          // sets MaxRecvMsgSize only; MaxSendMsgSize stays gRPC default
}

var DefaultServerOptions = &ServerOptions{
    EtcdEndpoints: []string{"localhost:2379"},
    DialTimeout:   5 * time.Second,
    MaxMsgSize:    4 << 20,
}

type ServerOption func(*ServerOptions)

func WithEtcdEndpoints(endpoints []string) ServerOption
func WithDialTimeout(timeout time.Duration) ServerOption

func NewServer(addr, svcName string, opts ...ServerOption) (*Server, error)
func (s *Server) Start() error    // blocks until grpcServer.Serve returns
func (s *Server) Stop()           // idempotent (sync.Once)
```

There is no TLS support: the former `WithTLS` option and
`ServerOptions.TLS/CertFile/KeyFile` fields were removed; transport is
plaintext.

The server registers `pb.SwiftyCacheServer` and a gRPC health server (initial
status `SERVING` under `svcName`). `Start` listens on `addr`, then registers
the service in etcd in a goroutine using the server's own
`EtcdEndpoints`/`DialTimeout` (via the internal `registerWithConfig`), so
`WithEtcdEndpoints` is honored for registration. `Stop` first flips health to
`NOT_SERVING`, then closes `stopCh` (triggering lease revocation), calls
`GracefulStop`, and closes the server's etcd client.

gRPC handlers (all inject the peer-request marker via `withPeerRequest`):

- `Get(ctx, *pb.Request) (*pb.ResponseForGet, error)` increments the group's
  `server_requests` counter, then calls `Group.Get` with the peer marker so a
  local miss is answered by the local Getter and never re-forwarded (no
  forwarding loops during ring divergence). Returns `view.ByteSlice()`. The
  group must be registered on the receiving node or the RPC errors.
- `Set(ctx, *pb.Request) (*pb.ResponseForGet, error)` marks the context and
  delegates to `Group.Set`, so the receiving peer does not re-broadcast.
  Echoes the value back.
- `Delete(ctx, *pb.Request) (*pb.ResponseForDelete, error)` marks the context
  and delegates to `Group.Delete`; the response `Value` is `err == nil`.

## Consistent hashing

```go
type ConHashConfig struct {
    DefaultReplicas int                      // default 50
    HashFunc        func(data []byte) uint32 // default crc32.ChecksumIEEE

    // Deprecated: the auto-rebalancer was removed to keep the key-to-node
    // mapping stable (groupcache semantics). These fields are ignored.
    MinReplicas          int
    MaxReplicas          int
    LoadBalanceThreshold float64
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

- The ring uses a fixed `DefaultReplicas` virtual nodes per physical node,
  keyed by `HashFunc([]byte(fmt.Sprintf("%s-%d", node, i)))`. There is no
  background rebalancer and no goroutine: the key-to-owner mapping is stable
  and `ConHashMap` needs no Close.
- `Add` is idempotent per node (re-adding an existing node is a no-op),
  skips empty node names, skips virtual-node hash collisions with existing
  entries (instead of stealing ownership), and re-sorts the ring. Returns an
  error only when zero nodes are provided.
- `Remove` returns an error for an empty or unknown node; it removes exactly
  the hashes recorded for that node (`nodeHashes`), so collisions skipped at
  add time cannot leave dangling ring entries.
- `Get` takes only a read lock; per-node request counters and the total are
  incremented atomically, so the hot path is not serialized. Returns `""`
  when the key is empty or the ring is empty.
- `GetStats` returns the cumulative traffic share per node as
  `map[node]fraction` (empty map when no requests yet); counters are never
  reset.

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

`Server.Start` performs registration itself (with the server's configured
endpoints); call `Register` directly only for custom setups. Behavior:

- Returns an error for an empty address. Creates its own etcd client
  (separate from `Server.etcdCli` and `ClientPicker.etcdCli`).
- The exported `Register` always uses `DefaultRegisterConfig`; only the
  server's internal path honors `WithEtcdEndpoints`.
- If `addr` begins with `:`, it is rewritten to
  `<firstNonLoopbackIPv4>:<port>` before being written to etcd.
  `NewClientPicker` applies the same rewrite to its own address, so passing
  the same `":port"` string to both sides stays consistent.
- Grants a 10-second lease and writes `/services/<svcName>/<addr>` (value =
  addr) under that lease; `KeepAlive` renews indefinitely.
- If the keepalive channel closes (etcd hiccup, partition), the registration
  goroutine re-registers with exponential backoff (1s doubling up to 30s)
  until it succeeds or `stopCh` closes, so nodes rejoin automatically after
  lease expiry.
- On `stopCh` close, revokes the lease with a 3-second timeout and closes the
  etcd client.

## Dashboard

The optional real-time dashboard streams cache state over WebSocket.

```go
func DashboardHandler() func(ctx *swifty_http.Context, next func())
func StartDashboard(addr string)
```

`StartDashboard` is guarded by a global `sync.Once`; only the first invocation
takes effect (a second group with a different `DashboardAddr` is silently
ignored). It creates a `swifty_http` application listening on `addr` with a
single route `GET /dashboard/ws`. `NewGroup` calls it automatically when
`CacheOptions.DashboardAddr` is non-empty.

`DashboardHandler` upgrades the connection to WebSocket (4 KiB buffers) and
serves the dashboard protocol; mount it on an existing `swifty_http` app to
share a port.

Protocol:

- On connect and every 2 seconds thereafter, the server pushes a snapshot
  JSON message (`{"type":"snapshot","groups":[...]}`) containing every group
  with `DashboardEnabled() == true`; each group entry includes its `Stats()`
  map and entry snapshots (`key`, `size`, `expire_at`, `level`).
- A 30-second WebSocket heartbeat (ping) is active.
- The client may send `{"action":"delete","group":"<name>","key":"<key>"}` to
  delete an entry from a dashboard-enabled group. Unknown actions and groups
  are logged and ignored. There is no authentication.

## Utilities

```go
func ValidPeerAddr(addr string) bool
```

Weak validator: accepts `localhost:<port>` or dotted-quad IPv4 with any
non-empty port string. No port-range validation, no IPv6. Not called anywhere
inside the package; treat it as opt-in.

## Protobuf surface (`pb/`)

```protobuf
message Request           { string group = 1; string key = 2; bytes value = 3; }
message ResponseForGet    { bytes value = 1; }
message ResponseForDelete { bool  value = 1; }

service SwiftyCache {
  rpc Get   (Request) returns (ResponseForGet);
  rpc Set   (Request) returns (ResponseForGet);
  rpc Delete(Request) returns (ResponseForDelete);
}
```

Generated symbols: `Request`, `ResponseForGet`, `ResponseForDelete`,
`SwiftyCacheClient`, `SwiftyCacheServer`, `UnimplementedSwiftyCacheServer`,
`NewSwiftyCacheClient`, `RegisterSwiftyCacheServer`, and the constants:

```go
const SwiftyCache_Get_FullMethodName    = "/pb.SwiftyCache/Get"
const SwiftyCache_Set_FullMethodName    = "/pb.SwiftyCache/Set"
const SwiftyCache_Delete_FullMethodName = "/pb.SwiftyCache/Delete"
```

Conventions when extending:

- All three RPCs share `Request`; the `value` field is only populated for
  `Set`. `Set` reuses `ResponseForGet`. New RPCs should introduce dedicated
  messages rather than overloading existing ones.
- The protocol carries no metadata fields (trace id, tenant, version); add
  them through gRPC metadata interceptors, not the message body.

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

1. `NewServer` and `srv.Start()` in a goroutine.
2. `NewClientPicker(addr)` with the same `addr` string handed to `NewServer`
   (both sides normalize `":port"` identically).
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

    // NewClientPicker reads DefaultRegisterConfig for etcd; adjust it when
    // etcd is not on localhost:2379.
    cache.DefaultRegisterConfig.Endpoints = []string{"127.0.0.1:2379"}

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
    opts.MaxBytes = 32 << 20 // WithCacheOptions replaces the cache, so set MaxBytes here
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

Unit tests in this repo do not require a running etcd or gRPC server. Reuse
the existing helpers:

- `group_test.go` defines `fakePeerPicker` (implements `PeerPicker` with
  configurable `peer`, `ok`, `isSelf`) and `fakePeer` (implements `Peer` with
  per-method function injection). Use them to exercise read fallback, self
  ownership, write propagation, and the peer-request-marker contract without
  touching the network.
- `client_test.go` defines `fakeSwiftyCacheClient` (implements
  `pb.SwiftyCacheClient`). Inject it via `&Client{grpcCli: ...}` to test
  `Client.Get` / `Set` / `Delete` happy and error paths.
- `regression_test.go` covers the refactor invariants: byte-budget eviction,
  L2 invalidation on Set, exactly-once OnEvicted for expired reads,
  singleflight panic recovery, duplicate-registration panic, peer-request
  non-forwarding, self-pick local load, in-callback cache double-check,
  DELETE-event key parsing, Cache close races, and stats counters. Preserve
  these when modifying the package.
- For end-to-end gRPC tests, use `google.golang.org/grpc/test/bufconn`; for
  etcd-backed flows, prefer an embedded etcd
  (`go.etcd.io/etcd/server/v3/embed`).

Always start tests with `DestroyAllGroups()`: `NewGroup` registers into a
process-global map and now panics on duplicate names, so leftover
registrations from earlier tests will crash later ones.

Run the suite from the module root:

```
cd swifty_cache && go test ./...
```

## Pitfalls / known limitations

1. `NewGroup` panics on a duplicate group name (groupcache semantics; the old
   replace-and-warn behavior is gone). Call `DestroyGroup`/`DestroyAllGroups`
   before re-creating a group, especially in tests.
2. `WithCacheOptions` replaces the entire `mainCache`, discarding the
   `cacheBytes` argument passed to `NewGroup`. Set `CacheOptions.MaxBytes`
   explicitly when you use this option, or the default 8 MiB applies.
3. TTLs below ~200 ms are unreliable: the internal clock (`Now()`) advances
   in ~100 ms increments and resyncs with the real clock about once per
   second, and `AddWithExpiration` mixes the real clock (duration
   computation) with the coarse clock (expiry checks).
4. Writes are best-effort: `Set`/`Delete` sync only to the single owning peer
   with one 3-second attempt, no retry, no acknowledgement. Non-owner
   replicas converge only via TTL, so always configure `WithExpiration` in a
   cluster.
5. `Client.Get` and `Client.Delete` ignore the caller's context and use their
   own 3-second background timeout; cancellation of the incoming request does
   not propagate to peer reads.
6. Transport is plaintext: the gRPC client hard-codes insecure credentials
   and the TLS server options were removed. Do not run across untrusted
   networks; use a mesh/sidecar if encryption is required.
7. The byte budget is split evenly per bucket (`MaxBytes / bucketCount`,
   minimum 1 byte). Skewed key distributions can evict from a hot bucket
   while cold buckets sit under budget, so the effective global capacity may
   be below `MaxBytes`.
8. Byte eviction is enforced only on the write path (after `Set`); an
   L1-to-L2 promotion on read never triggers byte eviction, so transient
   overshoot within a bucket is possible between writes.
9. `MaxMsgSize` sets only `MaxRecvMsgSize` on the server; the send side stays
   at the gRPC default (4 MiB), which also caps response sizes.
10. The exported `Register` and `NewClientPicker` always read the global
    `DefaultRegisterConfig` for etcd endpoints; only `Server.Start` honors
    `WithEtcdEndpoints`. When etcd is not on `localhost:2379`, mutate
    `DefaultRegisterConfig` before constructing the picker or the picker will
    watch the wrong cluster.
11. Three separate etcd clients exist per node (registration goroutine,
    `Server`, `ClientPicker`). `Client.NewClient`'s `etcdCli` parameter is
    dead weight: stored, never used, never closed; `nil` is fine.
12. `StartDashboard` is a process-global `sync.Once`: only the first address
    ever wins, and there is no error or log when later addresses are
    ignored. The dashboard WebSocket has no authentication and accepts
    `delete` commands; bind it to loopback or front it with auth middleware.
13. `getLocalIP` picks the first non-loopback IPv4 interface; on multi-NIC or
    container hosts the registered address may be unreachable by peers.
    Prefer passing a concrete `host:port` instead of `":port"`.
14. `lruStore` L1 slots freed by reads or deletes are only logically retired
    (`expireAt = 0`); the key stays in the internal hash map until the slot
    is recycled by an overflowing put, temporarily reducing effective L1
    capacity under read-heavy churn.
15. `Clear`, `Len`, and the cleanup sweep walk both LRU levels per bucket
    under the bucket lock (map-based dedup, O(N)); avoid calling `Clear`/`Len`
    in hot paths on large caches.
16. `ClientPicker.handleWatchEvents` and `fetchAllServices` create/close
    clients while holding the picker write lock; heavy topology churn briefly
    stalls `PickPeer`.
17. `SingleFlightGroup` has no `Forget`/`DoChan`; a slow leader blocks all
    followers for the full load duration (bounded by the 3s peer timeout plus
    your Getter's latency).
18. Values fetched from remote peers are cached locally without any hot-key
    probability gate (no groupcache hotCache), increasing memory replication
    across the cluster for read-heavy shared keys.

## File map

| File                   | Purpose                                                                                                          |
| ---------------------- | ----------------------------------------------------------------------------------------------------------------|
| `byte_view.go`         | Immutable `ByteView` value type and `cloneBytes`                                                                 |
| `store.go`             | `Value` and `Store` interfaces, `StoreOptions`, `NewStoreOptions`                                                |
| `lru.go`               | Bucketed two-level LRU (`lruStore`) with byte budget, `Entry`, `Now`, `HashBKRD`, `MaskOfNextPowOf2`, cleanup loop |
| `cache.go`             | `Cache` wrapper: lazy init, lock-guarded store access, stats (incl. bytes/evictions), idempotent close           |
| `group.go`             | `Group`: read-through with in-callback double-check, write propagation, peer-request marker, global registry     |
| `single_flight.go`     | `SingleFlightGroup` with panic recovery                                                                          |
| `con_hash.go`          | Consistent hash ring: fixed replicas, idempotent Add, collision-safe, atomic stats, no background goroutine      |
| `config.go`            | `ConHashConfig` and `DefaultConHashConfig` (rebalancer fields deprecated/ignored)                                |
| `peers.go`             | `PeerPicker`/`Peer` interfaces, `ClientPicker`: self-in-ring, key-based event parsing, resilient watch loop      |
| `server.go`            | gRPC server, health checking, graceful stop with traffic draining, peer-marker injection                         |
| `client.go`            | gRPC `Peer` implementation (insecure transport, 3s timeouts for Get/Delete)                                      |
| `register.go`          | `RegisterConfig`, `Register`, etcd lease + keepalive, re-registration with backoff, IP rewrite                   |
| `dashboard.go`         | WebSocket dashboard: `StartDashboard`, `DashboardHandler`, 2s snapshot push, delete command                      |
| `utils.go`             | `ValidPeerAddr` (opt-in helper, not used internally)                                                             |
| `regression_test.go`   | Regression tests pinning the refactor invariants (byte eviction, L2 invalidation, panic recovery, etc.)          |
| `group_test.go`        | Group/Cache/Server tests plus `fakePeerPicker` / `fakePeer` helpers                                              |
| `client_test.go`       | `Client` tests with `fakeSwiftyCacheClient`                                                                      |
| `lru_test.go`          | LRU list and store tests, `testValue` helper                                                                     |
| `con_hash_test.go`     | Hash ring add/get/remove/stats and concurrency tests                                                             |
| `single_flight_test.go`| Singleflight value/error propagation and dedup tests                                                             |
| `pb/swifty.proto`      | Protocol definition                                                                                              |
| `pb/swifty.pb.go`      | Generated protobuf message types                                                                                 |
| `pb/swifty_grpc.pb.go` | Generated gRPC client/server stubs and service descriptor                                                        |

## Dependencies

- `github.com/hangtiancheng/swifty.go/swifty_http` (local `replace` to
  `../swifty_http`) for the dashboard WebSocket server
- `go.etcd.io/etcd/client/v3` v3.6.x for service discovery and registration
- `google.golang.org/grpc` v1.73.x (including `health`) for transport and
  health checking
- `google.golang.org/protobuf` v1.36.x for message serialization
- Standard library beyond the above (`hash/crc32`, `math`, `sync`,
  `sync/atomic`, `context`, `time`, `log`, `net`, `sort`, `strings`,
  `errors`, `fmt`)

## Cross-references

- `swifty-http`: The dashboard depends on `swifty_http.Context`,
  `ctx.Upgrade`, `swifty_http.UpgradeOptions`, `swifty_http.WSConn`
  (`WriteJSON`, `ReadJSON`, `Heartbeat`, `Closed`), and
  `swifty_http.New()` / `app.Listen`. Load the swifty-http skill when working
  on dashboard code or mounting `DashboardHandler` on a shared application.
- `swifty-rpc`: For higher-level service patterns that compose swifty_cache
  groups with RPC service definitions.
- `swifty-orm`: For database-backed `Getter` implementations that load cache
  misses from MongoDB via the ORM layer.
