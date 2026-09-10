package example_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/gotcc"
	"github.com/hangtiancheng/swifty.go/apps/gotcc/example"
)

func TestMockTXStoreLockAndUnlock(t *testing.T) {
	ctx := context.Background()
	mockTXStore := example.NewMockTXStore(newFakeTXRecordDAO(), newFakeRedis())

	if err := mockTXStore.Lock(ctx, time.Second); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}
	if err := mockTXStore.Unlock(ctx); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
}

func TestMockTXStoreLockErr(t *testing.T) {
	ctx := context.Background()
	fake := newFakeRedis()
	fake.setNEXErr = func(string) error { return errors.New("lock err") }
	mockTXStore := example.NewMockTXStore(newFakeTXRecordDAO(), fake)

	if err := mockTXStore.Lock(ctx, time.Second); err == nil {
		t.Fatal("Lock() must fail when the lock cannot be acquired")
	}
}

func TestMockTXStoreCreateTX(t *testing.T) {
	ctx := context.Background()
	daoFake := newFakeTXRecordDAO()
	mockTXStore := example.NewMockTXStore(daoFake, newFakeRedis())

	txID, err := mockTXStore.CreateTX(ctx, example.NewMockComponent("component", nil))
	if err != nil {
		t.Fatalf("CreateTX() error = %v", err)
	}
	if txID != "1" {
		t.Fatalf("CreateTX() txID = %q, want %q", txID, "1")
	}

	tx, err := mockTXStore.GetTX(ctx, txID)
	if err != nil {
		t.Fatalf("GetTX() error = %v", err)
	}
	if tx.Status != gotcc.TXHanging {
		t.Fatalf("tx status = %s, want %s", tx.Status, gotcc.TXHanging)
	}
	if len(tx.Components) != 1 || tx.Components[0].ComponentID != "component" {
		t.Fatalf("unexpected components: %+v", tx.Components)
	}
	if tx.Components[0].TryStatus != gotcc.TryHanging {
		t.Fatalf("component try status = %s, want %s", tx.Components[0].TryStatus, gotcc.TryHanging)
	}
}

func TestMockTXStoreTXUpdate(t *testing.T) {
	ctx := context.Background()
	mockTXStore := example.NewMockTXStore(newFakeTXRecordDAO(), newFakeRedis())

	if _, err := mockTXStore.CreateTX(ctx, example.NewMockComponent("component", nil)); err != nil {
		t.Fatalf("CreateTX() error = %v", err)
	}

	if err := mockTXStore.TXUpdate(ctx, "1", "component", true); err != nil {
		t.Fatalf("TXUpdate() error = %v", err)
	}

	// Re-applying the same status is an idempotent no-op.
	if err := mockTXStore.TXUpdate(ctx, "1", "component", true); err != nil {
		t.Fatalf("TXUpdate() idempotent re-apply error = %v", err)
	}

	// A different target status must fail: the component is no longer
	// hanging.
	if err := mockTXStore.TXUpdate(ctx, "1", "component", false); err == nil {
		t.Fatal("TXUpdate() must fail for a component that is not hanging anymore")
	}

	// A non-numeric transaction id must be rejected.
	if err := mockTXStore.TXUpdate(ctx, "not-a-number", "component", true); err == nil {
		t.Fatal("TXUpdate() must fail for a non-numeric txid")
	}
}

func TestMockTXStoreGetHangingTXs(t *testing.T) {
	ctx := context.Background()
	mockTXStore := example.NewMockTXStore(newFakeTXRecordDAO(), newFakeRedis())

	if _, err := mockTXStore.CreateTX(ctx, example.NewMockComponent("component", nil)); err != nil {
		t.Fatalf("CreateTX() error = %v", err)
	}

	txs, err := mockTXStore.GetHangingTXs(ctx)
	if err != nil {
		t.Fatalf("GetHangingTXs() error = %v", err)
	}
	if len(txs) != 1 {
		t.Fatalf("GetHangingTXs() returned %d transactions, want 1", len(txs))
	}
	if txs[0].Status != gotcc.TXHanging {
		t.Fatalf("tx status = %s, want %s", txs[0].Status, gotcc.TXHanging)
	}
	if txs[0].Components[0].TryStatus != gotcc.TryHanging {
		t.Fatalf("component try status = %s, want %s", txs[0].Components[0].TryStatus, gotcc.TryHanging)
	}
}

func TestMockTXStoreTXSubmit(t *testing.T) {
	ctx := context.Background()

	t.Run("successful", func(t *testing.T) {
		mockTXStore := example.NewMockTXStore(newFakeTXRecordDAO(), newFakeRedis())
		if _, err := mockTXStore.CreateTX(ctx, example.NewMockComponent("component", nil)); err != nil {
			t.Fatalf("CreateTX() error = %v", err)
		}

		if err := mockTXStore.TXSubmit(ctx, "1", true); err != nil {
			t.Fatalf("TXSubmit() error = %v", err)
		}
		tx, err := mockTXStore.GetTX(ctx, "1")
		if err != nil {
			t.Fatalf("GetTX() error = %v", err)
		}
		if tx.Status != gotcc.TXSuccessful {
			t.Fatalf("tx status = %s, want %s", tx.Status, gotcc.TXSuccessful)
		}
		// Submitting the successful transaction as failed must be rejected.
		if err := mockTXStore.TXSubmit(ctx, "1", false); err == nil {
			t.Fatal("TXSubmit() must reject a successful -> failure transition")
		}
	})

	t.Run("failure", func(t *testing.T) {
		mockTXStore := example.NewMockTXStore(newFakeTXRecordDAO(), newFakeRedis())
		if _, err := mockTXStore.CreateTX(ctx, example.NewMockComponent("component", nil)); err != nil {
			t.Fatalf("CreateTX() error = %v", err)
		}

		if err := mockTXStore.TXSubmit(ctx, "1", false); err != nil {
			t.Fatalf("TXSubmit() error = %v", err)
		}
		tx, err := mockTXStore.GetTX(ctx, "1")
		if err != nil {
			t.Fatalf("GetTX() error = %v", err)
		}
		if tx.Status != gotcc.TXFailure {
			t.Fatalf("tx status = %s, want %s", tx.Status, gotcc.TXFailure)
		}
		// Submitting the failed transaction as successful must be rejected.
		if err := mockTXStore.TXSubmit(ctx, "1", true); err == nil {
			t.Fatal("TXSubmit() must reject a failure -> successful transition")
		}
	})

	t.Run("invalidTxid", func(t *testing.T) {
		mockTXStore := example.NewMockTXStore(newFakeTXRecordDAO(), newFakeRedis())
		if err := mockTXStore.TXSubmit(ctx, "not-a-number", true); err == nil {
			t.Fatal("TXSubmit() must fail for a non-numeric txid")
		}
	})
}

func TestMockTXStoreGetTX(t *testing.T) {
	ctx := context.Background()
	mockTXStore := example.NewMockTXStore(newFakeTXRecordDAO(), newFakeRedis())

	// Without any record, GetTX must fail.
	if _, err := mockTXStore.GetTX(ctx, "1"); err == nil {
		t.Fatal("GetTX() must fail when no record exists")
	}

	if _, err := mockTXStore.CreateTX(ctx, example.NewMockComponent("component", nil)); err != nil {
		t.Fatalf("CreateTX() error = %v", err)
	}

	tx, err := mockTXStore.GetTX(ctx, "1")
	if err != nil {
		t.Fatalf("GetTX() error = %v", err)
	}
	if tx.TXID != "1" {
		t.Fatalf("tx id = %q, want %q", tx.TXID, "1")
	}
	// A non-numeric transaction id must be rejected.
	if _, err := mockTXStore.GetTX(ctx, "not-a-number"); err == nil {
		t.Fatal("GetTX() must fail for a non-numeric txid")
	}
}
