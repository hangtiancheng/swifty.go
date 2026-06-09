package dao

import (
	"context"
	"log"
	"time"

	"github.com/hangtiancheng/lark-go/lark_cache"
	"lark_chat/internal/config"
)

var (
	UserInfoCache       *lark_cache.Group
	SessionListCache    *lark_cache.Group
	GrpSessionListCache *lark_cache.Group
	MessageListCache    *lark_cache.Group
	GrpMessageListCache *lark_cache.Group
	AuthCodeCache       *lark_cache.Group
)

func InitCache() {
	conf := config.Get()
	maxBytes := conf.Cache.MaxBytes
	expiration := time.Duration(conf.Cache.Expiration) * time.Second

	var dashboardOpts []lark_cache.GroupOption
	dashboardOpts = append(dashboardOpts, lark_cache.WithExpiration(expiration))
	if conf.Cache.DashboardAddr != "" {
		cacheOpts := lark_cache.DefaultCacheOptions()
		cacheOpts.MaxBytes = maxBytes
		cacheOpts.DashboardAddr = conf.Cache.DashboardAddr
		dashboardOpts = append(dashboardOpts, lark_cache.WithCacheOptions(cacheOpts))
	}

	UserInfoCache = lark_cache.NewGroup("user_info", maxBytes, lark_cache.GetterFunc(
		func(ctx context.Context, key string) ([]byte, error) {
			return nil, lark_cache.ErrKeyRequired
		},
	), dashboardOpts...)

	SessionListCache = lark_cache.NewGroup("session_list", maxBytes, lark_cache.GetterFunc(
		func(ctx context.Context, key string) ([]byte, error) {
			return nil, lark_cache.ErrKeyRequired
		},
	), dashboardOpts...)

	GrpSessionListCache = lark_cache.NewGroup("group_session_list", maxBytes, lark_cache.GetterFunc(
		func(ctx context.Context, key string) ([]byte, error) {
			return nil, lark_cache.ErrKeyRequired
		},
	), dashboardOpts...)

	MessageListCache = lark_cache.NewGroup("message_list", maxBytes, lark_cache.GetterFunc(
		func(ctx context.Context, key string) ([]byte, error) {
			return nil, lark_cache.ErrKeyRequired
		},
	), dashboardOpts...)

	GrpMessageListCache = lark_cache.NewGroup("group_message_list", maxBytes, lark_cache.GetterFunc(
		func(ctx context.Context, key string) ([]byte, error) {
			return nil, lark_cache.ErrKeyRequired
		},
	), dashboardOpts...)

	AuthCodeCache = lark_cache.NewGroup("auth_code", maxBytes/4, lark_cache.GetterFunc(
		func(ctx context.Context, key string) ([]byte, error) {
			return nil, lark_cache.ErrKeyRequired
		},
	), dashboardOpts...)

	log.Printf("cache initialized: maxBytes=%d, expiration=%v", maxBytes, expiration)
}

func CloseCache() {
	lark_cache.DestroyAllGroups()
}
