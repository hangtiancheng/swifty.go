package redis_lock

import "time"

const (
	// DefaultIdleTimeoutSeconds is the default time after which idle
	// connections are closed.
	DefaultIdleTimeoutSeconds = 10
	// DefaultMaxActive is the default upper bound of total connections in the
	// pool.
	DefaultMaxActive = 100
	// DefaultMaxIdle is the default number of idle connections kept around.
	DefaultMaxIdle = 20

	// DefaultLockExpireSeconds is the default expiry of a distributed lock.
	DefaultLockExpireSeconds = 30
	// WatchDogWorkStepSeconds is the interval between two watchdog renewal
	// rounds.
	WatchDogWorkStepSeconds = 10
)

type ClientOptions struct {
	maxIdle            int
	idleTimeoutSeconds int
	maxActive          int
	wait               bool
	// Required parameters.
	network  string
	address  string
	password string
}

type ClientOption func(c *ClientOptions)

// WithMaxIdle sets the number of idle connections kept in the pool. It maps
// onto go-redis MaxIdleConns.
func WithMaxIdle(maxIdle int) ClientOption {
	return func(c *ClientOptions) {
		c.maxIdle = maxIdle
	}
}

// WithIdleTimeoutSeconds sets the time after which idle connections are
// closed. It maps onto go-redis IdleTimeout.
func WithIdleTimeoutSeconds(idleTimeoutSeconds int) ClientOption {
	return func(c *ClientOptions) {
		c.idleTimeoutSeconds = idleTimeoutSeconds
	}
}

// WithMaxActive sets the upper bound of total connections in the pool. It
// maps onto go-redis PoolSize.
func WithMaxActive(maxActive int) ClientOption {
	return func(c *ClientOptions) {
		c.maxActive = maxActive
	}
}

// WithWaitMode makes callers block (bounded by the go-redis default pool
// timeout) when the pool is exhausted, instead of failing fast.
func WithWaitMode() ClientOption {
	return func(c *ClientOptions) {
		c.wait = true
	}
}

// repairClient fills unset or invalid pool options with the defaults.
func repairClient(c *ClientOptions) {
	if c.maxIdle <= 0 {
		c.maxIdle = DefaultMaxIdle
	}

	if c.idleTimeoutSeconds <= 0 {
		c.idleTimeoutSeconds = DefaultIdleTimeoutSeconds
	}

	if c.maxActive <= 0 {
		c.maxActive = DefaultMaxActive
	}
}

type LockOption func(*LockOptions)

type LockOptions struct {
	isBlock             bool
	blockWaitingSeconds int64
	expireSeconds       int64
	watchDogMode        bool
}

// WithBlock enables blocking mode: Lock keeps polling for the lock until it
// is acquired or the waiting time runs out.
func WithBlock() LockOption {
	return func(o *LockOptions) {
		o.isBlock = true
	}
}

// WithBlockWaitingSeconds sets the upper bound of the blocking wait time used
// in blocking mode.
func WithBlockWaitingSeconds(waitingSeconds int64) LockOption {
	return func(o *LockOptions) {
		o.blockWaitingSeconds = waitingSeconds
	}
}

// WithExpireSeconds sets the expiry of the distributed lock. When it is not
// set the watchdog mode is enabled instead.
func WithExpireSeconds(expireSeconds int64) LockOption {
	return func(o *LockOptions) {
		o.expireSeconds = expireSeconds
	}
}

// repairLock fills unset lock options with the defaults.
func repairLock(o *LockOptions) {
	if o.isBlock && o.blockWaitingSeconds <= 0 {
		// Default upper bound of the blocking wait time is 5 seconds.
		o.blockWaitingSeconds = 5
	}

	// When no expiry is configured for the lock, the watchdog takes over and
	// keeps renewing it until Unlock is called.
	if o.expireSeconds > 0 {
		return
	}

	o.expireSeconds = DefaultLockExpireSeconds
	o.watchDogMode = true
}

type RedLockOption func(*RedLockOptions)

type RedLockOptions struct {
	singleNodesTimeout time.Duration
	expireDuration     time.Duration
}

// WithSingleNodesTimeout sets the per-node time budget of a single RedLock
// lock attempt; a node that takes longer is not counted as a success.
func WithSingleNodesTimeout(singleNodesTimeout time.Duration) RedLockOption {
	return func(o *RedLockOptions) {
		o.singleNodesTimeout = singleNodesTimeout
	}
}

// WithRedLockExpireDuration sets the expiry of the red lock.
func WithRedLockExpireDuration(expireDuration time.Duration) RedLockOption {
	return func(o *RedLockOptions) {
		o.expireDuration = expireDuration
	}
}

type SingleNodeConf struct {
	Network  string
	Address  string
	Password string
	Opts     []ClientOption
}

// repairRedLock fills unset red lock options with the defaults.
func repairRedLock(o *RedLockOptions) {
	if o.singleNodesTimeout <= 0 {
		o.singleNodesTimeout = DefaultSingleLockTimeout
	}
}
