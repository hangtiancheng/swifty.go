package redis_lock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hangtiancheng/swifty.go/apps/redis_lock/internal/lua"
)

const RedisLockKeyPrefix = "REDIS_LOCK_PREFIX_"

var ErrLockAcquiredByOthers = errors.New("lock is acquired by others")

// errLockLost is returned by DelayExpire when the lock is no longer owned by
// the caller, e.g. because it expired or was already released.
var errLockLost = errors.New("can not expire lock without ownership of lock")

// watchDogHeadroom is the extra TTL added to every watchdog renewal so that
// the lock does not expire early due to network latency.
const watchDogHeadroom = 5 * time.Second

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
// Every lock instance carries its own random token, so two locks can never
// release or renew each other's lock.
type RedisLock struct {
	LockOptions
	key    string
	token  string
	client LockClient

	// dogInterval is the interval between two watchdog renewal rounds. It
	// defaults to WatchDogWorkStepSeconds; the field exists so tests can
	// shorten it.
	dogInterval time.Duration

	// dogMu guards the watchdog field below.
	dogMu sync.Mutex
	// dogCancel cancels the context of the watchdog started for the current
	// acquisition. It is nil until the first watchdog has been started.
	dogCancel context.CancelFunc
}

// NewRedisLock creates a distributed lock on key. client must satisfy the
// LockClient interface (*Client does). Without WithExpireSeconds the watchdog
// mode is enabled and the lock is renewed automatically until Unlock.
func NewRedisLock(key string, client LockClient, opts ...LockOption) *RedisLock {
	r := RedisLock{
		key:         key,
		token:       newLockToken(),
		client:      client,
		dogInterval: WatchDogWorkStepSeconds * time.Second,
	}

	for _, opt := range opts {
		opt(&r.LockOptions)
	}

	repairLock(&r.LockOptions)
	return &r
}

// newLockToken returns a random token identifying the lock owner. Each lock
// instance gets its own token so that ownership checks always refer to
// exactly one lock.
func newLockToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // crypto/rand.Read always succeeds.
	return hex.EncodeToString(b)
}

// Lock acquires the lock.
func (r *RedisLock) Lock(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			return
		}
		// On success the watchdog is started. The lock is not reentrant, so
		// while the lock is held a second Lock call fails with
		// ErrLockAcquiredByOthers before it can reach this point.
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
	// A single atomic SET ... EX ... NX: the lock is only created when the
	// key does not exist yet.
	reply, err := r.client.SetNEX(ctx, r.getLockKey(), r.token, r.expireSeconds)
	if err != nil {
		return err
	}
	if reply != 1 {
		return fmt.Errorf("reply: %d, err: %w", reply, ErrLockAcquiredByOthers)
	}

	return nil
}

// watchDog starts the watchdog that renews the lock in the background. A
// watchdog left over from a previous acquisition of this lock is stopped
// first, so at most one watchdog is renewing at a time. Besides Unlock, the
// watchdog stops by itself when ctx is canceled or the lock is lost.
func (r *RedisLock) watchDog(ctx context.Context) {
	// 1. Watchdog mode disabled, nothing to do.
	if !r.watchDogMode {
		return
	}

	r.dogMu.Lock()
	defer r.dogMu.Unlock()

	// 2. Stop a watchdog left over from a previous acquisition. Cancelling an
	// already cancelled context is a no-op, so this is always safe.
	if r.dogCancel != nil {
		r.dogCancel()
	}

	// 3. Start the watchdog.
	dogCtx, cancel := context.WithCancel(ctx)
	r.dogCancel = cancel
	go r.runWatchDog(dogCtx)
}

// stopWatchDog stops the watchdog of the current acquisition. It is safe to
// call it more than once and when no watchdog has ever been started.
func (r *RedisLock) stopWatchDog() {
	r.dogMu.Lock()
	defer r.dogMu.Unlock()

	if r.dogCancel != nil {
		r.dogCancel()
	}
}

func (r *RedisLock) runWatchDog(ctx context.Context) {
	ticker := time.NewTicker(r.dogInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		// The watchdog keeps renewing the lock while the user has not
		// unlocked it explicitly. The Lua script guarantees the lock is still
		// owned before extending it. To avoid the lock expiring early due to
		// network latency, each renewal adds an extra headroom on top of the
		// renewal interval.
		if err := r.DelayExpire(ctx, delayExpireSeconds(r.dogInterval)); err != nil {
			if errors.Is(err, errLockLost) {
				// The lock is gone: it expired or was released. Stop the
				// watchdog so that it never extends a lock that is owned by
				// someone else.
				return
			}
			// Transient errors (e.g. network failures) keep the watchdog
			// running; the next tick retries the renewal.
			continue
		}
	}
}

// delayExpireSeconds returns the TTL a watchdog renewal sets: the renewal
// interval plus the headroom.
func delayExpireSeconds(interval time.Duration) int64 {
	return int64((interval + watchDogHeadroom) / time.Second)
}

// DelayExpire extends the expiry of the lock. The Lua script keeps the
// check-and-extend operation atomic and verifies ownership first. It returns
// errLockLost when the lock is no longer owned by the caller.
func (r *RedisLock) DelayExpire(ctx context.Context, expireSeconds int64) error {
	keysAndArgs := []any{r.getLockKey(), r.token, expireSeconds}
	reply, err := r.client.Eval(ctx, lua.LuaCheckAndExpireDistributedLock, 1, keysAndArgs)
	if err != nil {
		return err
	}

	if ret, _ := reply.(int64); ret != 1 {
		return errLockLost
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
// operation atomic and verifies ownership first. Releasing a lock that is no
// longer owned (e.g. it expired) returns an error, and the watchdog is
// stopped in any case.
func (r *RedisLock) Unlock(ctx context.Context) error {
	// Stop the watchdog, no matter whether the lock could be released.
	defer r.stopWatchDog()

	keysAndArgs := []any{r.getLockKey(), r.token}
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
