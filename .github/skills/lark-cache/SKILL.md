---
name: lark-cache
description: >
  Distributed cache framework (lark_cache module), inspired by google/groupcache. Use
  this skill when working on cache groups, LRU-2 eviction, consistent hashing, gRPC cache
  servers or clients, etcd service discovery for cache peers, single-flight deduplication,
  or any code that imports github.com/hangtiancheng/lark-go/lark_cache. Also use it when
  the user asks about cache topology, peer synchronization, or ByteView semantics.
---

# lark_cache

A distributed, groupcache-inspired caching framework with gRPC transport, etcd-based
service discovery, consistent hashing with automatic rebalancing, single-flight
request coalescing, and a bucketed LRU-2 eviction store.

Module path: `github.com/hangtiancheng/lark-go/lark_cache`

Source root: `lark_cache/`

All types live in the `lark_cache` package (flat structure, no sub-packages except `pb/`).

## Architecture overview

```
Group (namespace)
  |-- mainCache (*Cache -> *lruStore)
  |-- loader   (*SingleFlightGroup)
  |-- peers    (PeerPicker -> *ClientPicker)
          |-- consHash (*ConHashMap)
          |-- clients  (map[addr]*Client)
          |-- etcdCli  (etcd service discovery)

Server (gRPC)
  |-- pb.LarkCacheServer (Get/Set/Delete RPCs)
  |-- Register()  (etcd lease + keepalive)
```

Lookup flow: `Group.Get` -> local cache hit? -> peer via consistent hash? ->
Getter callback -> populate local cache.

Set/Delete operations propagate to the owning peer asynchronously when the request
did not originate from a peer (checked via context key `peerRequestContextKey`).

## Core types

### Group

The top-level cache namespace. Each group has a name, a data-loading callback (`Getter`),
a local `Cache`, an optional `PeerPicker`, and a `SingleFlightGroup` for deduplication.

```go
type Getter interface {
    Get(ctx context.Context, key string) ([]byte, error)
}
type GetterFunc func(ctx context.Context, key string) ([]byte, error)
```

Group options:

```go
type GroupOption func(*Group)

func WithExpiration(d time.Duration) GroupOption
func WithPeers(peers PeerPicker) GroupOption
func WithCacheOptions(opts CacheOptions) GroupOption
```

Constructor and global registry:

```go
func NewGroup(name string, cacheBytes int64, getter Getter, opts ...GroupOption) *Group
func GetGroup(name string) *Group
func ListGroups() []string
func DestroyGroup(name string) bool
func DestroyAllGroups()
```

`NewGroup` panics if `getter` is nil. If a group with the same name exists, it is
replaced with a log warning.

Group methods:

| Method        | Signature                                  | Description                                        |
| ------------- | ------------------------------------------ | -------------------------------------------------- |
| Get           | `(ctx, key string) (ByteView, error)`      | Read-through: local cache, then peer, then Getter  |
| Set           | `(ctx, key string, value []byte) error`    | Write to local cache; async sync to owning peer    |
| Delete        | `(ctx, key string) error`                  | Remove from local cache; async sync to owning peer |
| Clear         | `()`                                       | Remove all entries from local cache                |
| Close         | `() error`                                 | Close the group and deregister it                  |
| RegisterPeers | `(PeerPicker)`                             | Attach peer topology (call once; panics on second) |
| Stats         | `() map[string]interface{}`                | Hit rate, load times, cache size                   |

Sentinel errors: `ErrKeyRequired`, `ErrValueRequired`, `ErrGroupClosed`.

### ByteView

Immutable wrapper for cached byte data. Implements `Value` (has `Len() int`).
The internal field `b []byte` is unexported; all access returns defensive copies.

```go
func (b ByteView) Len() int
func (b ByteView) ByteSlice() []byte   // defensive copy
func (b ByteView) String() string
```

### Cache

Wraps the underlying `lruStore` with lazy initialization, hit/miss stats, and atomic
close guard.

```go
type CacheOptions struct {
    MaxBytes     int64             // default: 8 MiB
    BucketCount  uint16            // sharding buckets (default: 16)
    CapPerBucket uint16            // per-bucket L1 capacity (default: 512)
    Level2Cap    uint16            // L2 capacity (default: 256)
    CleanupTime  time.Duration     // TTL cleanup interval (default: 1m)
    OnEvicted    func(key string, value Value)
}

func DefaultCacheOptions() CacheOptions
func NewCache(opts CacheOptions) *Cache
```

Cache methods:

| Method            | Signature                                   | Description                              |
| ----------------- | ------------------------------------------- | ---------------------------------------- |
| Add               | `(key string, value ByteView)`              | Store key-value pair                     |
| AddWithExpiration | `(key string, value ByteView, exp time.Time)` | Store with absolute expiration           |
| Get               | `(ctx, key string) (ByteView, bool)`        | Retrieve value; returns false on miss    |
| Delete            | `(key string) bool`                         | Remove key; returns false if not found   |
| Clear             | `()`                                        | Remove all entries, reset counters       |
| Len               | `() int`                                    | Number of stored entries                 |
| Close             | `()`                                        | Release resources (safe to call twice)   |
| Stats             | `() map[string]any`                         | hits, misses, hit_rate, size, closed     |

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

func NewStoreOptions() StoreOptions   // returns defaults
func NewStore(opts StoreOptions) *lruStore
```

The `lruStore` is a bucketed two-level LRU cache:
- Entries hash to one of `BucketCount` shards (rounded to power-of-2) via `HashBKRD`
- Each shard has two levels: L1 (entry point) and L2 (promoted on access)
- On `Get`, an L1 entry is removed from L1 and promoted to L2
- Each level is a fixed-capacity doubly-linked list with O(1) put/get/del
- A background goroutine runs every `CleanupInterval` to sweep expired entries
- Time is tracked via a coarse internal clock (`Now()`) that updates every ~100ms

Exported helpers:

```go
func HashBKRD(s string) int32
func MaskOfNextPowOf2(cap uint16) uint16
func Now() int64   // coarse nanosecond clock
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

### ClientPicker (etcd-backed PeerPicker)

Discovers cache nodes via etcd, maintains a consistent hash ring, and creates
gRPC `Client` instances for each peer address.

```go
type PickerOption func(*ClientPicker)
func WithServiceName(name string) PickerOption

func NewClientPicker(addr string, opts ...PickerOption) (*ClientPicker, error)
func (p *ClientPicker) PickPeer(key string) (Peer, bool, bool)
func (p *ClientPicker) PrintPeers()
func (p *ClientPicker) Close() error
```

`NewClientPicker` connects to etcd (using `DefaultRegisterConfig` endpoints), fetches
existing service registrations, and starts a background goroutine that watches for
put/delete events under `/services/<svcName>/`.

### Client (gRPC Peer)

Implements `Peer` using a gRPC connection to a remote `Server`.

```go
func NewClient(addr, svcName string, etcdCli *client_v3.Client) (*Client, error)
func (c *Client) Get(group, key string) ([]byte, error)
func (c *Client) Set(ctx context.Context, group, key string, value []byte) error
func (c *Client) Delete(group, key string) (bool, error)
func (c *Client) Close() error
```

Each RPC call uses a 3-second context timeout.

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

gRPC methods: `Get`, `Set`, `Delete` -- each looks up the `Group` by name from the
global registry.

## Consistent hashing

`ConHashMap` implements a virtual-node consistent hash ring with automatic
load-based rebalancing.

```go
type ConHashConfig struct {
    DefaultReplicas      int       // default: 50
    MinReplicas          int       // default: 10
    MaxReplicas          int       // default: 200
    HashFunc             func(data []byte) uint32   // default: crc32.ChecksumIEEE
    LoadBalanceThreshold float64   // default: 0.25
}

var DefaultConHashConfig *ConHashConfig

type ConHashOption func(*ConHashMap)
func WithConHashConfig(config *ConHashConfig) ConHashOption

func NewConHash(opts ...ConHashOption) *ConHashMap
func (m *ConHashMap) Add(nodes ...string) error
func (m *ConHashMap) Remove(node string) error
func (m *ConHashMap) Get(key string) string
func (m *ConHashMap) GetStats() map[string]float64
```

A background goroutine checks every second and rebalances replica counts when
the load deviation exceeds `LoadBalanceThreshold` and total requests exceed 1000.

## Single flight

`SingleFlightGroup` deduplicates concurrent loads for the same key.

```go
func (g *SingleFlightGroup) Do(key string, fn func() (interface{}, error)) (interface{}, error)
```

Uses `sync.Map.LoadOrStore` for lock-free fast path; duplicate callers block on
the first caller's `sync.WaitGroup`.

## Service registration

`Register` handles etcd service registration with lease-based health checks.

```go
type RegisterConfig struct {
    Endpoints   []string
    DialTimeout time.Duration
}

var DefaultRegisterConfig *RegisterConfig

func Register(svcName, addr string, stopCh <-chan error) error
```

Creates a 10-second lease, puts `/services/<svcName>/<addr>`, and renews via
`KeepAlive`. On `stopCh` close, revokes the lease.

## Utilities

```go
func ValidPeerAddr(addr string) bool
```

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

| File             | Purpose                                                      |
| ---------------- | ------------------------------------------------------------ |
| `group.go`       | Group type, Get/Set/Delete, load pipeline, global registry   |
| `cache.go`       | Cache wrapper with lazy init and stats                       |
| `byte_view.go`   | Immutable byte slice value type                              |
| `store.go`       | Store interface, Value interface, StoreOptions               |
| `lru.go`         | Bucketed two-level LRU implementation (lruStore)             |
| `con_hash.go`    | Consistent hash ring with virtual nodes and auto-rebalancing |
| `config.go`      | ConHashConfig and default values                             |
| `single_flight.go` | Request deduplication (SingleFlightGroup)                  |
| `peers.go`       | PeerPicker/Peer interfaces, ClientPicker with etcd discovery |
| `server.go`      | gRPC server with health check and TLS support                |
| `client.go`      | gRPC client implementing Peer                                |
| `register.go`    | etcd service registration with lease keepalive               |
| `utils.go`       | Address validation helper (ValidPeerAddr)                    |
| `pb/`            | Protobuf definitions and generated gRPC code                 |

## Dependencies

- `go.etcd.io/etcd/client/v3` -- service discovery and registration
- `google.golang.org/grpc` -- inter-node RPC transport
- `google.golang.org/protobuf` -- message serialization
