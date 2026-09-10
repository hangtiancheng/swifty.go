package redis

import (
	"context"
	"errors"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"github.com/hangtiancheng/swifty.go/apps/consistent_cache"
)

// Client abstracts the Redis operations used by the cache module.
type Client interface {
	Eval(ctx context.Context, src string, keyCount int, keysAndArgs []any) (any, error)
	Get(ctx context.Context, key string) (string, error)
	SetEx(ctx context.Context, key, value string, expireSeconds int64) error
	Del(ctx context.Context, key string) error
	PExpire(ctx context.Context, key string, expireMilis int64) error
}

// Cache is the Redis implementation of the cache module.
type Cache struct {
	client Client
}

// NewRedisCache builds a cache module backed by Redis.
func NewRedisCache(config *Config) *Cache {
	return &Cache{client: NewRClient(config)}
}

// Enable re-enables the read-flow write-cache mechanism for key.
// It shortens the expiry of the disable key to delayMilis milliseconds, so the
// mechanism becomes enabled again after that delay.
func (c *Cache) Enable(ctx context.Context, key string, delayMilis int64) error {
	// Remove the disable key of key in Redis: as long as the disable key does
	// not exist, the read-flow write-cache mechanism is considered enabled.
	// Give the disable key a relatively short expiry.
	return c.client.PExpire(ctx, c.disableKey(key), delayMilis)
}

// Disable disables the read-flow write-cache mechanism for key.
func (c *Cache) Disable(ctx context.Context, key string, expireSeconds int64) error {
	// Set the disable key of key in Redis: as long as the disable key exists,
	// the read-flow write-cache mechanism is considered disabled.
	return c.client.SetEx(ctx, c.disableKey(key), "1", expireSeconds)
}

// Get reads the cached value of key. It returns consistent_cache.ErrorCacheMiss
// when the key does not exist in Redis.
func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	// Read the key/value pair from Redis.
	reply, err := c.client.Get(ctx, key)
	if err != nil && !errors.Is(err, goredis.Nil) {
		return "", err
	}
	if errors.Is(err, goredis.Nil) {
		return "", consistent_cache.ErrorCacheMiss
	}
	return reply, nil
}

// PutWhenEnable writes value for key only when the read-flow write-cache
// mechanism for key is still enabled.
func (c *Cache) PutWhenEnable(ctx context.Context, key, value string, expireSeconds int64) (bool, error) {
	// Run the Redis Lua script to guarantee that the write is performed only
	// when the disable key does not exist.
	reply, err := c.client.Eval(ctx, LuaCheckEnableAndWriteCache, 2, []any{
		c.disableKey(key),
		key,
		value,
		expireSeconds,
	})
	if err != nil {
		return false, err
	}
	result, ok := reply.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected eval reply type %T", reply)
	}
	return result == 1, nil
}

// Del removes the cached value of key.
func (c *Cache) Del(ctx context.Context, key string) error {
	// Delete the key/value pair from Redis.
	return c.client.Del(ctx, key)
}

// disableKey maps the data key to its disable key expression.
func (c *Cache) disableKey(key string) string {
	// The {hash_tag} guarantees that, in Redis cluster mode, the key and its
	// disable key are routed to the same node.
	return fmt.Sprintf("Enable_Lock_Key_{%s}", key)
}
