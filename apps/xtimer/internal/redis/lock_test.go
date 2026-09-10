package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

const lockKey = "FTIMER_LOCK_PREFIX_" + "test-lock"

func newTestClient(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	inner := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = inner.Close()
	})
	return &Client{client: inner}, mr
}

// lockFromOtherGoroutine builds a locker whose token differs from the caller's
// token, since the token embeds the constructing goroutine's ID.
func lockFromOtherGoroutine(t *testing.T, c *Client, key string) DistributeLocker {
	t.Helper()
	done := make(chan DistributeLocker, 1)
	go func() {
		done <- c.GetDistributionLock(key)
	}()
	return <-done
}

func TestLockAndUnlock(t *testing.T) {
	c, mr := newTestClient(t)
	ctx := context.Background()

	locker := c.GetDistributionLock("test-lock")
	if err := locker.Lock(ctx, 60); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if !mr.Exists(lockKey) {
		t.Fatal("lock key not stored")
	}
	if err := locker.Unlock(ctx); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if mr.Exists(lockKey) {
		t.Fatal("lock key not removed after unlock")
	}
}

func TestLockIsReentrantForSameToken(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	locker := c.GetDistributionLock("test-lock")
	if err := locker.Lock(ctx, 60); err != nil {
		t.Fatalf("first Lock: %v", err)
	}
	// Re-locking from the same goroutine (same token) must succeed.
	if err := locker.Lock(ctx, 60); err != nil {
		t.Fatalf("reentrant Lock: %v", err)
	}
}

func TestLockContention(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()

	if err := c.GetDistributionLock("test-lock").Lock(ctx, 60); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	locker := lockFromOtherGoroutine(t, c, "test-lock")
	if err := locker.Lock(ctx, 60); err == nil {
		t.Fatal("expected Lock to fail while held by another token")
	}
}

func TestUnlockRequiresOwnership(t *testing.T) {
	c, mr := newTestClient(t)
	ctx := context.Background()

	if err := c.GetDistributionLock("test-lock").Lock(ctx, 60); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// A different token may neither unlock nor refresh the lock.
	other := lockFromOtherGoroutine(t, c, "test-lock")
	if err := other.Unlock(ctx); err == nil {
		t.Fatal("expected Unlock of a foreign lock to fail")
	}
	if err := other.ExpireLock(ctx, 120); err == nil {
		t.Fatal("expected ExpireLock of a foreign lock to fail")
	}
	if !mr.Exists(lockKey) {
		t.Fatal("foreign unlock removed the lock")
	}
}

func TestExpireLockRefreshesTTL(t *testing.T) {
	c, mr := newTestClient(t)
	ctx := context.Background()

	locker := c.GetDistributionLock("test-lock")
	if err := locker.Lock(ctx, 60); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := locker.ExpireLock(ctx, 120); err != nil {
		t.Fatalf("ExpireLock: %v", err)
	}
	if ttl := mr.TTL(lockKey); ttl <= 100*time.Second {
		t.Fatalf("TTL after refresh = %v, want about 120s", ttl)
	}
}

func TestTransactionRunsAllCommands(t *testing.T) {
	c, mr := newTestClient(t)
	ctx := context.Background()

	replies, err := c.Transaction(ctx,
		NewZAddCommand("zset", 42, "member"),
		NewExpireCommand("zset", 60),
	)
	if err != nil {
		t.Fatalf("Transaction: %v", err)
	}
	if len(replies) != 2 {
		t.Fatalf("got %d replies, want 2", len(replies))
	}
	if !mr.Exists("zset") {
		t.Fatal("zset not created")
	}
	if mr.TTL("zset") <= 0 {
		t.Fatal("expiry not applied")
	}
}
