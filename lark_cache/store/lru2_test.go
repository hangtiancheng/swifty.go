package store

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

type testValue string

func (v testValue) Len() int { return len(v) }

func TestNewOptionsAndNewStore(t *testing.T) {
	opts := NewOptions()
	if opts.MaxBytes <= 0 || opts.BucketCount == 0 || opts.CapPerBucket == 0 || opts.Level2Cap == 0 {
		t.Fatalf("unexpected default options: %+v", opts)
	}

	stores := []Store{
		NewStore(LRU, Options{MaxBytes: 64, CleanupInterval: time.Hour}),
		NewStore(LRU2, Options{BucketCount: 1, CapPerBucket: 2, Level2Cap: 2, CleanupInterval: time.Hour}),
		NewStore(CacheType("unknown"), Options{MaxBytes: 64, CleanupInterval: time.Hour}),
	}
	for _, s := range stores {
		if s == nil {
			t.Fatal("NewStore returned nil")
		}
		s.Close()
	}
}

func TestLRUCacheBasicOperations(t *testing.T) {
	var evicted []string
	c := newLRUCache(Options{
		MaxBytes:        int64(len("a") + len("alpha") + len("c") + len("gamma")),
		CleanupInterval: time.Hour,
		OnEvicted: func(key string, value Value) {
			evicted = append(evicted, key)
		},
	})
	defer c.Close()

	if _, ok := c.Get("missing"); ok {
		t.Fatal("missing key returned a hit")
	}

	if err := c.Set("a", testValue("alpha")); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := c.Set("b", testValue("beta")); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if got, ok := c.Get("a"); !ok || got != testValue("alpha") {
		t.Fatalf("Get(a) = %v, %v", got, ok)
	}

	if err := c.Set("c", testValue("gamma")); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if _, ok := c.Get("b"); ok {
		t.Fatal("least recently used key b should have been evicted")
	}
	if len(evicted) == 0 || evicted[0] != "b" {
		t.Fatalf("unexpected evictions: %v", evicted)
	}

	if !c.Delete("a") {
		t.Fatal("Delete(a) returned false")
	}
	if c.Delete("missing") {
		t.Fatal("Delete(missing) returned true")
	}
	c.Clear()
	if c.Len() != 0 || c.UsedBytes() != 0 {
		t.Fatalf("cache not empty after Clear: len=%d bytes=%d", c.Len(), c.UsedBytes())
	}
}

func TestLRUCacheExpirationAndMetadata(t *testing.T) {
	c := newLRUCache(Options{MaxBytes: 128, CleanupInterval: 10 * time.Millisecond})
	defer c.Close()

	if err := c.SetWithExpiration("soon", testValue("value"), 20*time.Millisecond); err != nil {
		t.Fatalf("SetWithExpiration returned error: %v", err)
	}
	if got, ttl, ok := c.GetWithExpiration("soon"); !ok || got != testValue("value") || ttl <= 0 {
		t.Fatalf("GetWithExpiration returned (%v, %v, %v)", got, ttl, ok)
	}
	if _, ok := c.GetExpiration("soon"); !ok {
		t.Fatal("GetExpiration did not find key")
	}
	if !c.UpdateExpiration("soon", time.Hour) {
		t.Fatal("UpdateExpiration returned false")
	}
	if c.UpdateExpiration("missing", time.Hour) {
		t.Fatal("UpdateExpiration returned true for missing key")
	}
	if c.MaxBytes() != 128 {
		t.Fatalf("MaxBytes = %d, want 128", c.MaxBytes())
	}
	c.SetMaxBytes(int64(len("soon") + len("value")))
	if c.UsedBytes() > c.MaxBytes() {
		t.Fatalf("UsedBytes = %d exceeds MaxBytes = %d", c.UsedBytes(), c.MaxBytes())
	}

	if err := c.SetWithExpiration("expired", testValue("value"), 10*time.Millisecond); err != nil {
		t.Fatalf("SetWithExpiration returned error: %v", err)
	}
	time.Sleep(30 * time.Millisecond)
	if _, ok := c.Get("expired"); ok {
		t.Fatal("expired key returned a hit")
	}

	c.Close()
	c.Close()
}

func TestLRUCacheNilValueDeletes(t *testing.T) {
	c := newLRUCache(Options{MaxBytes: 128, CleanupInterval: time.Hour})
	defer c.Close()

	if err := c.Set("key", testValue("value")); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := c.Set("key", nil); err != nil {
		t.Fatalf("Set nil returned error: %v", err)
	}
	if _, ok := c.Get("key"); ok {
		t.Fatal("nil Set should delete the key")
	}
}

func TestLRU2StoreBasicOperations(t *testing.T) {
	var evicted []string
	s := newLRU2Cache(Options{
		BucketCount:     1,
		CapPerBucket:    2,
		Level2Cap:       2,
		CleanupInterval: time.Hour,
		OnEvicted: func(key string, value Value) {
			evicted = append(evicted, key)
		},
	})
	defer s.Close()

	if err := s.Set("a", testValue("alpha")); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if got, ok := s.Get("a"); !ok || got != testValue("alpha") {
		t.Fatalf("Get(a) = %v, %v", got, ok)
	}
	if got, ok := s.Get("a"); !ok || got != testValue("alpha") {
		t.Fatalf("second Get(a) = %v, %v", got, ok)
	}

	if err := s.Set("a", testValue("updated")); err != nil {
		t.Fatalf("update returned error: %v", err)
	}
	if got, ok := s.Get("a"); !ok || got != testValue("updated") {
		t.Fatalf("updated Get(a) = %v, %v", got, ok)
	}

	if !s.Delete("a") {
		t.Fatal("Delete(a) returned false")
	}
	if s.Delete("missing") {
		t.Fatal("Delete(missing) returned true")
	}
	if _, ok := s.Get("a"); ok {
		t.Fatal("deleted key returned a hit")
	}
	if len(evicted) == 0 || evicted[len(evicted)-1] != "a" {
		t.Fatalf("expected delete eviction for a, got %v", evicted)
	}
}

func TestLRU2StorePromotionAndEviction(t *testing.T) {
	s := newLRU2Cache(Options{BucketCount: 1, CapPerBucket: 2, Level2Cap: 2, CleanupInterval: time.Hour})
	defer s.Close()

	for _, key := range []string{"a", "b"} {
		if err := s.Set(key, testValue("value-"+key)); err != nil {
			t.Fatalf("Set(%s) returned error: %v", key, err)
		}
		if _, ok := s.Get(key); !ok {
			t.Fatalf("Get(%s) should promote key to level 2", key)
		}
	}
	if err := s.Set("c", testValue("value-c")); err != nil {
		t.Fatalf("Set(c) returned error: %v", err)
	}
	if err := s.Set("d", testValue("value-d")); err != nil {
		t.Fatalf("Set(d) returned error: %v", err)
	}
	if _, ok := s.Get("a"); !ok {
		t.Fatal("promoted key a should still be in level 2")
	}
	if _, ok := s.Get("c"); !ok {
		t.Fatal("newer level 1 key c should still exist")
	}
}

func TestLRU2StoreExpirationCleanupAndClear(t *testing.T) {
	s := newLRU2Cache(Options{BucketCount: 2, CapPerBucket: 4, Level2Cap: 4, CleanupInterval: 10 * time.Millisecond})
	defer s.Close()

	if err := s.SetWithExpiration("soon", testValue("value"), 20*time.Millisecond); err != nil {
		t.Fatalf("SetWithExpiration returned error: %v", err)
	}
	if err := s.SetWithExpiration("later", testValue("value"), time.Hour); err != nil {
		t.Fatalf("SetWithExpiration returned error: %v", err)
	}
	if _, ok := s.Get("soon"); !ok {
		t.Fatal("soon should be found before expiration")
	}
	time.Sleep(200 * time.Millisecond)
	if _, ok := s.Get("soon"); ok {
		t.Fatal("soon should have expired")
	}
	if _, ok := s.Get("later"); !ok {
		t.Fatal("later should still be valid")
	}

	for i := 0; i < 5; i++ {
		if err := s.Set(fmt.Sprintf("key-%d", i), testValue("value")); err != nil {
			t.Fatalf("Set returned error: %v", err)
		}
	}
	if s.Len() == 0 {
		t.Fatal("Len should be positive before Clear")
	}
	s.Clear()
	if s.Len() != 0 {
		t.Fatalf("Len after Clear = %d, want 0", s.Len())
	}
	s.Close()
	s.Close()
}

func TestLRU2Helpers(t *testing.T) {
	if maskOfNextPowOf2(1) != 0 || maskOfNextPowOf2(2) != 1 || maskOfNextPowOf2(3) != 3 {
		t.Fatalf("unexpected masks: 1=%d 2=%d 3=%d", maskOfNextPowOf2(1), maskOfNextPowOf2(2), maskOfNextPowOf2(3))
	}
	if hashBKRD("key") == hashBKRD("other") {
		t.Fatal("different keys should not collide in this smoke test")
	}

	c := Create(2)
	c.put("a", testValue("alpha"), Now()+int64(time.Hour), nil)
	c.put("b", testValue("beta"), Now()+int64(time.Hour), nil)
	var walked []string
	c.walk(func(key string, value Value, expireAt int64) bool {
		walked = append(walked, key)
		return len(walked) < 1
	})
	if len(walked) != 1 {
		t.Fatalf("walked %d entries, want 1", len(walked))
	}
}

func TestLRU2StoreNoExpirationPersists(t *testing.T) {
	s := newLRU2Cache(Options{BucketCount: 1, CapPerBucket: 4, Level2Cap: 4, CleanupInterval: 10 * time.Millisecond})
	defer s.Close()

	if err := s.Set("permanent", testValue("value")); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if got, ok := s.Get("permanent"); !ok || got != testValue("value") {
		t.Fatalf("permanent key should not expire, got (%v, %v)", got, ok)
	}
}

func TestLRU2StoreConcurrentAccess(t *testing.T) {
	s := newLRU2Cache(Options{BucketCount: 8, CapPerBucket: 64, Level2Cap: 64, CleanupInterval: time.Hour})
	defer s.Close()

	const goroutines = 8
	const operations = 100
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < operations; i++ {
				key := fmt.Sprintf("g%d-key-%d", id, i)
				if err := s.Set(key, testValue("value")); err != nil {
					t.Errorf("Set returned error: %v", err)
				}
				_, _ = s.Get(key)
				if i%2 == 0 {
					s.Delete(key)
				}
			}
		}(g)
	}
	wg.Wait()
}
