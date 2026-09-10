package redis

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/config"
)

// Client is a Redis client backed by go-redis v9.
type Client struct {
	client *redis.Client
}

// GetClient builds a Redis client from the configuration.
func GetClient(confProvider *config.RedisConfigProvider) *Client {
	return &Client{client: newRedisClient(confProvider.Get())}
}

func newRedisClient(conf *config.RedisConfig) *redis.Client {
	if conf.Address == "" {
		panic("cannot get redis address from config")
	}

	opts := &redis.Options{
		Network:         conf.Network,
		Addr:            conf.Address,
		Password:        conf.Password,
		MaxIdleConns:    conf.MaxIdle,
		PoolSize:        conf.MaxActive,
		ConnMaxIdleTime: time.Duration(conf.IdleTimeoutSeconds) * time.Second,
	}
	if !conf.Wait {
		// A negative pool timeout makes requests fail immediately instead of
		// waiting for a free connection when the pool is exhausted.
		opts.PoolTimeout = -1
	}
	return redis.NewClient(opts)
}

// SetEx executes the Redis SET command with an expiry, expireSeconds in seconds.
func (c *Client) SetEx(ctx context.Context, key, value string, expireSeconds int64) error {
	if key == "" || value == "" {
		return errors.New("redis SET key or value can't be empty")
	}
	return c.client.Set(ctx, key, value, time.Duration(expireSeconds)*time.Second).Err()
}

// SetNX executes the Redis SETNX command with an expiry, expireSeconds in seconds.
func (c *Client) SetNX(ctx context.Context, key, value string, expireSeconds int64) (bool, error) {
	if key == "" || value == "" {
		return false, errors.New("redis SETNX key or value can't be empty")
	}
	return c.client.SetNX(ctx, key, value, time.Duration(expireSeconds)*time.Second).Result()
}

// Get executes the Redis GET command. It returns redis.Nil when the key does
// not exist.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, key).Result()
}

// Exists executes the Redis EXISTS command.
func (c *Client) Exists(ctx context.Context, keys ...string) (bool, error) {
	if len(keys) == 0 {
		return false, errors.New("redis EXISTS args can't be nil or empty")
	}
	cnt, err := c.client.Exists(ctx, keys...).Result()
	return cnt > 0, err
}

// HGet executes the Redis HGET command. It returns an empty string when the
// field does not exist.
func (c *Client) HGet(ctx context.Context, table, key string) (string, error) {
	res, err := c.client.HGet(ctx, table, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	return res, err
}

// HSet executes the Redis HSET command.
func (c *Client) HSet(ctx context.Context, table, key string, value any) error {
	return c.client.HSet(ctx, table, key, value).Err()
}

// ZRangeByScore executes the Redis ZRANGEBYSCORE command, both scores inclusive.
func (c *Client) ZRangeByScore(ctx context.Context, table string, score1, score2 int64) ([]string, error) {
	return c.client.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     table,
		Start:   score1,
		Stop:    score2,
		ByScore: true,
	}).Result()
}

// ZAdd executes the Redis ZADD command.
func (c *Client) ZAdd(ctx context.Context, table string, score int64, value any) error {
	return c.client.ZAdd(ctx, table, redis.Z{Score: float64(score), Member: value}).Err()
}

// Expire executes the Redis EXPIRE command, expireSeconds in seconds.
func (c *Client) Expire(ctx context.Context, key string, expireSeconds int64) error {
	return c.client.Expire(ctx, key, time.Duration(expireSeconds)*time.Second).Err()
}

// SetBit executes the Redis SETBIT command and returns the previous bit value.
func (c *Client) SetBit(ctx context.Context, key string, offset int32) (bool, error) {
	reply, err := c.client.SetBit(ctx, key, int64(offset), 1).Result()
	return reply == 1, err
}

// GetBit executes the Redis GETBIT command.
func (c *Client) GetBit(ctx context.Context, key string, offset int32) (bool, error) {
	reply, err := c.client.GetBit(ctx, key, int64(offset)).Result()
	return reply == 1, err
}

// MGet executes the Redis MGET command; missing keys are returned as empty strings.
func (c *Client) MGet(ctx context.Context, keys ...string) ([]string, error) {
	if len(keys) == 0 {
		return nil, errors.New("redis MGET args can't be nil or empty")
	}

	raws, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	res := make([]string, len(raws))
	for i, raw := range raws {
		if raw == nil {
			continue
		}
		if s, ok := raw.(string); ok {
			res[i] = s
		}
	}
	return res, nil
}

// NewSetCommand builds a SET command for use with Transaction.
func NewSetCommand(args ...any) *Command {
	return &Command{
		Name: "SET",
		Args: args,
	}
}

// NewZAddCommand builds a ZADD command for use with Transaction.
func NewZAddCommand(args ...any) *Command {
	return &Command{
		Name: "ZADD",
		Args: args,
	}
}

// NewSetBitCommand builds a SETBIT command for use with Transaction.
func NewSetBitCommand(args ...any) *Command {
	return &Command{
		Name: "SETBIT",
		Args: args,
	}
}

// NewExpireCommand builds an EXPIRE command for use with Transaction.
func NewExpireCommand(args ...any) *Command {
	return &Command{
		Name: "EXPIRE",
		Args: args,
	}
}

// Command is a raw Redis command executed as part of a MULTI/EXEC transaction.
type Command struct {
	Name string
	Args []any
}

// Transaction runs all commands atomically in a MULTI/EXEC block and returns
// the replies in order.
func (c *Client) Transaction(ctx context.Context, commands ...*Command) ([]any, error) {
	if len(commands) == 0 {
		return nil, nil
	}

	pipe := c.client.TxPipeline()
	cmds := make([]*redis.Cmd, 0, len(commands))
	for _, command := range commands {
		cmd := pipe.Do(ctx, append([]any{command.Name}, command.Args...)...)
		cmds = append(cmds, cmd)
	}

	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	res := make([]any, len(cmds))
	for i, cmd := range cmds {
		v, _ := cmd.Result()
		res[i] = v
	}
	return res, nil
}

// GetDistributionLock returns a reentrant distributed lock bound to key.
func (c *Client) GetDistributionLock(key string) DistributeLocker {
	return NewReentrantDistributeLock(key, c)
}
