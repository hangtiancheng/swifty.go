package redis_lock

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"
)

// testRedisAddr returns the address of a reachable Redis instance used by the
// tests. It honors the REDIS_ADDR environment variable and falls back to
// 127.0.0.1:6379. The test is skipped when no Redis is reachable.
func testRedisAddr(t *testing.T) string {
	t.Helper()

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}

	conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if err != nil {
		t.Skipf("redis is unreachable at %s, skipping test: %v", addr, err)
	}
	if err := conn.Close(); err != nil {
		t.Skipf("redis at %s cannot be closed after dial, skipping test: %v", addr, err)
	}
	return addr
}

func newTestClient(t *testing.T) *Client {
	t.Helper()
	return NewClient("tcp", testRedisAddr(t), os.Getenv("REDIS_PASSWORD"))
}

func TestBlockingLock(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	key := "test_blocking_lock_key"
	defer client.Del(ctx, RedisLockKeyPrefix+key)

	holder := NewRedisLock(key, client, WithExpireSeconds(10))
	waiter := NewRedisLock(key, client, WithBlock(), WithBlockWaitingSeconds(3))

	if err := holder.Lock(ctx); err != nil {
		t.Fatalf("holder failed to acquire the lock: %v", err)
	}

	acquired := make(chan error, 1)
	go func() {
		acquired <- waiter.Lock(ctx)
	}()

	// Give the waiter some time to start polling; it must not acquire the
	// lock while the holder still owns it.
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

func TestNonBlockingLock(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	key := "test_nonblocking_lock_key"
	defer client.Del(ctx, RedisLockKeyPrefix+key)

	holder := NewRedisLock(key, client, WithExpireSeconds(10))
	contender := NewRedisLock(key, client)

	if err := holder.Lock(ctx); err != nil {
		t.Fatalf("holder failed to acquire the lock: %v", err)
	}

	// The contender must fail immediately with a retryable error.
	err := contender.Lock(ctx)
	if err == nil || !errors.Is(err, ErrLockAcquiredByOthers) {
		t.Fatalf("got err: %v, expect: %v", err, ErrLockAcquiredByOthers)
	}
	if !IsRetryableErr(err) {
		t.Fatalf("expected a retryable error, got: %v", err)
	}

	if err := holder.Unlock(ctx); err != nil {
		t.Fatalf("holder failed to release the lock: %v", err)
	}

	// After the release the contender can acquire the lock.
	if err := contender.Lock(ctx); err != nil {
		t.Fatalf("contender failed to acquire the released lock: %v", err)
	}
	if err := contender.Unlock(ctx); err != nil {
		t.Fatalf("contender failed to release the lock: %v", err)
	}
}

func TestRedLock(t *testing.T) {
	ctx := context.Background()
	key := "test_red_lock_key"
	lockKey := RedisLockKeyPrefix + key

	// Optional dedicated red lock nodes; with three distinct addresses the
	// full acquire/release flow of the red lock is exercised.
	addrs := []string{
		os.Getenv("REDIS_ADDR1"),
		os.Getenv("REDIS_ADDR2"),
		os.Getenv("REDIS_ADDR3"),
	}
	distinct := addrs[0] != "" && addrs[0] != addrs[1] && addrs[0] != addrs[2] && addrs[1] != addrs[2]

	if !distinct {
		// Fall back to a single Redis instance. All three node locks then
		// share one key on one server, so a majority can never be reached and
		// Lock must fail - while releasing the lock it acquired on the only
		// available node.
		addr := testRedisAddr(t)
		password := os.Getenv("REDIS_PASSWORD")
		addrs = []string{addr, addr, addr}
		client := NewClient("tcp", addr, password)
		defer client.Del(ctx, lockKey)

		confs := make([]*SingleNodeConf, 0, len(addrs))
		for _, addr := range addrs {
			confs = append(confs, &SingleNodeConf{Network: "tcp", Address: addr, Password: password})
		}

		redLock, err := NewRedLock(key, confs, WithRedLockExpireDuration(10*time.Second), WithSingleNodesTimeout(100*time.Millisecond))
		if err != nil {
			t.Fatalf("NewRedLock failed: %v", err)
		}

		if err := redLock.Lock(ctx); err == nil {
			t.Fatal("expected Lock to fail when a majority of nodes cannot be acquired, but it succeeded")
		}

		// The partially acquired lock must have been released again.
		if _, err := client.Get(ctx, lockKey); !errors.Is(err, ErrNil) {
			t.Fatalf("expected the lock key to be released after a failed Lock, got value error: %v", err)
		}
		return
	}

	confs := make([]*SingleNodeConf, 0, len(addrs))
	for _, addr := range addrs {
		confs = append(confs, &SingleNodeConf{Network: "tcp", Address: addr, Password: os.Getenv("REDIS_PASSWORD")})
	}

	redLock, err := NewRedLock(key, confs, WithRedLockExpireDuration(10*time.Second), WithSingleNodesTimeout(100*time.Millisecond))
	if err != nil {
		t.Fatalf("NewRedLock failed: %v", err)
	}

	if err := redLock.Lock(ctx); err != nil {
		t.Fatalf("red lock failed to acquire the lock: %v", err)
	}

	if err := redLock.Unlock(ctx); err != nil {
		t.Fatalf("red lock failed to release the lock: %v", err)
	}
}

func TestClientOperations(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	key := "test_client_operations_key"
	defer client.Del(ctx, key)

	if n, err := client.Set(ctx, key, "v1"); err != nil || n != 1 {
		t.Fatalf("Set returned (%d, %v), expect (1, nil)", n, err)
	}
	if v, err := client.Get(ctx, key); err != nil || v != "v1" {
		t.Fatalf("Get returned (%q, %v), expect (\"v1\", nil)", v, err)
	}

	// SetNEX must not overwrite an existing key and must report 0 without an
	// error in that case.
	if n, err := client.SetNEX(ctx, key, "v2", 10); err != nil || n != 0 {
		t.Fatalf("SetNEX on an existing key returned (%d, %v), expect (0, nil)", n, err)
	}
	if v, err := client.Get(ctx, key); err != nil || v != "v1" {
		t.Fatalf("Get returned (%q, %v), expect (\"v1\", nil)", v, err)
	}

	if n, err := client.SetNX(ctx, key, "v3"); err != nil || n != 0 {
		t.Fatalf("SetNX on an existing key returned (%d, %v), expect (0, nil)", n, err)
	}

	other := key + "_nex"
	defer client.Del(ctx, other)
	if n, err := client.SetNEX(ctx, other, "v2", 10); err != nil || n != 1 {
		t.Fatalf("SetNEX on a new key returned (%d, %v), expect (1, nil)", n, err)
	}

	if n, err := client.Incr(ctx, key+"_counter"); err != nil || n != 1 {
		t.Fatalf("Incr returned (%d, %v), expect (1, nil)", n, err)
	}
	defer client.Del(ctx, key+"_counter")

	if err := client.Del(ctx, key); err != nil {
		t.Fatalf("Del returned error: %v", err)
	}
	if _, err := client.Get(ctx, key); !errors.Is(err, ErrNil) {
		t.Fatalf("Get on a deleted key returned %v, expect %v", err, ErrNil)
	}
}
