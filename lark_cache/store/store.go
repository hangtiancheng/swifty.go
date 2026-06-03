package store

import "time"

// Value is a cache value that reports its memory size.
type Value interface {
	Len() int
}

// Store is the interface shared by cache storage implementations.
type Store interface {
	Get(key string) (Value, bool)
	Set(key string, value Value) error
	SetWithExpiration(key string, value Value, expiration time.Duration) error
	Delete(key string) bool
	Clear()
	Len() int
	Close()
}

// CacheType identifies the storage implementation.
type CacheType string

const (
	LRU  CacheType = "lru"
	LRU2 CacheType = "lru2"
)

// Options configures store implementations.
type Options struct {
	MaxBytes        int64
	BucketCount     uint16
	CapPerBucket    uint16
	Level2Cap       uint16
	CleanupInterval time.Duration
	OnEvicted       func(key string, value Value)
}

// NewOptions returns default store options.
func NewOptions() Options {
	return Options{
		MaxBytes:        8192,
		BucketCount:     16,
		CapPerBucket:    512,
		Level2Cap:       256,
		CleanupInterval: time.Minute,
		OnEvicted:       nil,
	}
}

// NewStore creates a cache store implementation.
func NewStore(cacheType CacheType, opts Options) Store {
	switch cacheType {
	case LRU2:
		return newLRU2Cache(opts)
	case LRU:
		return newLRUCache(opts)
	default:
		return newLRUCache(opts)
	}
}
