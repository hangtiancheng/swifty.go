# redis_lock

A distributed lock library for Go built on top of
[github.com/redis/go-redis/v9](https://github.com/redis/go-redis/v9).

- Non-blocking and blocking (polling) lock acquisition
- Watchdog mode that automatically renews the lock while it is held
- RedLock support to mitigate Redis weak-consistency issues across nodes

## Installation

```shell
go get github.com/hangtiancheng/swifty.go/apps/redis_lock
```

## Watchdog mode

A lock created without `WithExpireSeconds` runs in watchdog mode: it gets the
default expiry (`DefaultLockExpireSeconds`, 30s) and a background goroutine
renews it every `WatchDogWorkStepSeconds` (10s) until `Unlock` is called:

```go
func repairLock(o *LockOptions) {
	if o.isBlock && o.blockWaitingSeconds <= 0 {
		// Default upper bound of the blocking wait time is 5 seconds.
		o.blockWaitingSeconds = 5
	}

	// When no expiry is configured for the lock, the watchdog takes over and
	// keeps renewing it until Unlock is called.
	if o.expireSeconds > 0 {
		return
	}

	o.expireSeconds = DefaultLockExpireSeconds
	o.watchDogMode = true
}
```

## Examples

### Non-blocking lock

```go
client := redis_lock.NewClient("tcp", "127.0.0.1:6379", "")

lock1 := redis_lock.NewRedisLock("test_key", client, redis_lock.WithExpireSeconds(1))
lock2 := redis_lock.NewRedisLock("test_key", client)

ctx := context.Background()
if err := lock1.Lock(ctx); err != nil {
	// handle error
}
defer lock1.Unlock(ctx)

// lock2 fails immediately because the lock is held by lock1.
if err := lock2.Lock(ctx); redis_lock.IsRetryableErr(err) {
	// the lock is acquired by others
}
```

### Blocking lock

```go
client := redis_lock.NewClient("tcp", "127.0.0.1:6379", "")

lock1 := redis_lock.NewRedisLock("test_key", client, redis_lock.WithExpireSeconds(1))
lock2 := redis_lock.NewRedisLock("test_key", client,
	redis_lock.WithBlock(), redis_lock.WithBlockWaitingSeconds(2))

ctx := context.Background()
if err := lock1.Lock(ctx); err != nil {
	// handle error
}
defer lock1.Unlock(ctx)

// lock2 keeps polling every 50 ms for up to 2 seconds.
if err := lock2.Lock(ctx); err != nil {
	// handle error
}
defer lock2.Unlock(ctx)
```

### Red lock

```go
confs := []*redis_lock.SingleNodeConf{
	{Network: "tcp", Address: "127.0.0.1:6379", Password: ""},
	{Network: "tcp", Address: "127.0.0.1:6380", Password: ""},
	{Network: "tcp", Address: "127.0.0.1:6381", Password: ""},
}

redLock, err := redis_lock.NewRedLock("test_key", confs,
	redis_lock.WithRedLockExpireDuration(10*time.Second),
	redis_lock.WithSingleNodesTimeout(100*time.Millisecond))
if err != nil {
	// handle error
}

ctx := context.Background()
if err := redLock.Lock(ctx); err != nil {
	// handle error
}
defer redLock.Unlock(ctx)
```

## Client options

Pool options follow redigo semantics and are mapped onto the go-redis
connection pool:

| Option                      | go-redis field                | Default                          |
| --------------------------- | ----------------------------- | -------------------------------- |
| `WithMaxIdle(n)`            | `MaxIdleConns`                | `DefaultMaxIdle` (20)            |
| `WithMaxActive(n)`          | `PoolSize` + `MaxActiveConns` | `DefaultMaxActive` (100)         |
| `WithIdleTimeoutSeconds(s)` | `ConnMaxIdleTime`             | `DefaultIdleTimeoutSeconds` (10) |
| `WithWaitMode()`            | `PoolTimeout`                 | fail fast when pool is exhausted |

With `WithWaitMode()` callers wait for a free connection (bounded by the
go-redis default pool timeout) instead of failing fast.

## Lock options

| Option                       | Effect                                                      |
| ---------------------------- | ----------------------------------------------------------- |
| `WithExpireSeconds(s)`       | Lock expiry; without it the watchdog mode is enabled        |
| `WithBlock()`                | Poll for the lock instead of failing immediately            |
| `WithBlockWaitingSeconds(s)` | Upper bound of the blocking wait (default 5s in block mode) |

Red lock options: `WithSingleNodesTimeout(d)` (per-node attempt budget,
default `DefaultSingleLockTimeout`, 50ms) and
`WithRedLockExpireDuration(d)` (lock expiry).

## Project layout

```
redis_lock/
├── client.go          # Client over go-redis/v9 (Get/Set/SetNEX/SetNX/Del/Incr/Eval/GetConn)
├── lock.go            # RedisLock with watchdog
├── redlock.go         # RedLock multi-node lock
├── option.go          # Client/Lock/RedLock options and defaults
├── lock_test.go       # integration tests (require a live Redis)
└── internal/
    ├── lua/lua.go     # atomic check-and-delete / check-and-expire scripts
    └── osutil/os.go   # process and goroutine id helpers
```

## Tests

The tests need a live Redis. They dial `REDIS_ADDR` (default
`127.0.0.1:6379`) with password `REDIS_PASSWORD` and skip with a clear reason
when the server is unreachable. `TestRedLock` runs the full acquire/release
flow when `REDIS_ADDR1`, `REDIS_ADDR2` and `REDIS_ADDR3` point to three
distinct nodes; otherwise it verifies against a single node that a failed
red lock attempt cannot succeed and releases the partially acquired lock.

```shell
go test ./...
```
