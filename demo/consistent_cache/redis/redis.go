package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Config holds redis client configuration.
type Config struct {
	Address            string
	Password           string
	DB                 int
	PoolSize           int
	MinIdleConns       int
	ConnMaxIdleSeconds int
	DialTimeoutSeconds int
	ReadTimeoutSeconds int
	WriteTimeoutSeconds int
}

// RClient wraps a github.com/redis/go-redis/v9 client.
type RClient struct {
	client *goredis.Client
}

// NewRClient builds an RClient from Config.
func NewRClient(config *Config) *RClient {
	return &RClient{client: getRedisClient(config)}
}

func getRedisClient(config *Config) *goredis.Client {
	if config.Address == "" {
		panic("redis address is required")
	}

	opts := &goredis.Options{
		Addr:         config.Address,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
	}
	if config.ConnMaxIdleSeconds > 0 {
		opts.ConnMaxIdleTime = time.Duration(config.ConnMaxIdleSeconds) * time.Second
	}
	if config.DialTimeoutSeconds > 0 {
		opts.DialTimeout = time.Duration(config.DialTimeoutSeconds) * time.Second
	}
	if config.ReadTimeoutSeconds > 0 {
		opts.ReadTimeout = time.Duration(config.ReadTimeoutSeconds) * time.Second
	}
	if config.WriteTimeoutSeconds > 0 {
		opts.WriteTimeout = time.Duration(config.WriteTimeoutSeconds) * time.Second
	}
	return goredis.NewClient(opts)
}

func (r *RClient) Get(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", errors.New("redis GET key can't be empty")
	}

	val, err := r.client.Get(ctx, key).Result()
	if errors.Is(err, goredis.Nil) {
		return "", ErrorCacheMiss
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

func (r *RClient) SetEx(ctx context.Context, key, value string, expireSeconds int64) error {
	if key == "" {
		return errors.New("redis SET EX key can't be empty")
	}

	return r.client.SetEx(ctx, key, value, time.Duration(expireSeconds)*time.Second).Err()
}

func (r *RClient) Del(ctx context.Context, key string) error {
	if key == "" {
		return errors.New("redis DEL key can't be empty")
	}

	return r.client.Del(ctx, key).Err()
}

// Eval runs the given Lua script. The first keyCount entries of keysAndArgs are KEYS, the rest are ARGV.
func (r *RClient) Eval(ctx context.Context, src string, keyCount int, keysAndArgs []interface{}) (interface{}, error) {
	keys := make([]string, 0, keyCount)
	args := make([]interface{}, 0, len(keysAndArgs)-keyCount)
	for i, v := range keysAndArgs {
		if i < keyCount {
			keys = append(keys, fmt.Sprintf("%v", v))
		} else {
			args = append(args, v)
		}
	}
	return r.client.Eval(ctx, src, keys, args...).Result()
}

func (r *RClient) PExpire(ctx context.Context, key string, expireMilis int64) error {
	return r.client.PExpire(ctx, key, time.Duration(expireMilis)*time.Millisecond).Err()
}

// ErrorCacheMiss is returned when a redis key is not found.
var ErrorCacheMiss = errors.New("redis cache miss")
