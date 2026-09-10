// Package redis provides a thin wrapper around github.com/redis/go-redis/v9
// used by the timewheel's distributed (redis-backed) time wheel.
package redis

import (
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Client is a redis client backed by github.com/redis/go-redis/v9. The
// underlying *goredis.Client is embedded, so all go-redis commands (Get, Set,
// Eval, ...) are available directly on the Client.
type Client struct {
	*goredis.Client
}

// NewClient creates a new redis client. The connection is established lazily
// on first use.
func NewClient(network, address, password string, opts ...ClientOption) *Client {
	c := &ClientOptions{
		network:  network,
		address:  address,
		password: password,
	}
	for _, opt := range opts {
		opt(c)
	}
	repairClient(c)

	if c.address == "" {
		panic("timewheel/redis: redis address is empty")
	}

	return &Client{
		Client: goredis.NewClient(&goredis.Options{
			Network:         c.network,
			Addr:            c.address,
			Password:        c.password,
			PoolSize:        c.maxActive,
			MaxIdleConns:    c.maxIdle,
			ConnMaxIdleTime: time.Duration(c.idleTimeoutSeconds) * time.Second,
		}),
	}
}
