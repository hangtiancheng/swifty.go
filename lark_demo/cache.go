package main

import (
	"context"

	lark_cache "github.com/hangtiancheng/lark_cache"
)

type cache struct {
	group *lark_cache.Group
}

func newCache() *cache {
	return &cache{group: lark_cache.NewGroup("lark_demo", 64<<20, lark_cache.GetterFunc(func(ctx context.Context, key string) ([]byte, error) {
		return nil, lark_cache.ErrKeyRequired
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
