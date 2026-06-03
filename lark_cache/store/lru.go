package store

import (
	"container/list"
	"sync"
	"time"
)

type lruCache struct {
	mu              sync.RWMutex
	list            *list.List
	items           map[string]*list.Element
	expires         map[string]time.Time
	maxBytes        int64
	usedBytes       int64
	onEvicted       func(key string, value Value)
	cleanupInterval time.Duration
	cleanupTicker   *time.Ticker
	closeCh         chan struct{}
	closeOnce       sync.Once
}

type lruEntry struct {
	key   string
	value Value
}

func newLRUCache(opts Options) *lruCache {
	cleanupInterval := opts.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = time.Minute
	}

	c := &lruCache{
		list:            list.New(),
		items:           make(map[string]*list.Element),
		expires:         make(map[string]time.Time),
		maxBytes:        opts.MaxBytes,
		onEvicted:       opts.OnEvicted,
		cleanupInterval: cleanupInterval,
		closeCh:         make(chan struct{}),
	}

	c.cleanupTicker = time.NewTicker(c.cleanupInterval)
	go c.cleanupLoop()

	return c
}

func (c *lruCache) Get(key string) (Value, bool) {
	c.mu.RLock()
	elem, ok := c.items[key]
	if !ok {
		c.mu.RUnlock()
		return nil, false
	}

	if expTime, hasExp := c.expires[key]; hasExp && time.Now().After(expTime) {
		c.mu.RUnlock()

		go c.Delete(key)

		return nil, false
	}

	entry := elem.Value.(*lruEntry)
	value := entry.value
	c.mu.RUnlock()

	c.mu.Lock()
	if _, ok := c.items[key]; ok {
		c.list.MoveToBack(elem)
	}
	c.mu.Unlock()

	return value, true
}

func (c *lruCache) Set(key string, value Value) error {
	return c.SetWithExpiration(key, value, 0)
}

func (c *lruCache) SetWithExpiration(key string, value Value, expiration time.Duration) error {
	if value == nil {
		c.Delete(key)
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var expTime time.Time
	if expiration > 0 {
		expTime = time.Now().Add(expiration)
		c.expires[key] = expTime
	} else {
		delete(c.expires, key)
	}

	if elem, ok := c.items[key]; ok {
		oldEntry := elem.Value.(*lruEntry)
		c.usedBytes += int64(value.Len() - oldEntry.value.Len())
		oldEntry.value = value
		c.list.MoveToBack(elem)
		return nil
	}

	entry := &lruEntry{key: key, value: value}
	elem := c.list.PushBack(entry)
	c.items[key] = elem
	c.usedBytes += int64(len(key) + value.Len())

	c.evict()

	return nil
}

func (c *lruCache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.removeElement(elem)
		return true
	}
	return false
}

func (c *lruCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.onEvicted != nil {
		for _, elem := range c.items {
			entry := elem.Value.(*lruEntry)
			c.onEvicted(entry.key, entry.value)
		}
	}

	c.list.Init()
	c.items = make(map[string]*list.Element)
	c.expires = make(map[string]time.Time)
	c.usedBytes = 0
}

func (c *lruCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.list.Len()
}

func (c *lruCache) removeElement(elem *list.Element) {
	entry := elem.Value.(*lruEntry)
	c.list.Remove(elem)
	delete(c.items, entry.key)
	delete(c.expires, entry.key)
	c.usedBytes -= int64(len(entry.key) + entry.value.Len())

	if c.onEvicted != nil {
		c.onEvicted(entry.key, entry.value)
	}
}

func (c *lruCache) evict() {
	now := time.Now()
	for key, expTime := range c.expires {
		if now.After(expTime) {
			if elem, ok := c.items[key]; ok {
				c.removeElement(elem)
			}
		}
	}

	for c.maxBytes > 0 && c.usedBytes > c.maxBytes && c.list.Len() > 0 {
		elem := c.list.Front()
		if elem != nil {
			c.removeElement(elem)
		}
	}
}

func (c *lruCache) cleanupLoop() {
	for {
		select {
		case <-c.cleanupTicker.C:
			c.mu.Lock()
			c.evict()
			c.mu.Unlock()
		case <-c.closeCh:
			return
		}
	}
}

func (c *lruCache) Close() {
	c.closeOnce.Do(func() {
		if c.cleanupTicker != nil {
			c.cleanupTicker.Stop()
		}
		close(c.closeCh)
	})
}

func (c *lruCache) GetWithExpiration(key string) (Value, time.Duration, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, 0, false
	}

	now := time.Now()
	if expTime, hasExp := c.expires[key]; hasExp {
		if now.After(expTime) {
			return nil, 0, false
		}

		ttl := expTime.Sub(now)
		c.list.MoveToBack(elem)
		return elem.Value.(*lruEntry).value, ttl, true
	}

	c.list.MoveToBack(elem)
	return elem.Value.(*lruEntry).value, 0, true
}

func (c *lruCache) GetExpiration(key string) (time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	expTime, ok := c.expires[key]
	return expTime, ok
}

func (c *lruCache) UpdateExpiration(key string, expiration time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.items[key]; !ok {
		return false
	}

	if expiration > 0 {
		c.expires[key] = time.Now().Add(expiration)
	} else {
		delete(c.expires, key)
	}

	return true
}

func (c *lruCache) UsedBytes() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.usedBytes
}

func (c *lruCache) MaxBytes() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.maxBytes
}

func (c *lruCache) SetMaxBytes(maxBytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.maxBytes = maxBytes
	if maxBytes > 0 {
		c.evict()
	}
}
