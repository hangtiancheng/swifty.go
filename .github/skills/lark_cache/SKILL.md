---
name: lark-cache
description: >
  Distributed cache framework (lark_cache module). Use this skill when working on
  cache groups, LRU/LRU-2 eviction, consistent hashing, gRPC cache servers or clients,
  etcd service discovery for cache peers, single-flight deduplication, or any code
  that imports github.com/hangtiancheng/lark_cache. Also use it when the user asks
  about cache topology, peer synchronization, or ByteView semantics.
---

# lark_cache

A distributed, groupcache-inspired caching framework with gRPC transport, etcd-based
service discovery, consistent hashing with automatic rebalancing, single-flight
request coalescing, and pluggable LRU / LRU-2 eviction stores.

Module path: `github.com/hangtiancheng/lark_cache`

Source root: `lark_cache/`

## Architecture overview

```
Group (namespace)
  |-- mainCache (Cache -> store.Store)
  |-- loader (single_flight.Group)
  |-- peers  (PeerPicker -> ClientPicker)
          |-- consHash (consistent_hash.Map)
          |-- clients  (map[addr]*Client)
          |-- etcdCli  (service discovery)

Server (gRPC)
  |-- pb.LarkCacheServer (Get/Set/Delete RPCs)
  |-- registry.Register  (etcd lease + keepalive)
```

Lookup flow: Group.Get -> local cache hit? -> peer via consistent hash? -> Getter callback -> populate local cache.

Set/Delete operations propagate to the owning peer asynchronously when the request did not originate from a peer.

## Core types

### Group

The top-level cache namespace. Each group has a name, a data-loading callback (`Getter`),
a local `Cache`, an optional `PeerPicker`, and a `single_flight.Group`.

```go
type Getter interface {
    Get(ctx context.Context, key string) ([]byte, error)
}
type GetterFunc func(ctx context.Context, key string) ([]byte, error)

type GroupOption func(*Group)

func WithExpiration(d time.Duration) GroupOption
func WithPeers(peers PeerPicker) GroupOption
func WithCacheOptions(opts CacheOptions) GroupOption

func NewGroup(name string, cacheBytes int64, getter Getter, opts ...GroupOption) *Group
func GetGroup(name string) *Group
func ListGroups() []string
func DestroyGroup(name string) bool
func DestroyAllGroups()
```

Group methods:

| Method        | Signature                         | Description                                        |
| ------------- | --------------------------------- | -------------------------------------------------- |
| Get           | `(ctx, key) -> (ByteView, error)` | Read-through: local cache, then peer, then Getter  |
| Set           | `(ctx, key, value) -> error`      | Write to local cache; async sync to owning peer    |
| Delete        | `(ctx, key) -> error`             | Remove from local cache; async sync to owning peer |
| Clear         | `()`                              | Remove all entries from local cache                |
| Close         | `() -> error`                     | Close the group and deregister it                  |
| RegisterPeers | `(PeerPicker)`                    | Attach peer topology (call once)                   |
| Stats         | `() -> map[string]interface{}`    | Hit rate, load times, cache size                   |

Sentinel errors: `ErrKeyRequired`, `ErrValueRequired`, `ErrGroupClosed`.

### ByteView

Immutable wrapper for cached byte data. Implements `store.Value` (has `Len() int`).

```go
func (b ByteView) Len() int
func (b ByteView) ByteSlice() []byte   // defensive copy
func (b ByteView) String() string
```

### Cache

Wraps `store.Store` with lazy initialization, hit/miss stats, and atomic close guard.

```go
type CacheOptions struct {
    CacheType    store.CacheType   // LRU or LRU2 (default: LRU2)
    MaxBytes     int64             // default: 8 MiB
    BucketCount  uint16            // sharding buckets (default: 16)
    CapPerBucket uint16            // per-bucket capacity (default: 512)
    Level2Cap    uint16            // LRU-2 second-level capacity (default: 256)
    CleanupTime  time.Duration     // TTL cleanup interval (default: 1m)
    OnEvicted    func(key string, value store.Value)
}

func DefaultCacheOptions() CacheOptions
func NewCache(opts CacheOptions) *Cache
```

Cache methods: `Add`, `Get`, `AddWithExpiration`, `Delete`, `Clear`, `Len`, `Close`, `Stats`.

### store.Store interface

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

type CacheType string
const (
    LRU  CacheType = "lru"
    LRU2 CacheType = "lru2"
)
```

`store.NewStore(cacheType, opts)` returns the selected implementation.

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

### ClientPicker (etcd-backed PeerPicker)

Discovers cache nodes via etcd, maintains a consistent hash ring, and creates
gRPC `Client` instances for each peer address.

```go
type PickerOption func(*ClientPicker)
func WithServiceName(name string) PickerOption

func NewClientPicker(addr string, opts ...PickerOption) (*ClientPicker, error)
```

`NewClientPicker` connects to etcd, fetches existing service registrations,
and starts a background goroutine that watches for put/delete events under
`/services/<svcName>/`.

### Client (gRPC Peer)

Implements `Peer` using a gRPC connection to a remote `Server`.

```go
func NewClient(addr, svcName string, etcdCli *client_v3.Client) (*Client, error)
```

Uses `grpc.NewClient` with insecure credentials and `WaitForReady(true)`.
Each RPC call has a 3-second context timeout.

### Server (gRPC)

Exposes cache groups over gRPC with health checking and etcd registration.

```go
type ServerOption func(*ServerOptions)

func WithEtcdEndpoints(endpoints []string) ServerOption
func WithDialTimeout(timeout time.Duration) ServerOption
func WithTLS(certFile, keyFile string) ServerOption

func NewServer(addr, svcName string, opts ...ServerOption) (*Server, error)
func (s *Server) Start() error
func (s *Server) Stop()
```

Default settings: etcd at `localhost:2379`, 5s dial timeout, 4 MiB max message size.

gRPC service methods: `Get`, `Set`, `Delete` -- each looks up the `Group` by name
from the global registry.

## Consistent hashing

Package `consistent_hash` implements a virtual-node consistent hash ring with
automatic load-based rebalancing.

```go
func New(opts ...Option) *Map
func WithConfig(config *Config) Option

func (m *Map) Add(nodes ...string) error
func (m *Map) Remove(node string) error
func (m *Map) Get(key string) string
func (m *Map) GetStats() map[string]float64
```

Config fields: `DefaultReplicas` (150), `MinReplicas` (50), `MaxReplicas` (500),
`LoadBalanceThreshold` (0.25), `HashFunc` (CRC32).

A background goroutine checks every second and rebalances replica counts when
the load deviation exceeds `LoadBalanceThreshold` and total requests exceed 1000.

## Single flight

Package `single_flight` deduplicates concurrent loads for the same key.

```go
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error)
```

Uses `sync.Map.LoadOrStore` for lock-free fast path; duplicate callers block on
the first caller's `sync.WaitGroup`.

## Service registry

Package `registry` handles etcd service registration with lease-based health checks.

```go
func Register(svcName, addr string, stopCh <-chan error) error
```

Creates a 10-second lease, puts `/services/<svcName>/<addr>`, and renews via
`KeepAlive`. On `stopCh` close, revokes the lease.

## Typical usage

```go
// Create a group with a data-loading callback
g := lark_cache.NewGroup("scores", 64<<20, lark_cache.GetterFunc(
    func(ctx context.Context, key string) ([]byte, error) {
        return db.GetScore(ctx, key)
    },
), lark_cache.WithExpiration(5*time.Minute))

// Read-through lookup
view, err := g.Get(ctx, "player:42")
fmt.Println(view.String())

// Explicit write
_ = g.Set(ctx, "player:42", []byte("9999"))

// Distributed setup
srv, _ := lark_cache.NewServer(":9001", "my_cache")
go srv.Start()

picker, _ := lark_cache.NewClientPicker(":9001")
g.RegisterPeers(picker)
```

## File map

| File               | Purpose                                                      |
| ------------------ | ------------------------------------------------------------ |
| `group.go`         | Group type, Get/Set/Delete, load pipeline, global registry   |
| `cache.go`         | Cache wrapper with lazy init and stats                       |
| `byte_view.go`     | Immutable byte slice value type                              |
| `peers.go`         | PeerPicker/Peer interfaces, ClientPicker with etcd discovery |
| `server.go`        | gRPC server with health check and TLS support                |
| `client.go`        | gRPC client implementing Peer                                |
| `utils.go`         | Address validation helper                                    |
| `store/store.go`   | Store interface, CacheType constants, factory                |
| `store/lru.go`     | Classic LRU implementation                                   |
| `store/lru2.go`    | LRU-2 (two-level frequency) implementation                   |
| `consistent_hash/` | Consistent hash ring with virtual nodes and auto-rebalancing |
| `single_flight/`   | Request deduplication                                        |
| `registry/`        | etcd service registration with lease keepalive               |
| `pb/`              | Protobuf definitions and generated gRPC code                 |

## Dependencies

- `go.etcd.io/etcd/client/v3` -- service discovery and registration
- `google.golang.org/grpc` -- inter-node RPC transport
- `google.golang.org/protobuf` -- message serialization
