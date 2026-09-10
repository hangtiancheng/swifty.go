# timewheel

<p align="center">
  <b>timewheel: a timing wheel framework implemented in pure Go</b>
</p>

## Features

- A standalone (in-process) timing wheel built on a Go time ticker plus a circular array of slots<br/><br/>
- A distributed timing wheel built on a Go time ticker plus redis zsets (with Lua scripts), which triggers HTTP callbacks<br/><br/>

## Installation

```
go get github.com/hangtiancheng/swifty.go/apps/timewheel
```

The only third-party dependency is [github.com/redis/go-redis/v9](https://github.com/redis/go-redis/v9).

## Project layout

```
.
├── time_wheel.go        # standalone time wheel (public API)
├── redis_time_wheel.go  # redis-backed distributed time wheel (public API)
├── time_wheel_lua.go    # Lua scripts used by the redis-backed wheel
├── time_wheel_test.go   # tests
└── internal/
    ├── http/            # minimal JSON HTTP client for task callbacks
    ├── redis/           # thin go-redis/v9 client wrapper with pool options
    └── timex/           # time helpers
```

## Usage

Runnable example code: see [./time_wheel_test.go](./time_wheel_test.go).

- Standalone time wheel

```go
func Test_timeWheel(t *testing.T) {
	timeWheel := NewTimeWheel(10, 100*time.Millisecond)
	defer timeWheel.Stop()

	executed := make(chan string, 8)
	timeWheel.AddTask("test1", func() { executed <- "test1" }, time.Now().Add(300*time.Millisecond))
	// Re-adding a key replaces the pending task.
	timeWheel.AddTask("test2", func() { executed <- "test2" }, time.Now().Add(500*time.Millisecond))
}
```

- Redis-backed distributed time wheel

```go
func Test_redis_timeWheel(t *testing.T) {
	rTimeWheel := NewRTimeWheel(
		redis.NewClient("tcp", "127.0.0.1:6379", ""),
		thttp.NewClient(),
	)
	defer rTimeWheel.Stop()

	ctx := context.Background()
	if err := rTimeWheel.AddTask(ctx, "test1", &RTaskElement{
		CallbackURL: "https://example.com/callback",
		Method:      "POST",
		Req:         map[string]string{"key": "test1"},
	}, time.Now().Add(time.Second)); err != nil {
		// handle error
	}

	// Flag a task as deleted so it is skipped when it becomes due.
	if err := rTimeWheel.RemoveTask(ctx, "test2", time.Now().Add(2*time.Second)); err != nil {
		// handle error
	}
}
```

Notes:

- The `internal/` packages can only be imported from within this module; the
  redis client and HTTP client passed to `NewRTimeWheel` are constructed with
  `internal/redis.NewClient(...)` and `internal/http.NewClient(...)`.
- Tests that need a live redis (`Test_redis_timeWheel`) dial
  `127.0.0.1:6379` first and skip themselves when no redis is reachable; the
  pure time-wheel logic tests always run.
