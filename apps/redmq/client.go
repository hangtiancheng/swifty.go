package redmq

import "github.com/hangtiancheng/swifty.go/apps/redmq/internal/redis"

// Client is the Redis client used by Producer and Consumer. It is an alias
// of the Redis client implementation shipped with this module.
type Client = redis.Client

// ClientOption configures optional Client settings, such as connection pool
// behaviour.
type ClientOption = redis.ClientOption

// MsgEntity is a single message received from a stream.
type MsgEntity = redis.MsgEntity

// NewRedisClient creates a Redis client with the given network, address and
// password. Optional pool behaviour can be configured with ClientOption.
func NewRedisClient(network, address, password string, opts ...ClientOption) *Client {
	return redis.NewClient(network, address, password, opts...)
}
