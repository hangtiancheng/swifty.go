package redis_lock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// LockClient is the minimal Redis capability required by RedisLock. *Client
// satisfies it; a custom implementation may be provided as well.
type LockClient interface {
	// SetNEX sets key to value with an expiry of expireSeconds, but only when
	// the key does not exist yet (SET ... EX ... NX). It returns 1 when the key
	// was set and 0 when it already existed.
	SetNEX(ctx context.Context, key, value string, expireSeconds int64) (int64, error)
	// Eval runs the Lua script src, declaring the first keyCount entries of
	// keysAndArgs as KEYS and the remaining entries as ARGV.
	Eval(ctx context.Context, src string, keyCount int, keysAndArgs []any) (any, error)
}

// Client is a Redis client built on top of github.com/redis/go-redis/v9.
// It manages its own connection pool internally.
type Client struct {
	ClientOptions
	client *redis.Client
}

// NewClient creates a Redis client. network is "tcp" (or "unix"), address is
// "host:port" and password may be empty when the server requires no auth.
func NewClient(network, address, password string, opts ...ClientOption) *Client {
	c := Client{
		ClientOptions: ClientOptions{
			network:  network,
			address:  address,
			password: password,
		},
	}

	for _, opt := range opts {
		opt(&c.ClientOptions)
	}

	repairClient(&c.ClientOptions)

	c.client = c.newGoRedisClient()
	return &c
}

// newGoRedisClient maps the redigo-style pool options onto go-redis:
//
//	maxActive   -> PoolSize + MaxActiveConns (base pool size and hard upper
//	                bound of total connections, i.e. redigo MaxActive)
//	maxIdle     -> MaxIdleConns (maximum number of idle connections kept
//	                around, i.e. redigo MaxIdle)
//	wait        -> PoolTimeout   (see poolTimeout)
//	idleTimeout -> ConnMaxIdleTime
func (c *Client) newGoRedisClient() *redis.Client {
	if c.address == "" {
		panic("cannot get redis address from config")
	}

	return redis.NewClient(&redis.Options{
		Network:         c.network,
		Addr:            c.address,
		Password:        c.password,
		PoolSize:        c.maxActive,
		MaxActiveConns:  c.maxActive,
		MaxIdleConns:    c.maxIdle,
		ConnMaxIdleTime: time.Duration(c.idleTimeoutSeconds) * time.Second,
		PoolTimeout:     poolTimeout(c.wait),
	})
}

// poolTimeout maps redigo's Wait option onto go-redis PoolTimeout:
//   - wait == true: block until a connection becomes available. go-redis
//     cannot block indefinitely, so PoolTimeout is left at 0 and go-redis
//     falls back to its default (ReadTimeout + 1s) bounded wait.
//   - wait == false: fail fast when the pool is exhausted, approximating
//     redigo's immediate "connection pool exhausted" error with a minimal
//     PoolTimeout.
func poolTimeout(wait bool) time.Duration {
	if wait {
		return 0
	}
	return time.Millisecond
}

// GetConn returns the underlying go-redis client. go-redis manages its
// connection pool internally, so there is no per-operation connection to hand
// out; the returned client can be used for commands that are not covered by
// the Client helpers. The ctx parameter is kept for interface stability.
func (c *Client) GetConn(ctx context.Context) (*redis.Client, error) {
	if c.client == nil {
		return nil, errors.New("redis client is not initialized")
	}
	return c.client, nil
}

// Get returns the string value stored at key. When the key does not exist the
// error wraps ErrNil.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", errors.New("redis GET key can't be empty")
	}

	return c.client.Get(ctx, key).Result()
}

// Set stores key with an unlimited lifetime. It returns 1 on success and -1
// together with an error on failure.
func (c *Client) Set(ctx context.Context, key, value string) (int64, error) {
	if key == "" || value == "" {
		return -1, errors.New("redis SET key or value can't be empty")
	}

	if err := c.client.Set(ctx, key, value, 0).Err(); err != nil {
		return -1, err
	}
	return 1, nil
}

// SetNEX stores key with an expiry of expireSeconds, but only when the key
// does not exist yet. It returns 1 when the key was set and 0 when the key
// already existed (no error in that case), and -1 together with an error on
// failure.
func (c *Client) SetNEX(ctx context.Context, key, value string, expireSeconds int64) (int64, error) {
	if key == "" || value == "" {
		return -1, errors.New("redis SET key or value can't be empty")
	}
	// A non-positive expiry would make go-redis store the key without any
	// expiry at all, which contradicts the EX contract of this method.
	if expireSeconds <= 0 {
		return -1, errors.New("redis SETNEX expireSeconds must be positive")
	}

	reply, err := c.client.SetNX(ctx, key, value, time.Duration(expireSeconds)*time.Second).Result()
	if err != nil {
		return -1, err
	}
	if reply {
		return 1, nil
	}
	return 0, nil
}

// SetNX stores key with an unlimited lifetime, but only when the key does not
// exist yet. It returns 1 when the key was set and 0 when the key already
// existed (no error in that case), and -1 together with an error on failure.
func (c *Client) SetNX(ctx context.Context, key, value string) (int64, error) {
	if key == "" || value == "" {
		return -1, errors.New("redis SET key or value can't be empty")
	}

	reply, err := c.client.SetNX(ctx, key, value, 0).Result()
	if err != nil {
		return -1, err
	}
	if reply {
		return 1, nil
	}
	return 0, nil
}

// Del removes key.
func (c *Client) Del(ctx context.Context, key string) error {
	if key == "" {
		return errors.New("redis DEL key can't be empty")
	}

	return c.client.Del(ctx, key).Err()
}

// Incr increments the integer value stored at key by one and returns the new
// value.
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	if key == "" {
		return -1, errors.New("redis INCR key can't be empty")
	}

	return c.client.Incr(ctx, key).Result()
}

// Eval runs the Lua script src. keyCount declares how many of the leading
// entries of keysAndArgs are KEYS; the remaining entries are passed as ARGV.
// This adapts the redigo "EVAL src keyCount keysAndArgs..." convention to
// go-redis's Script.Run(ctx, client, keys, args...).
func (c *Client) Eval(ctx context.Context, src string, keyCount int, keysAndArgs []any) (any, error) {
	if keyCount < 0 || keyCount > len(keysAndArgs) {
		return nil, fmt.Errorf("redis EVAL invalid keyCount %d for %d keys and args", keyCount, len(keysAndArgs))
	}

	keys := make([]string, 0, keyCount)
	for _, k := range keysAndArgs[:keyCount] {
		key, ok := k.(string)
		if !ok {
			return nil, errors.New("redis EVAL key must be a string")
		}
		keys = append(keys, key)
	}

	return redis.NewScript(src).Run(ctx, c.client, keys, keysAndArgs[keyCount:]...).Result()
}
