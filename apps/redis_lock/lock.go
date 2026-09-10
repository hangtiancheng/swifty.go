package redis_lock

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hangtiancheng/swifty.go/apps/redis_lock/internal/lua"
	"github.com/hangtiancheng/swifty.go/apps/redis_lock/internal/osutil"
)

const RedisLockKeyPrefix = "REDIS_LOCK_PREFIX_"

var ErrLockAcquiredByOthers = errors.New("lock is acquired by others")

// ErrNil is returned when Redis replies with a nil value, e.g. GET on a
// missing key.
var ErrNil = redis.Nil

// IsRetryableErr reports whether a lock error may be retried, i.e. the lock
// is currently held by someone else.
func IsRetryableErr(err error) bool {
	return errors.Is(err, ErrLockAcquiredByOthers)
}

// RedisLock is a Redis based distributed lock. It is not reentrant, but it
// guarantees symmetric ownership: only the holder can release or renew it.
type RedisLock struct {
	LockOptions
	key    string
	token  string
	client LockClient

	// Whether the watchdog is currently running.
	runningDog int32
	// stopDog stops the watchdog.
	stopDog context.CancelFunc
}

// NewRedisLock creates a distributed lock on key. client must satisfy the
// LockClient interface (*Client does). Without WithExpireSeconds the watchdog
// mode is enabled and the lock is renewed automatically until Unlock.
func NewRedisLock(key string, client LockClient, opts ...LockOption) *RedisLock {
	r := RedisLock{
		key:    key,
		token:  osutil.GetProcessAndGoroutineIDStr(),
		client: client,
	}

	for _, opt := range opts {
		opt(&r.LockOptions)
	}

	repairLock(&r.LockOptions)
	return &r
}

// Lock acquires the lock.
func (r *RedisLock) Lock(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			return
		}
		// On success the watchdog is started. The lock is not reentrant, so
		// the watchdog can never be started twice for the same lock.
		r.watchDog(ctx)
	}()

	// Regardless of the blocking mode, try to acquire the lock once first.
	err = r.tryLock(ctx)
	if err == nil {
		return nil
	}

	// In non-blocking mode a failed attempt is returned immediately.
	if !r.isBlock {
		return err
	}

	// Only retryable errors may be retried; everything else is returned.
	if !IsRetryableErr(err) {
		return err
	}

	// In blocking mode keep polling for the lock.
	err = r.blockingLock(ctx)
	return
}

func (r *RedisLock) tryLock(ctx context.Context) error {
	// First check whether the lock is already owned by this caller.
	reply, err := r.client.SetNEX(ctx, r.getLockKey(), r.token, r.expireSeconds)
	if err != nil {
		return err
	}
	if reply != 1 {
		return fmt.Errorf("reply: %d, err: %w", reply, ErrLockAcquiredByOthers)
	}

	return nil
}

// watchDog starts the watchdog that renews the lock in the background.
func (r *RedisLock) watchDog(ctx context.Context) {
	// 1. Watchdog mode disabled, nothing to do.
	if !r.watchDogMode {
		return
	}

	// 2. Make sure a previously started watchdog has been fully reclaimed.
	for !atomic.CompareAndSwapInt32(&r.runningDog, 0, 1) {
	}

	// 3. Start the watchdog.
	ctx, r.stopDog = context.WithCancel(ctx)
	go func() {
		defer func() {
			atomic.StoreInt32(&r.runningDog, 0)
		}()
		r.runWatchDog(ctx)
	}()
}

func (r *RedisLock) runWatchDog(ctx context.Context) {
	ticker := time.NewTicker(WatchDogWorkStepSeconds * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// The watchdog keeps renewing the lock while the user has not
		// unlocked it explicitly. The Lua script guarantees the lock is still
		// owned before extending it. To avoid the lock expiring early due to
		// network latency, each renewal adds an extra 5 s of headroom.
		_ = r.DelayExpire(ctx, WatchDogWorkStepSeconds+5)
	}
}

// DelayExpire extends the expiry of the lock. The Lua script keeps the
// check-and-extend operation atomic and verifies ownership first.
func (r *RedisLock) DelayExpire(ctx context.Context, expireSeconds int64) error {
	keysAndArgs := []interface{}{r.getLockKey(), r.token, expireSeconds}
	reply, err := r.client.Eval(ctx, lua.LuaCheckAndExpireDistributedLock, 1, keysAndArgs)
	if err != nil {
		return err
	}

	if ret, _ := reply.(int64); ret != 1 {
		return errors.New("can not expire lock without ownership of lock")
	}

	return nil
}

func (r *RedisLock) blockingLock(ctx context.Context) error {
	// Upper bound of the blocking wait time.
	timeoutCh := time.After(time.Duration(r.blockWaitingSeconds) * time.Second)
	// Polling ticker: try to acquire the lock every 50 ms.
	ticker := time.NewTicker(time.Duration(50) * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		select {
		// ctx is done.
		case <-ctx.Done():
			return fmt.Errorf("lock failed, ctx timeout, err: %w", ctx.Err())
		// Blocking wait reached its upper bound.
		case <-timeoutCh:
			return fmt.Errorf("block waiting time out, err: %w", ErrLockAcquiredByOthers)
		// Keep going.
		default:
		}

		// Try to acquire the lock.
		err := r.tryLock(ctx)
		if err == nil {
			// Acquired, return the result.
			return nil
		}

		// Non-retryable errors are returned immediately.
		if !IsRetryableErr(err) {
			return err
		}
	}

	// Unreachable: the ticker channel is never closed.
	return nil
}

// Unlock releases the lock. The Lua script keeps the check-and-delete
// operation atomic and verifies ownership first.
func (r *RedisLock) Unlock(ctx context.Context) error {
	defer func() {
		// Stop the watchdog.
		if r.stopDog != nil {
			r.stopDog()
		}
	}()

	keysAndArgs := []interface{}{r.getLockKey(), r.token}
	reply, err := r.client.Eval(ctx, lua.LuaCheckAndDeleteDistributedLock, 1, keysAndArgs)
	if err != nil {
		return err
	}

	if ret, _ := reply.(int64); ret != 1 {
		return errors.New("can not unlock without ownership of lock")
	}

	return nil
}

func (r *RedisLock) getLockKey() string {
	return RedisLockKeyPrefix + r.key
}
