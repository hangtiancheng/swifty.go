// Package pkg provides the infrastructure helpers (Redis client, MySQL
// client and Redis key builders) used by the gotcc example.
package pkg

import (
	"context"
	"fmt"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/redis_lock"
	"github.com/redis/go-redis/v9"
)

// RedisClient is the Redis surface consumed by the example components and
// the example TXStore. Any implementation also works as the lock backend of
// redis_lock.NewRedisLock because it embeds redis_lock.LockClient.
type RedisClient interface {
	redis_lock.LockClient

	// Get returns the string value stored at key. A missing key reports an
	// error that satisfies errors.Is(err, redis.Nil).
	Get(ctx context.Context, key string) (string, error)
	// Set stores key with an unlimited lifetime and returns 1 on success.
	Set(ctx context.Context, key, value string) (int64, error)
	// SetNX stores key only when it does not exist yet. It returns 1 when
	// the key was set and 0 when it already existed.
	SetNX(ctx context.Context, key, value string) (int64, error)
	// Del removes key.
	Del(ctx context.Context, key string) error
	// Ping verifies connectivity to the Redis server.
	Ping(ctx context.Context) error
}

// Compile-time check: the RedisClient abstraction can back the
// redis_lock distributed locks.
var _ redis_lock.LockClient = (RedisClient)(nil)

// redisClient is a RedisClient implementation built on go-redis/v9.
type redisClient struct {
	client *redis.Client
}

// NewRedisClient builds a Redis client. network is "tcp" (or "unix"),
// address is "host:port" and password may be empty when the server requires
// no authentication. The connection is established lazily on first use.
func NewRedisClient(network, address, password string) RedisClient {
	return &redisClient{
		client: redis.NewClient(&redis.Options{
			Network:  network,
			Addr:     address,
			Password: password,
		}),
	}
}

func (c *redisClient) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

func (c *redisClient) Set(ctx context.Context, key, value string) (int64, error) {
	if err := c.client.Set(ctx, key, value, 0).Err(); err != nil {
		return -1, err
	}
	return 1, nil
}

func (c *redisClient) SetNX(ctx context.Context, key, value string) (int64, error) {
	reply, err := c.client.SetNX(ctx, key, value, 0).Result()
	if err != nil {
		return -1, err
	}
	if reply {
		return 1, nil
	}
	return 0, nil
}

func (c *redisClient) Del(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

func (c *redisClient) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// SetNEX implements redis_lock.LockClient: SET key value EX seconds NX.
func (c *redisClient) SetNEX(ctx context.Context, key, value string, expireSeconds int64) (int64, error) {
	reply, err := c.client.SetNX(ctx, key, value, time.Duration(expireSeconds)*time.Second).Result()
	if err != nil {
		return -1, err
	}
	if reply {
		return 1, nil
	}
	return 0, nil
}

// Eval implements redis_lock.LockClient. keyCount declares how many leading
// entries of keysAndArgs are KEYS; the remaining entries are passed as ARGV.
func (c *redisClient) Eval(ctx context.Context, src string, keyCount int, keysAndArgs []interface{}) (interface{}, error) {
	if keyCount < 0 || keyCount > len(keysAndArgs) {
		return nil, fmt.Errorf("redis EVAL invalid keyCount %d for %d keys and args", keyCount, len(keysAndArgs))
	}

	keys := make([]string, 0, keyCount)
	for _, k := range keysAndArgs[:keyCount] {
		key, ok := k.(string)
		if !ok {
			return nil, fmt.Errorf("redis EVAL key must be a string, got %T", k)
		}
		keys = append(keys, key)
	}

	return redis.NewScript(src).Run(ctx, c.client, keys, keysAndArgs[keyCount:]...).Result()
}

// BuildTXKey builds the transaction status key, used for idempotency dedup
// of try requests.
func BuildTXKey(componentID, txID string) string {
	return fmt.Sprintf("txKey:%s:%s", componentID, txID)
}

// BuildTXDetailKey builds the transaction detail key, mapping a transaction
// id to its business id.
func BuildTXDetailKey(componentID, txID string) string {
	return fmt.Sprintf("txDetailKey:%s:%s", componentID, txID)
}

// BuildDataKey builds the business data key, recording the frozen data
// state machine of one business id.
func BuildDataKey(componentID, txID, bizID string) string {
	return fmt.Sprintf("txKey:%s:%s:%s", componentID, txID, bizID)
}

// BuildTXLockKey builds the per-transaction lock key.
func BuildTXLockKey(componentID, txID string) string {
	return fmt.Sprintf("txLockKey:%s:%s", componentID, txID)
}

// BuildTXRecordLockKey builds the lock key guarding the tx_record monitor
// task.
func BuildTXRecordLockKey() string {
	return "gotcc:txRecord:lock"
}
