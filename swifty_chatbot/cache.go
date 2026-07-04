package main

import (
	"context"

	swifty_cache "github.com/hangtiancheng/swifty.go/swifty_cache"
)

type cache struct {
	group *swifty_cache.Group
}

func newCache() *cache {
	return &cache{group: swifty_cache.NewGroup("swifty_ai", 64<<20, swifty_cache.GetterFunc(func(ctx context.Context, key string) ([]byte, error) {
		return nil, swifty_cache.ErrKeyRequired
	}))}
}

func (c *cache) Get(ctx context.Context, key string) (string, bool) {
	view, err := c.group.Get(ctx, key)
	if err != nil {
		return "", false
	}
	return view.String(), true
}

func (c *cache) Set(ctx context.Context, key string, value string) error {
	return c.group.Set(ctx, key, []byte(value))
}

func (c *cache) Delete(ctx context.Context, key string) error {
	return c.group.Delete(ctx, key)
}
