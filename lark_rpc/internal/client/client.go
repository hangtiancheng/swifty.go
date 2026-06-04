package client

import (
	"sync"
	"time"

	"github.com/hangtiancheng/lark-go/lark_rpc/internal/codec"
	"github.com/hangtiancheng/lark-go/lark_rpc/internal/limiter"
	"github.com/hangtiancheng/lark-go/lark_rpc/internal/load_balance"
	"github.com/hangtiancheng/lark-go/lark_rpc/internal/registry"
	"github.com/hangtiancheng/lark-go/lark_rpc/internal/transport"
)

type Client struct {
	reg     *registry.Registry
	lb      load_balance.LoadBalancer
	limiter *limiter.TokenBucket
	timeout time.Duration
	codec   codec.Codec
	breaker sync.Map // map[string]*CircuitBreaker

	pools sync.Map // map[string]*transport.ConnectionPool
}

func NewClient(reg *registry.Registry, opts ...ClientOption) (*Client, error) {
	cc, err := codec.New(codec.JSON)
	if err != nil {
		return nil, err
	}

	c := &Client{
		reg:     reg,
		lb:      &load_balance.RoundRobin{},
		limiter: limiter.NewTokenBucket(10000),
		timeout: 5 * time.Second,
		codec:   cc,
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *Client) Close() {
	c.pools.Range(func(key, value interface{}) bool {
		pool := value.(*transport.ConnectionPool)
		pool.Close()
		return true
	})
}
