# gotcc

<p align="center">
  <b>gotcc: a TCC (Try-Confirm-Cancel) distributed transaction SDK written in pure Go</b>
  <a title="Go Report Card" target="_blank" href="https://goreportcard.com/report/github.com/hangtiancheng/swifty.go/apps/gotcc"><img src="https://goreportcard.com/badge/github.com/hangtiancheng/swifty.go/apps/gotcc?style=flat-square" /></a>
  <a title="Codecov" target="_blank" href="https://codecov.io/gh/hangtiancheng/gotcc">
    <img src="https://img.shields.io/codecov/c/github/hangtiancheng/gotcc?style=flat-square&logo=codecov"/>
  </a>
</p>

## Introduction

"Theory first, practice follows." Before using this framework, it is worth
reviewing the theory behind TCC (Try-Confirm-Cancel) transactions so that
the implementation below makes sense.<br/><br/>

## Core capabilities

The `TXManager` transaction coordinator organizes and drives the
try-confirm/cancel two-phase commit flow across all registered components.<br/><br/>

## Integration guide

gotcc is a library; you plug in two building blocks and register them with
the coordinator.

### 1. Implement the transaction log store (`gotcc.TXStore`)

The transaction log tracks the progress of every distributed transaction.
Store it in a durable, shared backend (e.g. MySQL) and inject the
implementation into `gotcc.NewTXManager`:

```go
// TXStore persists the transaction log.
type TXStore interface {
	// CreateTX creates a new transaction record and returns the globally
	// unique transaction id.
	CreateTX(ctx context.Context, components ...TCCComponent) (txID string, err error)
	// TXUpdate records the try-phase response of a single component.
	TXUpdate(ctx context.Context, txID string, componentID string, accept bool) error
	// TXSubmit commits the final transaction outcome (success or failure).
	TXSubmit(ctx context.Context, txID string, success bool) error
	// GetHangingTXs returns every transaction that has not finished yet.
	GetHangingTXs(ctx context.Context) ([]*Transaction, error)
	// GetTX returns a single transaction by id.
	GetTX(ctx context.Context, txID string) (*Transaction, error)
	// Lock acquires the store-wide (distributed) lock for the monitor task.
	Lock(ctx context.Context, expireDuration time.Duration) error
	// Unlock releases the store-wide lock.
	Unlock(ctx context.Context) error
}
```

### 2. Implement your business components (`gotcc.TCCComponent`)

Each participant of the distributed transaction implements the TCC
component interface and is registered with the coordinator:

```go
// TCCComponent is a single participant of a TCC distributed transaction.
type TCCComponent interface {
	// ID returns the unique identifier of the component.
	ID() string
	// Try executes the first phase of the two-phase commit.
	Try(ctx context.Context, req *TCCReq) (*TCCResp, error)
	// Confirm executes the second-phase confirm operation.
	Confirm(ctx context.Context, txID string) (*TCCResp, error)
	// Cancel executes the second-phase cancel operation.
	Cancel(ctx context.Context, txID string) (*TCCResp, error)
}
```

## Example

A complete, runnable example (Redis backed TCC components + MySQL backed
transaction store) lives in the [`example`](./example) package:

- [`example/tcccomponent.go`](./example/tcccomponent.go) — a Redis backed `TCCComponent` whose try phase freezes the business data, confirm commits it and cancel releases it. It uses [`github.com/hangtiancheng/swifty.go/apps/redis_lock`](https://github.com/hangtiancheng/swifty.go/apps/redis_lock) for per-transaction serialization.
- [`example/txstore.go`](./example/txstore.go) — a MySQL (GORM) backed `TXStore` with a Redis distributed lock guarding the monitor task.
- [`example/example_test.go`](./example/example_test.go) — an end-to-end test wiring everything together.

```go
redisClient := pkg.NewRedisClient("tcp", "127.0.0.1:6379", "")
mysqlDB, err := pkg.NewDB(dsn)
if err != nil {
	t.Fatal(err)
}

componentAID := "componentA"
componentBID := "componentB"
componentCID := "componentC"

// Build the TCC components.
componentA := example.NewMockComponent(componentAID, redisClient)
componentB := example.NewMockComponent(componentBID, redisClient)
componentC := example.NewMockComponent(componentCID, redisClient)

// Build the transaction log store.
txRecordDAO := dao.NewTXRecordDAO(mysqlDB)
txStore := example.NewMockTXStore(txRecordDAO, redisClient)

txManager := gotcc.NewTXManager(txStore, gotcc.WithMonitorTick(time.Second))
defer txManager.Stop()

// Register the components.
for _, component := range []gotcc.TCCComponent{componentA, componentB, componentC} {
	if err := txManager.Register(component); err != nil {
		t.Fatal(err)
	}
}

ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
txID, success, err := txManager.Transaction(ctx, []*gotcc.RequestEntity{
	{ComponentID: componentAID, Request: map[string]interface{}{"biz_id": componentAID + "_biz"}},
	{ComponentID: componentBID, Request: map[string]interface{}{"biz_id": componentBID + "_biz"}},
	{ComponentID: componentCID, Request: map[string]interface{}{"biz_id": componentCID + "_biz"}},
}...)
if err != nil {
	t.Fatalf("tx failed, err: %v", err)
}
if !success {
	t.Fatal("tx failed")
}
```

## Project layout

```
gotcc/
├── component.go        # TCCComponent interface and request/response types
├── tccregister.go      # internal component registry
├── txmanager.go        # TXManager: the transaction coordinator
├── txstore.go          # TXStore interface
├── model.go            # transaction model and state machine
├── option.go           # functional options (WithTimeout, WithMonitorTick)
├── internal/
│   ├── log/            # zap based stdout logger
│   └── uuid/           # minimal RFC 4122 v4 UUID helper (crypto/rand)
└── example/            # runnable example: Redis components + MySQL store
    ├── pkg/            # example infra: go-redis/v9 client, GORM MySQL client, key builders
    ├── dao/            # GORM DAO for the transaction records
    ├── tcccomponent.go # example TCC component
    └── txstore.go      # example TXStore
```

## Dependencies

- [go.uber.org/zap](https://github.com/uber-go/zap) — logging (stdout only)
- [github.com/hangtiancheng/swifty.go/apps/redis_lock](https://github.com/hangtiancheng/swifty.go/apps/redis_lock) — Redis distributed locks (built on go-redis/v9)
- [github.com/redis/go-redis/v9](https://github.com/redis/go-redis) — Redis client of the example
- [gorm.io/gorm](https://gorm.io) + [gorm.io/driver/mysql](https://gorm.io) — MySQL access of the example

## Testing

```bash
go build ./...
go vet ./...
go test ./...
```

Unit tests are hermetic (in-memory fakes, no services required). The
integration tests that exercise live MySQL/Redis attempt a quick
connection and skip with a clear reason when the services are not
reachable. Configure them with:

- `GOTCC_TEST_MYSQL_DSN` — MySQL DSN for the live tests
- `GOTCC_TEST_REDIS_NETWORK` / `GOTCC_TEST_REDIS_ADDR` / `GOTCC_TEST_REDIS_PASSWORD` — Redis endpoint of the live tests
