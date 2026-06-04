package lark_cache

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

const maxExpireAt = math.MaxInt64

type lruStore struct {
	locks       []sync.Mutex
	caches      [][2]*cache
	onEvicted   func(key string, value Value)
	cleanupTick *time.Ticker
	closeCh     chan struct{}
	closeOnce   sync.Once
	mask        int32
}

// NewStore creates a cache store implementation.
func NewStore(opts StoreOptions) *lruStore {
	if opts.BucketCount == 0 {
		opts.BucketCount = 16
	}
	if opts.CapPerBucket == 0 {
		opts.CapPerBucket = 1024
	}
	if opts.Level2Cap == 0 {
		opts.Level2Cap = 1024
	}
	if opts.CleanupInterval <= 0 {
		opts.CleanupInterval = time.Minute
	}

	mask := MaskOfNextPowOf2(opts.BucketCount)
	s := &lruStore{
		locks:       make([]sync.Mutex, mask+1),
		caches:      make([][2]*cache, mask+1),
		onEvicted:   opts.OnEvicted,
		cleanupTick: time.NewTicker(opts.CleanupInterval),
		closeCh:     make(chan struct{}),
		mask:        int32(mask),
	}

	for i := range s.caches {
		s.caches[i][0] = Create(opts.CapPerBucket)
		s.caches[i][1] = Create(opts.Level2Cap)
	}

	if opts.CleanupInterval > 0 {
		go s.cleanupLoop()
	}

	return s
}

func (s *lruStore) Get(key string) (Value, bool) {
	idx := HashBKRD(key) & s.mask
	s.locks[idx].Lock()
	defer s.locks[idx].Unlock()

	currentTime := Now()

	n1, status1, expireAt := s.caches[idx][0].del(key)
	if status1 > 0 {
		if expireAt > 0 && currentTime >= expireAt {
			s.delete(key, idx)
			return nil, false
		}

		s.caches[idx][1].put(key, n1.v, expireAt, s.onEvicted)
		return n1.v, true
	}

	n2, status2 := s._get(key, idx, 1)
	if status2 > 0 && n2 != nil {
		if n2.expireAt > 0 && currentTime >= n2.expireAt {
			s.delete(key, idx)
			return nil, false
		}

		return n2.v, true
	}

	return nil, false
}

func (s *lruStore) Set(key string, value Value) error {
	return s.SetWithExpiration(key, value, 0)
}

func (s *lruStore) SetWithExpiration(key string, value Value, expiration time.Duration) error {
	var expireAt int64
	if expiration > 0 {
		expireAt = Now() + int64(expiration.Nanoseconds())
	} else {
		expireAt = maxExpireAt
	}

	idx := HashBKRD(key) & s.mask
	s.locks[idx].Lock()
	defer s.locks[idx].Unlock()

	s.caches[idx][0].put(key, value, expireAt, s.onEvicted)

	return nil
}

func (s *lruStore) Delete(key string) bool {
	idx := HashBKRD(key) & s.mask
	s.locks[idx].Lock()
	defer s.locks[idx].Unlock()

	return s.delete(key, idx)
}

func (s *lruStore) Clear() {
	var keys []string

	for i := range s.caches {
		s.locks[i].Lock()

		s.caches[i][0].walk(func(key string, value Value, expireAt int64) bool {
			keys = append(keys, key)
			return true
		})
		s.caches[i][1].walk(func(key string, value Value, expireAt int64) bool {
			for _, k := range keys {
				if key == k {
					return true
				}
			}
			keys = append(keys, key)
			return true
		})

		s.locks[i].Unlock()
	}

	for _, key := range keys {
		s.Delete(key)
	}

	//s.expirations = sync.Map{}
}

func (s *lruStore) Len() int {
	count := 0

	for i := range s.caches {
		s.locks[i].Lock()

		s.caches[i][0].walk(func(key string, value Value, expireAt int64) bool {
			count++
			return true
		})
		s.caches[i][1].walk(func(key string, value Value, expireAt int64) bool {
			count++
			return true
		})

		s.locks[i].Unlock()
	}

	return count
}

func (s *lruStore) Close() {
	s.closeOnce.Do(func() {
		if s.cleanupTick != nil {
			s.cleanupTick.Stop()
		}
		close(s.closeCh)
	})
}

var clock, p, n = time.Now().UnixNano(), uint16(0), uint16(1)

func Now() int64 { return atomic.LoadInt64(&clock) }

func init() {
	go func() {
		for {
			atomic.StoreInt64(&clock, time.Now().UnixNano())
			for i := 0; i < 9; i++ {
				time.Sleep(100 * time.Millisecond)
				atomic.AddInt64(&clock, int64(100*time.Millisecond))
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
}

func HashBKRD(s string) (hash int32) {
	for i := 0; i < len(s); i++ {
		hash = hash*131 + int32(s[i])
	}

	return hash
}

func MaskOfNextPowOf2(cap uint16) uint16 {
	if cap > 0 && cap&(cap-1) == 0 {
		return cap - 1
	}

	cap |= cap >> 1
	cap |= cap >> 2
	cap |= cap >> 4

	return cap | (cap >> 8)
}

type node struct {
	k        string
	v        Value
	expireAt int64
}

type cache struct {
	doubleLink [][2]uint16
	m          []node
	hashMap    map[string]uint16
	last       uint16
}

func Create(cap uint16) *cache {
	return &cache{
		doubleLink: make([][2]uint16, cap+1),
		m:          make([]node, cap),
		hashMap:    make(map[string]uint16, cap),
		last:       0,
	}
}

func (c *cache) put(key string, val Value, expireAt int64, onEvicted func(string, Value)) int {
	if idx, ok := c.hashMap[key]; ok {
		c.m[idx-1].v, c.m[idx-1].expireAt = val, expireAt
		c.adjust(idx, p, n)
		return 0
	}

	if c.last == uint16(cap(c.m)) {
		tail := &c.m[c.doubleLink[0][p]-1]
		if onEvicted != nil && (*tail).expireAt > 0 {
			onEvicted((*tail).k, (*tail).v)
		}

		delete(c.hashMap, (*tail).k)
		c.hashMap[key], (*tail).k, (*tail).v, (*tail).expireAt = c.doubleLink[0][p], key, val, expireAt
		c.adjust(c.doubleLink[0][p], p, n)

		return 1
	}

	c.last++
	if len(c.hashMap) <= 0 {
		c.doubleLink[0][p] = c.last
	} else {
		c.doubleLink[c.doubleLink[0][n]][p] = c.last
	}

	c.m[c.last-1].k = key
	c.m[c.last-1].v = val
	c.m[c.last-1].expireAt = expireAt
	c.doubleLink[c.last] = [2]uint16{0, c.doubleLink[0][n]}
	c.hashMap[key] = c.last
	c.doubleLink[0][n] = c.last

	return 1
}

func (c *cache) get(key string) (*node, int) {
	if idx, ok := c.hashMap[key]; ok {
		c.adjust(idx, p, n)
		return &c.m[idx-1], 1
	}
	return nil, 0
}

func (c *cache) del(key string) (*node, int, int64) {
	if idx, ok := c.hashMap[key]; ok && c.m[idx-1].expireAt > 0 {
		e := c.m[idx-1].expireAt
		c.m[idx-1].expireAt = 0
		c.adjust(idx, n, p)
		return &c.m[idx-1], 1, e
	}

	return nil, 0, 0
}

func (c *cache) walk(walker func(key string, value Value, expireAt int64) bool) {
	for idx := c.doubleLink[0][n]; idx != 0; idx = c.doubleLink[idx][n] {
		if c.m[idx-1].expireAt > 0 && !walker(c.m[idx-1].k, c.m[idx-1].v, c.m[idx-1].expireAt) {
			return
		}
	}
}

func (c *cache) adjust(idx, f, t uint16) {
	if c.doubleLink[idx][f] != 0 {
		c.doubleLink[c.doubleLink[idx][t]][f] = c.doubleLink[idx][f]
		c.doubleLink[c.doubleLink[idx][f]][t] = c.doubleLink[idx][t]
		c.doubleLink[idx][f] = 0
		c.doubleLink[idx][t] = c.doubleLink[0][t]
		c.doubleLink[c.doubleLink[0][t]][f] = idx
		c.doubleLink[0][t] = idx
	}
}

func (s *lruStore) _get(key string, idx, level int32) (*node, int) {
	if n, st := s.caches[idx][level].get(key); st > 0 && n != nil {
		currentTime := Now()
		if n.expireAt <= 0 || currentTime >= n.expireAt {
			return nil, 0
		}
		return n, st
	}

	return nil, 0
}

func (s *lruStore) delete(key string, idx int32) bool {
	n1, s1, _ := s.caches[idx][0].del(key)
	n2, s2, _ := s.caches[idx][1].del(key)
	deleted := s1 > 0 || s2 > 0

	if deleted && s.onEvicted != nil {
		if n1 != nil && n1.v != nil {
			s.onEvicted(key, n1.v)
		} else if n2 != nil && n2.v != nil {
			s.onEvicted(key, n2.v)
		}
	}

	if deleted {
		//s.expirations.Delete(key)
	}

	return deleted
}

func (s *lruStore) cleanupLoop() {
	for {
		select {
		case <-s.cleanupTick.C:
			currentTime := Now()

			for i := range s.caches {
				s.locks[i].Lock()

				var expiredKeys []string

				s.caches[i][0].walk(func(key string, value Value, expireAt int64) bool {
					if expireAt > 0 && currentTime >= expireAt {
						expiredKeys = append(expiredKeys, key)
					}
					return true
				})

				s.caches[i][1].walk(func(key string, value Value, expireAt int64) bool {
					if expireAt > 0 && currentTime >= expireAt {
						for _, k := range expiredKeys {
							if key == k {
								return true
							}
						}
						expiredKeys = append(expiredKeys, key)
					}
					return true
				})

				for _, key := range expiredKeys {
					s.delete(key, int32(i))
				}

				s.locks[i].Unlock()
			}
		case <-s.closeCh:
			return
		}
	}
}
