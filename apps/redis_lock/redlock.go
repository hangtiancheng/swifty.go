package redis_lock

import (
	"context"
	"errors"
	"time"
)

// DefaultSingleLockTimeout is the default per-node timeout of a single red
// lock attempt.
const DefaultSingleLockTimeout = 50 * time.Millisecond

type RedLock struct {
	locks []*RedisLock
	RedLockOptions
}

// NewRedLock creates a red lock over the given nodes. A red lock needs at
// least 3 nodes to be meaningful.
func NewRedLock(key string, confs []*SingleNodeConf, opts ...RedLockOption) (*RedLock, error) {
	if len(confs) < 3 {
		return nil, errors.New("can not use redLock less than 3 nodes")
	}

	r := RedLock{}
	for _, opt := range opts {
		opt(&r.RedLockOptions)
	}

	repairRedLock(&r.RedLockOptions)
	if r.expireDuration > 0 && time.Duration(len(confs))*r.singleNodesTimeout*10 > r.expireDuration {
		// The accumulated per-node timeout budget must stay below one tenth
		// of the lock expiry.
		return nil, errors.New("expire thresholds of single node is too long")
	}

	r.locks = make([]*RedisLock, 0, len(confs))
	for _, conf := range confs {
		client := NewClient(conf.Network, conf.Address, conf.Password, conf.Opts...)
		r.locks = append(r.locks, NewRedisLock(key, client, WithExpireSeconds(int64(r.expireDuration.Seconds()))))
	}

	return &r, nil
}

// Lock acquires the red lock on a majority of the nodes. When a majority
// cannot be acquired, the locks already taken on individual nodes are
// released again so that no lock (or its watchdog) is left behind.
func (r *RedLock) Lock(ctx context.Context) error {
	var successCnt int
	for _, lock := range r.locks {
		startTime := time.Now()
		err := lock.Lock(ctx)
		cost := time.Since(startTime)
		if err == nil && cost <= r.singleNodesTimeout {
			successCnt++
		}
	}

	if successCnt < len(r.locks)>>1+1 {
		r.unlockQuietly(ctx)
		return errors.New("lock failed")
	}

	return nil
}

// Unlock releases the red lock on all nodes.
func (r *RedLock) Unlock(ctx context.Context) error {
	var err error
	for _, lock := range r.locks {
		if _err := lock.Unlock(ctx); _err != nil {
			if err == nil {
				err = _err
			}
		}
	}
	return err
}

// unlockQuietly releases all node locks on a failed Lock attempt. Nodes whose
// lock was never acquired report an ownership error, which is ignored here.
func (r *RedLock) unlockQuietly(ctx context.Context) {
	for _, lock := range r.locks {
		_ = lock.Unlock(ctx)
	}
}
