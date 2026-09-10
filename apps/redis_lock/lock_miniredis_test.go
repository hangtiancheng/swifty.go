package redis_lock

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/hangtiancheng/swifty.go/apps/redis_lock/internal/lua"
)

// newMiniRedis starts an in-memory Redis and returns it together with a
// client connected to it, so these tests run without an external Redis.
func newMiniRedis(t *testing.T) (*miniredis.Miniredis, *Client) {
	t.Helper()

	mr := miniredis.RunT(t)
	return mr, NewClient("tcp", mr.Addr(), "")
}

// renewalCounter counts the check-and-expire EVAL calls routed through it.
type renewalCounter struct {
	LockClient
	renewals atomic.Int64
}

func (c *renewalCounter) Eval(ctx context.Context, src string, keyCount int, keysAndArgs []any) (any, error) {
	if src == lua.LuaCheckAndExpireDistributedLock {
		c.renewals.Add(1)
	}
	return c.LockClient.Eval(ctx, src, keyCount, keysAndArgs)
}

func TestLockTokenIsUniquePerInstance(t *testing.T) {
	_, client := newMiniRedis(t)

	first := NewRedisLock("token_key", client)
	second := NewRedisLock("token_key", client)

	if first.token == "" {
		t.Fatal("lock token must not be empty")
	}
	if first.token == second.token {
		t.Fatalf("two lock instances share the same token %q", first.token)
	}
}

func TestUnlockCannotReleaseNewHoldersLock(t *testing.T) {
	mr, client := newMiniRedis(t)
	ctx := context.Background()
	key := "reacquired_key"
	lockKey := RedisLockKeyPrefix + key

	first := NewRedisLock(key, client, WithExpireSeconds(1))
	if err := first.Lock(ctx); err != nil {
		t.Fatalf("first holder failed to acquire the lock: %v", err)
	}

	// Let the first lock expire and let a second holder take over.
	mr.FastForward(2 * time.Second)
	second := NewRedisLock(key, client, WithExpireSeconds(10))
	if err := second.Lock(ctx); err != nil {
		t.Fatalf("second holder failed to acquire the expired lock: %v", err)
	}

	// The expired first holder must not be able to release the lock of the
	// new holder: the tokens differ.
	if err := first.Unlock(ctx); err == nil {
		t.Fatal("expected the unlock of the expired first holder to fail")
	}
	if _, err := client.Get(ctx, lockKey); err != nil {
		t.Fatalf("the lock of the second holder was released by the first holder: %v", err)
	}

	if err := second.Unlock(ctx); err != nil {
		t.Fatalf("second holder failed to release the lock: %v", err)
	}
}

func TestUnlockTwice(t *testing.T) {
	_, client := newMiniRedis(t)
	ctx := context.Background()

	lock := NewRedisLock("double_unlock_key", client, WithExpireSeconds(10))
	if err := lock.Lock(ctx); err != nil {
		t.Fatalf("failed to acquire the lock: %v", err)
	}
	if err := lock.Unlock(ctx); err != nil {
		t.Fatalf("failed to release the lock: %v", err)
	}
	if err := lock.Unlock(ctx); err == nil {
		t.Fatal("expected the second unlock to fail")
	}
}

func TestUnlockWithoutLock(t *testing.T) {
	_, client := newMiniRedis(t)
	ctx := context.Background()

	lock := NewRedisLock("never_locked_key", client, WithExpireSeconds(10))
	if err := lock.Unlock(ctx); err == nil {
		t.Fatal("expected the unlock of a lock that was never acquired to fail")
	}
}

func TestDelayExpireRequiresOwnership(t *testing.T) {
	mr, client := newMiniRedis(t)
	ctx := context.Background()
	key := "delay_expire_key"
	lockKey := RedisLockKeyPrefix + key

	lock := NewRedisLock(key, client, WithExpireSeconds(30))
	if err := lock.DelayExpire(ctx, 30); err == nil {
		t.Fatal("expected extending a lock that is not owned to fail")
	}

	if err := lock.Lock(ctx); err != nil {
		t.Fatalf("failed to acquire the lock: %v", err)
	}
	if err := lock.DelayExpire(ctx, 20); err != nil {
		t.Fatalf("failed to extend the owned lock: %v", err)
	}
	if ttl := mr.TTL(lockKey); ttl != 20*time.Second {
		t.Fatalf("TTL after the extension is %v, expect 20s", ttl)
	}

	if err := lock.Unlock(ctx); err != nil {
		t.Fatalf("failed to release the lock: %v", err)
	}
}

func TestWatchDogRenewsLock(t *testing.T) {
	mr, client := newMiniRedis(t)
	ctx := context.Background()
	key := "watchdog_renew_key"
	lockKey := RedisLockKeyPrefix + key

	lock := NewRedisLock(key, client) // watchdog mode
	// Shorten the renewal interval so the test does not need minutes.
	lock.dogInterval = 50 * time.Millisecond
	if err := lock.Lock(ctx); err != nil {
		t.Fatalf("failed to acquire the lock: %v", err)
	}

	// Each renewal sets a TTL of the interval plus the 5s headroom, here 5s.
	// Advance the Redis clock step by step: without renewals the lock would
	// expire within 30s, with renewals it survives the whole loop.
	for range 12 {
		mr.FastForward(3 * time.Second)
		time.Sleep(2 * lock.dogInterval) // let the watchdog renew
	}

	if _, err := client.Get(ctx, lockKey); err != nil {
		t.Fatalf("the watchdog failed to keep the lock alive: %v", err)
	}
	if ttl := mr.TTL(lockKey); ttl < 4*time.Second {
		t.Fatalf("the lock was not renewed recently, TTL: %v", ttl)
	}

	if err := lock.Unlock(ctx); err != nil {
		t.Fatalf("failed to release the lock: %v", err)
	}
}

func TestWatchDogStopsWhenLockIsLost(t *testing.T) {
	_, client := newMiniRedis(t)
	ctx := context.Background()
	key := "watchdog_lost_key"
	lockKey := RedisLockKeyPrefix + key

	counter := &renewalCounter{LockClient: client}
	lock := NewRedisLock(key, counter) // watchdog mode
	lock.dogInterval = 20 * time.Millisecond
	if err := lock.Lock(ctx); err != nil {
		t.Fatalf("failed to acquire the lock: %v", err)
	}

	// Wait until the watchdog renewed at least once.
	deadline := time.Now().Add(5 * time.Second)
	for counter.renewals.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the watchdog did not renew the lock")
		}
		time.Sleep(lock.dogInterval)
	}

	// The lock disappears, e.g. because it expired during a long pause.
	if err := client.Del(ctx, lockKey); err != nil {
		t.Fatalf("failed to delete the lock key: %v", err)
	}

	// The watchdog must stop renewing instead of extending a lock that now
	// belongs to someone else. One renewal may already be in flight.
	baseline := counter.renewals.Load()
	time.Sleep(20 * lock.dogInterval)
	if grew := counter.renewals.Load() - baseline; grew > 1 {
		t.Fatalf("the watchdog kept renewing the lost lock: %d additional renewals", grew)
	}

	// The unlock of the lost lock fails, but the stale watchdog is stopped.
	if err := lock.Unlock(ctx); err == nil {
		t.Fatal("expected the unlock of the lost lock to fail")
	}

	// A second holder can take over and keeps the lock.
	second := NewRedisLock(key, client, WithExpireSeconds(10))
	if err := second.Lock(ctx); err != nil {
		t.Fatalf("second holder failed to acquire the lost lock: %v", err)
	}
	time.Sleep(10 * lock.dogInterval)
	if _, err := client.Get(ctx, lockKey); err != nil {
		t.Fatalf("the lock of the second holder disappeared: %v", err)
	}
	if err := second.Unlock(ctx); err != nil {
		t.Fatalf("second holder failed to release the lock: %v", err)
	}
}

func TestLockReuseAfterUnlock(t *testing.T) {
	_, client := newMiniRedis(t)
	ctx := context.Background()

	lock := NewRedisLock("reuse_key", client) // watchdog mode
	if err := lock.Lock(ctx); err != nil {
		t.Fatalf("first acquisition failed: %v", err)
	}
	if err := lock.Unlock(ctx); err != nil {
		t.Fatalf("first release failed: %v", err)
	}

	// The watchdog of the first acquisition must not delay or break the
	// second one.
	start := time.Now()
	if err := lock.Lock(ctx); err != nil {
		t.Fatalf("second acquisition failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("re-acquiring the lock took %v, expect an immediate attempt", elapsed)
	}
	if err := lock.Unlock(ctx); err != nil {
		t.Fatalf("second release failed: %v", err)
	}
}

func TestBlockingLockOnMiniRedis(t *testing.T) {
	_, client := newMiniRedis(t)
	ctx := context.Background()

	holder := NewRedisLock("mini_blocking_key", client, WithExpireSeconds(10))
	waiter := NewRedisLock("mini_blocking_key", client, WithBlock(), WithBlockWaitingSeconds(3))

	if err := holder.Lock(ctx); err != nil {
		t.Fatalf("holder failed to acquire the lock: %v", err)
	}

	acquired := make(chan error, 1)
	go func() {
		acquired <- waiter.Lock(ctx)
	}()

	// While the holder still owns the lock, the waiter must keep blocking.
	time.Sleep(200 * time.Millisecond)
	select {
	case err := <-acquired:
		t.Fatalf("waiter acquired the lock before it was released, err: %v", err)
	default:
	}

	if err := holder.Unlock(ctx); err != nil {
		t.Fatalf("holder failed to release the lock: %v", err)
	}

	select {
	case err := <-acquired:
		if err != nil {
			t.Fatalf("waiter failed to acquire the lock: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("waiter timed out while blocking on the lock")
	}

	if err := waiter.Unlock(ctx); err != nil {
		t.Fatalf("waiter failed to release the lock: %v", err)
	}
}

func TestBlockingLockHonorsCanceledContext(t *testing.T) {
	_, client := newMiniRedis(t)
	ctx := context.Background()

	holder := NewRedisLock("canceled_ctx_key", client, WithExpireSeconds(10))
	if err := holder.Lock(ctx); err != nil {
		t.Fatalf("holder failed to acquire the lock: %v", err)
	}
	defer func() { _ = holder.Unlock(ctx) }() // best effort cleanup

	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	waiter := NewRedisLock("canceled_ctx_key", client, WithBlock(), WithBlockWaitingSeconds(3))
	err := waiter.Lock(canceled)
	if err == nil {
		t.Fatal("expected Lock to fail with a canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, expect it to wrap context.Canceled", err)
	}
}

func TestOnlyOneContenderAcquiresTheLock(t *testing.T) {
	_, client := newMiniRedis(t)
	ctx := context.Background()

	const contenders = 16
	var winners atomic.Int64

	var wg sync.WaitGroup
	for range contenders {
		wg.Go(func() {
			lock := NewRedisLock("contended_key", client, WithExpireSeconds(10))
			if err := lock.Lock(ctx); err == nil {
				winners.Add(1)
			} else if !IsRetryableErr(err) {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
	wg.Wait()

	if n := winners.Load(); n != 1 {
		t.Fatalf("got %d winners, expect exactly 1", n)
	}
}

func TestSetNEXRejectsNonPositiveExpiry(t *testing.T) {
	mr, client := newMiniRedis(t)
	ctx := context.Background()

	for _, expireSeconds := range []int64{0, -1} {
		if _, err := client.SetNEX(ctx, "setnex_key", "v", expireSeconds); err == nil {
			t.Fatalf("expected an error for expireSeconds %d", expireSeconds)
		}
	}
	if mr.Exists("setnex_key") {
		t.Fatal("the key must not be created")
	}
}

func TestNewRedLockRequiresExpiry(t *testing.T) {
	mr, _ := newMiniRedis(t)

	confs := make([]*SingleNodeConf, 0, 3)
	for range 3 {
		confs = append(confs, &SingleNodeConf{Network: "tcp", Address: mr.Addr()})
	}

	if _, err := NewRedLock("red_key", confs); err == nil {
		t.Fatal("expected an error when no expiry is configured")
	}
	if _, err := NewRedLock("red_key", confs, WithRedLockExpireDuration(500*time.Millisecond)); err == nil {
		t.Fatal("expected an error when the expiry is below one second")
	}
}
