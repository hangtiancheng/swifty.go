package redis

import (
	"context"
	"errors"

	goredis "github.com/redis/go-redis/v9"
)

// Config holds the Redis client configuration.
type Config struct {
	Address  string
	Password string
	// PoolSize is the maximum number of socket connections.
	// A value of zero uses the go-redis default.
	PoolSize int
	// MinIdleConns is the minimum number of idle connections kept open.
	MinIdleConns int
}

// RClient is a Redis client backed by go-redis.
type RClient struct {
	client goredis.UniversalClient
}

// NewRClient builds a Redis client from the given config.
func NewRClient(config *Config) *RClient {
	return &RClient{client: newRedisClient(config)}
}

func newRedisClient(config *Config) goredis.UniversalClient {
	if config.Address == "" {
		panic("redis address must not be empty")
	}
	return goredis.NewClient(&goredis.Options{
		Addr:         config.Address,
		Password:     config.Password,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
	})
}

// Get reads the value of key. It returns goredis.Nil when the key does not exist.
func (r *RClient) Get(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", errors.New("redis GET key can't be empty")
	}
	return r.client.Get(ctx, key).Result()
}

// SetEx writes key with value and the given expiry in seconds.
func (r *RClient) SetEx(ctx context.Context, key, value string, expireSeconds int64) error {
	if key == "" {
		return errors.New("redis SET EX key can't be empty")
	}
	return r.client.SetEx(ctx, key, value, secondsToDuration(expireSeconds)).Err()
}

// Del removes key.
func (r *RClient) Del(ctx context.Context, key string) error {
	if key == "" {
		return errors.New("redis DEL key can't be empty")
	}
	return r.client.Del(ctx, key).Err()
}

// Eval runs the given Lua script with keyCount keys taken from the head of
// keysAndArgs and the remaining elements as script arguments.
func (r *RClient) Eval(ctx context.Context, src string, keyCount int, keysAndArgs []interface{}) (interface{}, error) {
	if keyCount < 0 || keyCount > len(keysAndArgs) {
		return nil, errors.New("redis EVAL invalid key count")
	}
	keys := make([]string, keyCount)
	for i := 0; i < keyCount; i++ {
		key, ok := keysAndArgs[i].(string)
		if !ok {
			return nil, errors.New("redis EVAL key must be a string")
		}
		keys[i] = key
	}
	return goredis.NewScript(src).Run(ctx, r.client, keys, keysAndArgs[keyCount:]...).Result()
}

// PExpire sets key to expire after expireMilis milliseconds.
func (r *RClient) PExpire(ctx context.Context, key string, expireMilis int64) error {
	return r.client.PExpire(ctx, key, millisToDuration(expireMilis)).Err()
}
