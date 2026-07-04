package dao

import (
	"context"
	"log"
	"time"

	"github.com/hangtiancheng/swifty.go/swifty_chat/internal/config"

	"github.com/hangtiancheng/swifty.go/swifty_cache"
)

var (
	UserInfoCache       *swifty_cache.Group
	SessionListCache    *swifty_cache.Group
	GrpSessionListCache *swifty_cache.Group
	MessageListCache    *swifty_cache.Group
	GrpMessageListCache *swifty_cache.Group
	AuthCodeCache       *swifty_cache.Group
)

func InitCache() {
	conf := config.Get()
	maxBytes := conf.Cache.MaxBytes
	expiration := time.Duration(conf.Cache.Expiration) * time.Second

	var dashboardOpts []swifty_cache.GroupOption
	dashboardOpts = append(dashboardOpts, swifty_cache.WithExpiration(expiration))
	if conf.Cache.DashboardAddr != "" {
		cacheOpts := swifty_cache.DefaultCacheOptions()
		cacheOpts.MaxBytes = maxBytes
		cacheOpts.DashboardAddr = conf.Cache.DashboardAddr
		dashboardOpts = append(dashboardOpts, swifty_cache.WithCacheOptions(cacheOpts))
	}

	UserInfoCache = swifty_cache.NewGroup("user_info", maxBytes, swifty_cache.GetterFunc(
		func(ctx context.Context, key string) ([]byte, error) {
			return nil, swifty_cache.ErrKeyRequired
		},
	), dashboardOpts...)

	SessionListCache = swifty_cache.NewGroup("session_list", maxBytes, swifty_cache.GetterFunc(
		func(ctx context.Context, key string) ([]byte, error) {
			return nil, swifty_cache.ErrKeyRequired
		},
	), dashboardOpts...)

	GrpSessionListCache = swifty_cache.NewGroup("group_session_list", maxBytes, swifty_cache.GetterFunc(
		func(ctx context.Context, key string) ([]byte, error) {
			return nil, swifty_cache.ErrKeyRequired
		},
	), dashboardOpts...)

	MessageListCache = swifty_cache.NewGroup("message_list", maxBytes, swifty_cache.GetterFunc(
		func(ctx context.Context, key string) ([]byte, error) {
			return nil, swifty_cache.ErrKeyRequired
		},
	), dashboardOpts...)

	GrpMessageListCache = swifty_cache.NewGroup("group_message_list", maxBytes, swifty_cache.GetterFunc(
		func(ctx context.Context, key string) ([]byte, error) {
			return nil, swifty_cache.ErrKeyRequired
		},
	), dashboardOpts...)

	AuthCodeCache = swifty_cache.NewGroup("auth_code", maxBytes/4, swifty_cache.GetterFunc(
		func(ctx context.Context, key string) ([]byte, error) {
			return nil, swifty_cache.ErrKeyRequired
		},
	), dashboardOpts...)

	log.Printf("cache initialized: maxBytes=%d, expiration=%v", maxBytes, expiration)
}

func CloseCache() {
	swifty_cache.DestroyAllGroups()
}
