// Copyright (c) 2026 hangtiancheng
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package swifty_cache

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Cache wraps the underlying store implementation with lazy initialization and stats.
type Cache struct {
	mu          sync.RWMutex
	store       Store
	opts        CacheOptions
	hits        int64
	misses      int64
	initialized int32
	closed      int32
}

// CacheOptions configures the underlying cache store.
type CacheOptions struct {
	MaxBytes      int64
	BucketCount   uint16
	CapPerBucket  uint16
	Level2Cap     uint16
	CleanupTime   time.Duration
	OnEvicted     func(key string, value Value)
	DashboardAddr string
}

// DefaultCacheOptions returns the default cache settings.
func DefaultCacheOptions() CacheOptions {
	return CacheOptions{
		MaxBytes:     8 * 1024 * 1024,
		BucketCount:  16,
		CapPerBucket: 512,
		Level2Cap:    256,
		CleanupTime:  time.Minute,
		OnEvicted:    nil,
	}
}

// NewCache creates a lazily initialized cache wrapper.
func NewCache(opts CacheOptions) *Cache {
	return &Cache{opts: opts}
}

func (c *Cache) ensureInitialized() {
	if atomic.LoadInt32(&c.initialized) == 1 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if atomic.LoadInt32(&c.closed) == 1 {
		return
	}
	if atomic.LoadInt32(&c.initialized) == 0 {
		storeOpts := StoreOptions{
			MaxBytes:        c.opts.MaxBytes,
			BucketCount:     c.opts.BucketCount,
			CapPerBucket:    c.opts.CapPerBucket,
			Level2Cap:       c.opts.Level2Cap,
			CleanupInterval: c.opts.CleanupTime,
			OnEvicted:       c.opts.OnEvicted,
		}
		c.store = NewStore(storeOpts)
		atomic.StoreInt32(&c.initialized, 1)
		log.Printf("Cache initialized, max bytes: %d", c.opts.MaxBytes)
	}
}

// Add stores a key-value pair.
func (c *Cache) Add(key string, value ByteView) {
	if atomic.LoadInt32(&c.closed) == 1 {
		log.Printf("Attempted to add to a closed cache: %s", key)
		return
	}

	c.ensureInitialized()

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.store == nil {
		return
	}
	if err := c.store.Set(key, value); err != nil {
		log.Printf("Failed to add key %s to cache: %v", key, err)
	}
}

// Get returns a cached value when it exists and has not expired.
func (c *Cache) Get(ctx context.Context, key string) (ByteView, bool) {
	if atomic.LoadInt32(&c.closed) == 1 {
		return ByteView{}, false
	}
	if atomic.LoadInt32(&c.initialized) == 0 {
		atomic.AddInt64(&c.misses, 1)
		return ByteView{}, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.store == nil {
		atomic.AddInt64(&c.misses, 1)
		return ByteView{}, false
	}

	val, found := c.store.Get(key)
	if !found {
		atomic.AddInt64(&c.misses, 1)
		return ByteView{}, false
	}

	bv, ok := val.(ByteView)
	if !ok {
		log.Printf("Type assertion failed for key %s, expected ByteView", key)
		atomic.AddInt64(&c.misses, 1)
		return ByteView{}, false
	}

	atomic.AddInt64(&c.hits, 1)
	return bv, true
}

// AddWithExpiration stores a key-value pair with an absolute expiration time.
func (c *Cache) AddWithExpiration(key string, value ByteView, expirationTime time.Time) {
	if atomic.LoadInt32(&c.closed) == 1 {
		log.Printf("Attempted to add to a closed cache: %s", key)
		return
	}

	expiration := time.Until(expirationTime)
	if expiration <= 0 {
		log.Printf("Key %s already expired, not adding to cache", key)
		return
	}

	c.ensureInitialized()

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.store == nil {
		return
	}
	if err := c.store.SetWithExpiration(key, value, expiration); err != nil {
		log.Printf("Failed to add key %s to cache with expiration: %v", key, err)
	}
}

// Delete removes a key from the cache.
func (c *Cache) Delete(key string) bool {
	if atomic.LoadInt32(&c.closed) == 1 || atomic.LoadInt32(&c.initialized) == 0 {
		return false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.store == nil {
		return false
	}
	return c.store.Delete(key)
}

// Clear removes all cached values and resets hit/miss counters.
func (c *Cache) Clear() {
	if atomic.LoadInt32(&c.closed) == 1 || atomic.LoadInt32(&c.initialized) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.store == nil {
		return
	}
	c.store.Clear()
	atomic.StoreInt64(&c.hits, 0)
	atomic.StoreInt64(&c.misses, 0)
}

// Len returns the number of stored entries.
func (c *Cache) Len() int {
	if atomic.LoadInt32(&c.closed) == 1 || atomic.LoadInt32(&c.initialized) == 0 {
		return 0
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.store == nil {
		return 0
	}
	return c.store.Len()
}

// Close releases cache resources. It is safe to call more than once.
func (c *Cache) Close() {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.store != nil {
		c.store.Close()
		c.store = nil
	}
	atomic.StoreInt32(&c.initialized, 0)
	log.Printf("Cache closed, hits: %d, misses: %d", atomic.LoadInt64(&c.hits), atomic.LoadInt64(&c.misses))
}

// DashboardEnabled reports whether the dashboard is enabled for this cache.
func (c *Cache) DashboardEnabled() bool {
	return c.opts.DashboardAddr != ""
}

// Entries returns all live cache entries.
func (c *Cache) Entries() []Entry {
	if atomic.LoadInt32(&c.closed) == 1 || atomic.LoadInt32(&c.initialized) == 0 {
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.store == nil {
		return nil
	}

	var entries []Entry
	c.store.Walk(func(e Entry) bool {
		entries = append(entries, e)
		return true
	})
	return entries
}

// Stats returns a cache statistics snapshot.
func (c *Cache) Stats() map[string]any {
	stats := map[string]any{
		"initialized": atomic.LoadInt32(&c.initialized) == 1,
		"closed":      atomic.LoadInt32(&c.closed) == 1,
		"hits":        atomic.LoadInt64(&c.hits),
		"misses":      atomic.LoadInt64(&c.misses),
	}

	if atomic.LoadInt32(&c.initialized) == 1 {
		stats["size"] = c.Len()
		totalRequests := stats["hits"].(int64) + stats["misses"].(int64)
		if totalRequests > 0 {
			stats["hit_rate"] = float64(stats["hits"].(int64)) / float64(totalRequests)
		} else {
			stats["hit_rate"] = 0.0
		}

		c.mu.RLock()
		if s, ok := c.store.(*lruStore); ok {
			stats["bytes"] = s.Bytes()
			stats["evictions"] = s.Evictions()
		}
		c.mu.RUnlock()
	}

	return stats
}
