package redis

import (
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	// DefaultIdleTimeoutSeconds is the default time after which idle
	// connections are closed.
	DefaultIdleTimeoutSeconds = 10
	// DefaultMaxActive is the default maximum number of connections
	// allocated by the pool.
	DefaultMaxActive = 100
	// DefaultMaxIdle is the default maximum number of idle connections.
	DefaultMaxIdle = 20
)

// ClientOptions holds the optional configuration of a Client.
type ClientOptions struct {
	maxIdle            int
	idleTimeoutSeconds int
	maxActive          int
	wait               bool
	// required parameters
	network  string
	address  string
	password string
}

// ClientOption configures optional Client settings.
type ClientOption func(c *ClientOptions)

// WithMaxIdle sets the maximum number of idle connections.
func WithMaxIdle(maxIdle int) ClientOption {
	return func(c *ClientOptions) {
		c.maxIdle = maxIdle
	}
}

// WithIdleTimeoutSeconds sets the time after which idle connections are
// closed, in seconds.
func WithIdleTimeoutSeconds(idleTimeoutSeconds int) ClientOption {
	return func(c *ClientOptions) {
		c.idleTimeoutSeconds = idleTimeoutSeconds
	}
}

// WithMaxActive sets the maximum number of connections allocated by the pool.
func WithMaxActive(maxActive int) ClientOption {
	return func(c *ClientOptions) {
		c.maxActive = maxActive
	}
}

// WithWaitMode makes the client block until a connection becomes available
// when the pool is exhausted, instead of failing fast.
func WithWaitMode() ClientOption {
	return func(c *ClientOptions) {
		c.wait = true
	}
}

// repairClient replaces invalid option values with defaults.
func repairClient(c *ClientOptions) {
	if c.maxIdle < 0 {
		c.maxIdle = DefaultMaxIdle
	}

	if c.idleTimeoutSeconds < 0 {
		c.idleTimeoutSeconds = DefaultIdleTimeoutSeconds
	}

	if c.maxActive < 0 {
		c.maxActive = DefaultMaxActive
	}
}

// toGoRedisOptions maps the client options onto go-redis connection options.
func (c *ClientOptions) toGoRedisOptions() *goredis.Options {
	return &goredis.Options{
		Network:         c.network,
		Addr:            c.address,
		Password:        c.password,
		PoolSize:        c.maxActive,
		MaxIdleConns:    c.maxIdle,
		MaxActiveConns:  c.maxActive,
		ConnMaxIdleTime: time.Duration(c.idleTimeoutSeconds) * time.Second,
		PoolTimeout:     c.poolTimeout(),
	}
}

// poolTimeout returns how long the client waits for a free connection when
// the pool is exhausted. Without wait mode the client fails fast instead,
// so the smallest meaningful wait is used.
func (c *ClientOptions) poolTimeout() time.Duration {
	if c.wait {
		return 0 // 0 lets go-redis apply its default pool timeout
	}
	return time.Millisecond
}
