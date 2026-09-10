// This file holds the end-to-end example of the gotcc example package. It
// needs a live MySQL and Redis instance; the tests are skipped with a clear
// reason when either service cannot be reached.
//
// Configuration via environment variables:
//   - GOTCC_TEST_MYSQL_DSN:      MySQL DSN, e.g.
//     "user:pass@tcp(127.0.0.1:3306)/gotcc?parseTime=true"
//   - GOTCC_TEST_REDIS_NETWORK:  Redis network, defaults to "tcp"
//   - GOTCC_TEST_REDIS_ADDR:     Redis address, defaults to "127.0.0.1:6379"
//   - GOTCC_TEST_REDIS_PASSWORD: Redis password, defaults to empty
package example_test

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/gotcc"
	"github.com/hangtiancheng/swifty.go/apps/gotcc/example"
	"github.com/hangtiancheng/swifty.go/apps/gotcc/example/dao"
	"github.com/hangtiancheng/swifty.go/apps/gotcc/example/pkg"
)

// failingComponent is a TCCComponent whose try phase always rejects, used
// to drive the rollback flow deterministically.
type failingComponent struct {
	id string
}

func (f *failingComponent) ID() string { return f.id }

func (f *failingComponent) Try(ctx context.Context, req *gotcc.TCCReq) (*gotcc.TCCResp, error) {
	return &gotcc.TCCResp{ComponentID: f.id, TXID: req.TXID}, nil
}

func (f *failingComponent) Confirm(ctx context.Context, txID string) (*gotcc.TCCResp, error) {
	return &gotcc.TCCResp{ComponentID: f.id, ACK: true, TXID: txID}, nil
}

func (f *failingComponent) Cancel(ctx context.Context, txID string) (*gotcc.TCCResp, error) {
	return &gotcc.TCCResp{ComponentID: f.id, ACK: true, TXID: txID}, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// setupExample connects to the live services, prepares the table and the
// components, and returns everything the tests need.
func setupExample(t *testing.T) (gotcc.TXStore, pkg.RedisClient, *gotcc.TXManager) {
	t.Helper()

	dsn := os.Getenv("GOTCC_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("GOTCC_TEST_MYSQL_DSN is not set; this test needs a live MySQL")
	}

	redisNetwork := envOr("GOTCC_TEST_REDIS_NETWORK", "tcp")
	redisAddr := envOr("GOTCC_TEST_REDIS_ADDR", "127.0.0.1:6379")
	redisPassword := os.Getenv("GOTCC_TEST_REDIS_PASSWORD")

	redisClient := pkg.NewRedisClient(redisNetwork, redisAddr, redisPassword)

	// Quick connectivity checks; skip when a service is unreachable.
	pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := redisClient.Ping(pingCtx); err != nil {
		t.Skipf("live Redis not reachable at %s: %v", redisAddr, err)
	}

	mysqlDB, err := pkg.NewDB(dsn)
	if err != nil {
		t.Skipf("live MySQL not reachable with GOTCC_TEST_MYSQL_DSN: %v", err)
	}
	sqlDB, err := mysqlDB.DB()
	if err != nil {
		t.Skipf("cannot obtain the MySQL handle: %v", err)
	}
	if err := sqlDB.PingContext(pingCtx); err != nil {
		t.Skipf("live MySQL not reachable with GOTCC_TEST_MYSQL_DSN: %v", err)
	}
	if err := mysqlDB.AutoMigrate(&dao.TXRecordPO{}); err != nil {
		t.Skipf("cannot ensure the tx_record table: %v", err)
	}

	txStore := example.NewMockTXStore(dao.NewTXRecordDAO(mysqlDB), redisClient)
	txManager := gotcc.NewTXManager(txStore, gotcc.WithMonitorTick(time.Second))
	t.Cleanup(txManager.Stop)

	return txStore, redisClient, txManager
}

func TestTCCExample(t *testing.T) {
	_, redisClient, txManager := setupExample(t)

	componentAID := "componentA"
	componentBID := "componentB"
	componentCID := "componentC"

	// Build and register the TCC components.
	for _, componentID := range []string{componentAID, componentBID, componentCID} {
		if err := txManager.Register(example.NewMockComponent(componentID, redisClient)); err != nil {
			t.Fatal(err)
		}
	}

	// Unique business ids per run so the test is repeatable.
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	txID, success, err := txManager.Transaction(ctx, []*gotcc.RequestEntity{
		{
			ComponentID: componentAID,
			Request: map[string]any{
				"biz_id": componentAID + "_biz_" + suffix,
			},
		},
		{
			ComponentID: componentBID,
			Request: map[string]any{
				"biz_id": componentBID + "_biz_" + suffix,
			},
		},
		{
			ComponentID: componentCID,
			Request: map[string]any{
				"biz_id": componentCID + "_biz_" + suffix,
			},
		},
	}...)
	if err != nil {
		t.Fatalf("tx failed, err: %v", err)
	}
	if !success {
		t.Fatal("tx failed")
	}

	// The business data of every component must be committed.
	for _, componentID := range []string{componentAID, componentBID, componentCID} {
		bizID := componentID + "_biz_" + suffix
		status, err := redisClient.Get(ctx, pkg.BuildDataKey(componentID, txID, bizID))
		if err != nil {
			t.Fatalf("get data status failed: %v", err)
		}
		if status != example.DataSuccessful.String() {
			t.Fatalf("data status of %s = %q, want %q", componentID, status, example.DataSuccessful.String())
		}
		if txStatus, err := redisClient.Get(ctx, pkg.BuildTXKey(componentID, txID)); err != nil || txStatus != example.TXConfirmed.String() {
			t.Fatalf("tx status of %s = %q (err: %v), want %q", componentID, txStatus, err, example.TXConfirmed.String())
		}
	}

	t.Log("tcc example transaction succeeded")
}

func TestTCCExampleRollback(t *testing.T) {
	txStore, redisClient, txManager := setupExample(t)

	componentAID := "componentA"
	componentBID := "componentB"
	componentCID := "componentC"
	failingID := "failingComponent"

	// Register the example components plus one component that always
	// rejects its try phase, forcing the rollback flow.
	components := map[string]gotcc.TCCComponent{
		componentAID: example.NewMockComponent(componentAID, redisClient),
		componentBID: example.NewMockComponent(componentBID, redisClient),
		componentCID: example.NewMockComponent(componentCID, redisClient),
		failingID:    &failingComponent{id: failingID},
	}
	for _, component := range components {
		if err := txManager.Register(component); err != nil {
			t.Fatal(err)
		}
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	txID, success, err := txManager.Transaction(ctx, []*gotcc.RequestEntity{
		{
			ComponentID: componentAID,
			Request: map[string]any{
				"biz_id": componentAID + "_biz_" + suffix,
			},
		},
		{
			ComponentID: componentBID,
			Request: map[string]any{
				"biz_id": componentBID + "_biz_" + suffix,
			},
		},
		{
			ComponentID: componentCID,
			Request: map[string]any{
				"biz_id": componentCID + "_biz_" + suffix,
			},
		},
		{
			ComponentID: failingID,
		},
	}...)
	if err != nil {
		t.Fatalf("tx failed, err: %v", err)
	}
	if success {
		t.Fatal("tx must fail when one component rejects its try phase")
	}

	tx, err := txStore.GetTX(ctx, txID)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status != gotcc.TXFailure {
		t.Fatalf("tx status = %s, want %s", tx.Status, gotcc.TXFailure)
	}

	// Every example component must have been canceled: the frozen data
	// record is released and the transaction is marked canceled.
	for _, componentID := range []string{componentAID, componentBID, componentCID} {
		bizID := componentID + "_biz_" + suffix
		if _, err := redisClient.Get(ctx, pkg.BuildDataKey(componentID, txID, bizID)); err == nil {
			t.Fatalf("frozen data of %s must be released after the rollback", componentID)
		}
		txStatus, err := redisClient.Get(ctx, pkg.BuildTXKey(componentID, txID))
		if err != nil || txStatus != example.TXCanceled.String() {
			t.Fatalf("tx status of %s = %q (err: %v), want %q", componentID, txStatus, err, example.TXCanceled.String())
		}
	}

	t.Log("tcc example rollback succeeded")
}
