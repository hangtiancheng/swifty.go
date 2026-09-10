<p align="center">
  <b>consistent_cache: a cache read/write consistency service implemented in pure Go</b>
</p>

## Introduction

Built for learning and practice, this is a cache read/write consistency library implemented in 100% pure Go. Features include:

- Cache consistency guarantee
  - Write flow: set the write-cache disable mark -> delete the cache -> write the database -> re-enable the write-cache mark after a delay
  - Read flow: read the cache -> read the database -> write the cache only while the write-cache mark is enabled
- Cache avalanche prevention
  - Random jitter on the cache expiry time, preventing massive amounts of data from expiring at the same moment
- Cache penetration countermeasure
  - A `NullData` placeholder is cached for missing data, preventing repeated lookups of non-existent keys from hitting the database

## Technical Details

<a href="">Consistency cache theory and practice (link to be added)</a> <br/><br/>

## Project Layout

```
consistent_cache/
├── service.go            # Service: the public cache consistency service (write/read flows)
├── interface.go          # Public abstractions: Cache, DB, Object and Logger interfaces
├── option.go             # Public option functions (WithXxx)
├── internal/
│   ├── log/              # Default zap logger (writes to standard output only)
│   ├── runtime/          # Process/goroutine id helpers
│   ├── mysql/            # gorm-based DB module implementation
│   └── redis/            # go-redis based Cache module implementation (Lua scripts included)
└── example/              # Sample Object implementation and integration tests
```

## Dependencies

- [github.com/redis/go-redis/v9](https://github.com/redis/go-redis) v9.22.0 - Redis client, Lua scripts via `redis.NewScript(...).Run(...)`
- [gorm.io/gorm](https://github.com/go-gorm/gorm) + [gorm.io/driver/mysql](https://github.com/go-gorm/mysql) - MySQL ORM (go-sql-driver/mysql is pulled in transitively)
- [go.uber.org/zap](https://github.com/uber-go/zap) v1.28.0 - structured logging (standard output only)

## Usage Example

```go
func TestConsistentCache(t *testing.T) {
	service := consistent_cache.NewService(
		// Cache module.
		redis.NewRedisCache(&redis.Config{
			// Redis address.
			Address: redisAddress,
			// Redis password.
			Password: redisPassword,
		}),
		// Database module.
		mysqlDB,
		// Cache expiry time of 120s.
		consistent_cache.WithCacheExpireSeconds(120),
		// Random jitter on the cache expiry time to prevent a cache avalanche.
		consistent_cache.WithCacheExpireRandomMode(),
		// The write-cache disable mark is re-enabled after a 1s delay.
		consistent_cache.WithDisableExpireSeconds(1),
	)
	ctx := context.Background()
	exp := example.Example{
		Key_: "test",
		Data: "test",
	}
	// Write operation.
	if err := service.Put(ctx, &exp); err != nil {
		t.Fatal(err)
	}

	// Read operation.
	expReceiver := example.Example{
		Key_: "test",
	}
	if _, err := service.Get(ctx, &expReceiver); err != nil {
		t.Fatal(err)
	}

	// The data that was read, and whether the cache was used.
	t.Logf("read data: %s", expReceiver.Data)
}
```

With the following imports:

```go
import (
	"github.com/hangtiancheng/swifty.go/apps/consistent_cache"
	"github.com/hangtiancheng/swifty.go/apps/consistent_cache/example"
	"github.com/hangtiancheng/swifty.go/apps/consistent_cache/internal/mysql"
	"github.com/hangtiancheng/swifty.go/apps/consistent_cache/internal/redis"
)
```

Run the example SQL in [example/example.sql](example/example.sql) to create the sample table. The integration tests in [example/example_test.go](example/example_test.go) require a live MySQL and Redis; they are skipped automatically when either service is unreachable. Target addresses can be overridden with the `CONSISTENT_CACHE_TEST_REDIS_ADDR`, `CONSISTENT_CACHE_TEST_REDIS_PASSWORD` and `CONSISTENT_CACHE_TEST_MYSQL_DSN` environment variables.
