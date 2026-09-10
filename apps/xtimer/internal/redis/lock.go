package redis

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/utils"
)

const ftimerLockKeyPrefix = "FTIMER_LOCK_PREFIX_"

type DistributeLocker interface {
	Lock(context.Context, int64) error
	Unlock(context.Context) error
	ExpireLock(ctx context.Context, expireSeconds int64) error
}

// ReentrantDistributeLock is a reentrant distributed lock.
type ReentrantDistributeLock struct {
	key    string
	token  string
	client *Client
}

func NewReentrantDistributeLock(key string, client *Client) *ReentrantDistributeLock {
	return &ReentrantDistributeLock{
		key:    key,
		token:  utils.GetProcessAndGoroutineIDStr(),
		client: client,
	}
}

// Lock acquires the lock. Re-locking with the same token succeeds. expireSeconds
// is the lock expiry in seconds.
func (r *ReentrantDistributeLock) Lock(ctx context.Context, expireSeconds int64) error {
	lockKey := r.getLockKey()

	// First check whether the lock already belongs to us.
	res, err := r.client.Get(ctx, lockKey)
	if err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	if res == r.token {
		return nil
	}

	// The lock definitely does not belong to us; try to acquire it.
	acquired, err := r.client.SetNX(ctx, lockKey, r.token, expireSeconds)
	if err != nil {
		return err
	}
	if !acquired {
		return errors.New("lock is acquired by others")
	}
	return nil
}

// Unlock releases the lock. Atomicity is guaranteed by a Lua script.
func (r *ReentrantDistributeLock) Unlock(ctx context.Context) error {
	script := newScript(LuaCheckAndDeleteDistributionLock)
	reply, err := script.Run(ctx, r.client.client, []string{r.getLockKey()}, r.token).Result()
	if err != nil {
		return err
	}

	if ret, _ := reply.(int64); ret != 1 {
		return errors.New("can not unlock without ownership of lock")
	}
	return nil
}

// ExpireLock refreshes the lock expiry. Atomicity is guaranteed by a Lua script.
func (r *ReentrantDistributeLock) ExpireLock(ctx context.Context, expireSeconds int64) error {
	script := newScript(LuaCheckAndExpireDistributionLock)
	reply, err := script.Run(ctx, r.client.client, []string{r.getLockKey()}, r.token, expireSeconds).Result()
	if err != nil {
		return err
	}

	if ret, _ := reply.(int64); ret != 1 {
		return errors.New("can not expire lock without ownership of lock")
	}
	return nil
}

func (r *ReentrantDistributeLock) getLockKey() string {
	return ftimerLockKeyPrefix + r.key
}
