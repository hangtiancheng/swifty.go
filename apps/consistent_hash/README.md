# consistent_hash

<p align="center">
  <b>consistent_hash: a consistent hashing module implemented in pure Go</b>
</p>

## How it works

`consistent_hash` distributes data keys over a set of nodes on a hash ring:

- Every node registers `weight * replicas` **virtual nodes** on the ring, which
  keeps the data distribution balanced.
- A data key belongs to the first virtual node found clockwise from the key's
  own position on the ring (`GetNode`).
- When a node joins or leaves, the ring computes exactly which data keys change
  owner and hands them to a user supplied **migrator** callback, so the caller
  can move the actual data (`AddNode` / `RemoveNode`).
- All ring mutations are serialized through a distributed lock, so several
  processes can share one ring safely.

The ring state can be stored in two interchangeable backends behind the
`HashRing` interface:

- `internal/local` — an in-memory skiplist ring, no external dependencies.
- `internal/redis` — a Redis sorted set ring. Every operation is an atomic Lua
  script executed through the `redis_lock` client, which also provides the
  distributed ring lock.

## Requirements

- Go 1.25 or newer.
- Optional: a reachable Redis instance (Redis 6.2+ for the `ZRANGE ... BYSCORE`
  syntax) when using the redis backed ring.
- The only non-stdlib dependency is the local sibling module
  `github.com/hangtiancheng/swifty.go/apps/redis_lock`, resolved through a `replace`
  directive to `../redis_lock`.

## Layout

```
consistent_hash/
├── consistent_hash.go   # public API: ConsistentHash (AddNode / RemoveNode / GetNode)
├── hash_ring.go         # HashRing interface implemented by both backends
├── option.go            # WithReplicas / WithLockExpireSeconds options
├── migration.go         # Migrator callback type and migration planning
├── encryptor.go         # Encryptor interface and the FNV-1a hasher
├── example_test.go      # runnable usage examples
└── internal/
    ├── local/           # in-memory skiplist hash ring
    ├── redis/           # Redis sorted set hash ring (Lua scripts)
    └── osutil/          # process / goroutine id helpers
```

## Usage

Build a redis client and a redis backed ring, then register a migrator and
start placing keys:

```go
package main

import (
	"context"

	"github.com/hangtiancheng/swifty.go/apps/consistent_hash"
	"github.com/hangtiancheng/swifty.go/apps/consistent_hash/internal/redis"
	"github.com/hangtiancheng/swifty.go/apps/redis_lock"
)

func main() {
	ctx := context.Background()

	redisClient := redis_lock.NewClient("tcp", "127.0.0.1:6379", "")
	hashRing := redis.NewRedisHashRing("my_ring", redisClient)

	consistentHash := consistent_hash.NewConsistentHash(
		hashRing,
		consistent_hash.NewFnvHasher(),
		// Move the actual data whenever ownership changes.
		func(ctx context.Context, dataKeys map[string]struct{}, from, to string) error {
			// ... copy dataKeys from node "from" to node "to" ...
			return nil
		},
		consistent_hash.WithReplicas(5),
		consistent_hash.WithLockExpireSeconds(15),
	)

	if err := consistentHash.AddNode(ctx, "node_a", 2); err != nil {
		panic(err)
	}

	node, err := consistentHash.GetNode(ctx, "data_key")
	if err != nil {
		panic(err)
	}
	_ = node // the owner of "data_key"
}
```

For a dependency-free setup, swap `internal/redis` for the in-memory ring from
`internal/local`.

More runnable examples live in [example_test.go](./example_test.go).

## Testing

```
go test ./...
```

Pure-logic tests (ring math, migration planning, skiplist behavior) always
run. The integration tests against Redis dial `localhost:6379` first and skip
with a clear reason when no Redis instance is reachable.
