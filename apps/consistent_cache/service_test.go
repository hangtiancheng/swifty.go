package consistent_cache

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

var errFake = errors.New("fake error")

// fakeCache is a Cache implementation that scripts results and records calls.
type fakeCache struct {
	mu sync.Mutex

	// scripted results, configured before the service is exercised.
	getValue     string
	getErr       error
	putEnableOK  bool
	putEnableErr error
	disableErr   error
	delErr       error
	enableErr    error

	// recorded calls.
	enableCalls        []enableCall
	disableCalls       []disableCall
	putWhenEnableCalls []putWhenEnableCall
	delKeys            []string

	// enabled receives one send per Enable call.
	enabled chan string
}

type enableCall struct {
	key        string
	delayMilis int64
}

type disableCall struct {
	key           string
	expireSeconds int64
}

type putWhenEnableCall struct {
	key           string
	value         string
	expireSeconds int64
}

func newFakeCache() *fakeCache {
	return &fakeCache{enabled: make(chan string, 16)}
}

func (c *fakeCache) Enable(_ context.Context, key string, delayMilis int64) error {
	c.mu.Lock()
	c.enableCalls = append(c.enableCalls, enableCall{key: key, delayMilis: delayMilis})
	c.mu.Unlock()
	c.enabled <- key
	return c.enableErr
}

func (c *fakeCache) Disable(_ context.Context, key string, expireSeconds int64) error {
	c.mu.Lock()
	c.disableCalls = append(c.disableCalls, disableCall{key: key, expireSeconds: expireSeconds})
	c.mu.Unlock()
	return c.disableErr
}

func (c *fakeCache) Get(_ context.Context, _ string) (string, error) {
	return c.getValue, c.getErr
}

func (c *fakeCache) Del(_ context.Context, key string) error {
	c.mu.Lock()
	c.delKeys = append(c.delKeys, key)
	c.mu.Unlock()
	return c.delErr
}

func (c *fakeCache) PutWhenEnable(_ context.Context, key, value string, expireSeconds int64) (bool, error) {
	c.mu.Lock()
	c.putWhenEnableCalls = append(c.putWhenEnableCalls, putWhenEnableCall{key: key, value: value, expireSeconds: expireSeconds})
	c.mu.Unlock()
	return c.putEnableOK, c.putEnableErr
}

// waitEnable waits for the next (delayed) Enable call of the service.
func (c *fakeCache) waitEnable(t *testing.T) {
	t.Helper()
	select {
	case <-c.enabled:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the delayed enable")
	}
}

// assertNoEnable verifies that no delayed Enable call arrives shortly after.
func (c *fakeCache) assertNoEnable(t *testing.T) {
	t.Helper()
	select {
	case key := <-c.enabled:
		t.Fatalf("unexpected delayed enable for key %q", key)
	case <-time.After(100 * time.Millisecond):
	}
}

// fakeDB is a DB implementation that scripts results and records calls.
type fakeDB struct {
	putErr  error
	getErr  error
	getBody string

	mu      sync.Mutex
	putObjs []Object
	gotGets []string
}

func (d *fakeDB) Put(_ context.Context, obj Object) error {
	d.mu.Lock()
	d.putObjs = append(d.putObjs, obj)
	d.mu.Unlock()
	return d.putErr
}

func (d *fakeDB) Get(_ context.Context, obj Object) error {
	d.mu.Lock()
	d.gotGets = append(d.gotGets, obj.Key())
	d.mu.Unlock()
	if d.getErr != nil {
		return d.getErr
	}
	return obj.Read(d.getBody)
}

// fakeObject is an Object implementation that records the deserialized body.
type fakeObject struct {
	key      string
	body     string
	readErr  error
	writeErr error
	gotRead  string
}

func (o *fakeObject) KeyColumn() string { return "key" }
func (o *fakeObject) Key() string       { return o.key }

func (o *fakeObject) Write() (string, error) {
	if o.writeErr != nil {
		return "", o.writeErr
	}
	return o.body, nil
}

func (o *fakeObject) Read(body string) error {
	o.gotRead = body
	return o.readErr
}

// TestPutWriteFlowOrderAndDelayedEnable verifies the write flow of Put: the
// cache is disabled, then deleted, then the database is written, and only
// after a successful write flow the mechanism is re-enabled with a delay.
func TestPutWriteFlowOrderAndDelayedEnable(t *testing.T) {
	cache := newFakeCache()
	db := &fakeDB{}
	s := NewService(cache, db,
		WithDisableExpireSeconds(7),
		WithEnableDelayMilis(1234),
		WithLogger(&testLogger{}),
	)
	obj := &fakeObject{key: "k1", body: "v1"}

	if err := s.Put(context.Background(), obj); err != nil {
		t.Fatalf("put: %v", err)
	}

	if got, want := len(cache.disableCalls), 1; got != want {
		t.Fatalf("disable calls = %d, want %d", got, want)
	}
	if got, want := cache.disableCalls[0], (disableCall{key: "k1", expireSeconds: 7}); got != want {
		t.Errorf("disable call = %+v, want %+v", got, want)
	}
	if got, want := cache.delKeys, []string{"k1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("deleted keys = %v, want %v", got, want)
	}
	if got, want := len(db.putObjs), 1; got != want {
		t.Fatalf("db puts = %d, want %d", got, want)
	}
	if db.putObjs[0] != Object(obj) {
		t.Errorf("db put object = %T, want the original object", db.putObjs[0])
	}

	// The re-enable happens asynchronously after the write flow succeeded.
	cache.waitEnable(t)
	if got, want := cache.enableCalls[0], (enableCall{key: "k1", delayMilis: 1234}); got != want {
		t.Errorf("enable call = %+v, want %+v", got, want)
	}
}

// TestPutDisableFailureAbortsWriteFlow verifies that a failed disable leaves
// the cache and the database untouched and never schedules the enable.
func TestPutDisableFailureAbortsWriteFlow(t *testing.T) {
	cache := newFakeCache()
	cache.disableErr = errFake
	db := &fakeDB{}
	s := NewService(cache, db, WithLogger(&testLogger{}))

	if err := s.Put(context.Background(), &fakeObject{key: "k1"}); !errors.Is(err, errFake) {
		t.Fatalf("put error = %v, want %v", err, errFake)
	}
	if got := len(cache.delKeys); got != 0 {
		t.Errorf("cache deletions = %d, want 0", got)
	}
	if got := len(db.putObjs); got != 0 {
		t.Errorf("db puts = %d, want 0", got)
	}
	cache.assertNoEnable(t)
}

// TestPutDelFailureSkipsDBWriteAndEnable verifies that a failed cache deletion
// aborts the write flow before the database write and never schedules the
// enable: otherwise the still cached stale value would become readable again
// while the write is incomplete.
func TestPutDelFailureSkipsDBWriteAndEnable(t *testing.T) {
	cache := newFakeCache()
	cache.delErr = errFake
	db := &fakeDB{}
	s := NewService(cache, db, WithLogger(&testLogger{}))

	if err := s.Put(context.Background(), &fakeObject{key: "k1"}); !errors.Is(err, errFake) {
		t.Fatalf("put error = %v, want %v", err, errFake)
	}
	if got := len(db.putObjs); got != 0 {
		t.Errorf("db puts = %d, want 0", got)
	}
	cache.assertNoEnable(t)
}

// TestPutDBFailureSkipsEnable verifies that a failed database write does not
// schedule the delayed enable.
func TestPutDBFailureSkipsEnable(t *testing.T) {
	cache := newFakeCache()
	db := &fakeDB{putErr: errFake}
	s := NewService(cache, db, WithLogger(&testLogger{}))

	if err := s.Put(context.Background(), &fakeObject{key: "k1"}); !errors.Is(err, errFake) {
		t.Fatalf("put error = %v, want %v", err, errFake)
	}
	if got, want := len(cache.delKeys), 1; got != want {
		t.Errorf("cache deletions = %d, want %d", got, want)
	}
	cache.assertNoEnable(t)
}

// TestGetServesValueFromCache verifies the cache hit path of Get.
func TestGetServesValueFromCache(t *testing.T) {
	cache := newFakeCache()
	cache.getValue = "cached-body"
	db := &fakeDB{}
	s := NewService(cache, db, WithLogger(&testLogger{}))
	obj := &fakeObject{key: "k1"}

	useCache, err := s.Get(context.Background(), obj)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !useCache {
		t.Error("useCache = false, want true")
	}
	if got, want := obj.gotRead, "cached-body"; got != want {
		t.Errorf("deserialized body = %q, want %q", got, want)
	}
	if got := len(db.gotGets); got != 0 {
		t.Errorf("db reads = %d, want 0", got)
	}
	if got := len(cache.putWhenEnableCalls); got != 0 {
		t.Errorf("cache writes = %d, want 0", got)
	}
}

// TestGetNullDataPlaceholder verifies that the cached NullData placeholder is
// reported as ErrorDataNotExist without deserializing it into the object.
func TestGetNullDataPlaceholder(t *testing.T) {
	cache := newFakeCache()
	cache.getValue = NullData
	db := &fakeDB{}
	s := NewService(cache, db, WithLogger(&testLogger{}))
	obj := &fakeObject{key: "k1"}

	useCache, err := s.Get(context.Background(), obj)
	if !errors.Is(err, ErrorDataNotExist) {
		t.Fatalf("get error = %v, want %v", err, ErrorDataNotExist)
	}
	if !useCache {
		t.Error("useCache = false, want true")
	}
	if got := obj.gotRead; got != "" {
		t.Errorf("the placeholder was deserialized into the object: %q", got)
	}
}

// TestGetCacheErrorPropagates verifies that a cache error other than a miss is
// propagated and the database is not read.
func TestGetCacheErrorPropagates(t *testing.T) {
	cache := newFakeCache()
	cache.getErr = errFake
	db := &fakeDB{}
	s := NewService(cache, db, WithLogger(&testLogger{}))

	useCache, err := s.Get(context.Background(), &fakeObject{key: "k1"})
	if !errors.Is(err, errFake) {
		t.Fatalf("get error = %v, want %v", err, errFake)
	}
	if useCache {
		t.Error("useCache = true, want false")
	}
	if got := len(db.gotGets); got != 0 {
		t.Errorf("db reads = %d, want 0", got)
	}
}

// TestGetCacheMissReadsDBAndFillsCache verifies the cache miss path: the data
// is read from the database, written into the cache with the configured expiry
// and returned to the caller.
func TestGetCacheMissReadsDBAndFillsCache(t *testing.T) {
	cache := newFakeCache()
	cache.getErr = ErrorCacheMiss
	db := &fakeDB{getBody: "db-body"}
	s := NewService(cache, db,
		WithCacheExpireSeconds(90),
		WithLogger(&testLogger{}),
	)
	obj := &fakeObject{key: "k1", body: "serialized"}

	useCache, err := s.Get(context.Background(), obj)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if useCache {
		t.Error("useCache = true, want false")
	}
	if got, want := obj.gotRead, "db-body"; got != want {
		t.Errorf("deserialized body = %q, want %q", got, want)
	}
	if got, want := len(cache.putWhenEnableCalls), 1; got != want {
		t.Fatalf("cache writes = %d, want %d", got, want)
	}
	if got, want := cache.putWhenEnableCalls[0], (putWhenEnableCall{key: "k1", value: "serialized", expireSeconds: 90}); got != want {
		t.Errorf("cache write = %+v, want %+v", got, want)
	}
}

// TestGetCacheMissRandomExpire verifies that the expiry handed to the cache
// write follows the random jitter mode.
func TestGetCacheMissRandomExpire(t *testing.T) {
	cache := newFakeCache()
	cache.getErr = ErrorCacheMiss
	db := &fakeDB{getBody: "db-body"}
	s := NewService(cache, db,
		WithCacheExpireSeconds(5),
		WithCacheExpireRandomMode(),
		WithLogger(&testLogger{}),
	)

	if _, err := s.Get(context.Background(), &fakeObject{key: "k1", body: "v"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got, want := len(cache.putWhenEnableCalls), 1; got != want {
		t.Fatalf("cache writes = %d, want %d", got, want)
	}
	if got := cache.putWhenEnableCalls[0].expireSeconds; got < 5 || got > 10 {
		t.Errorf("cache expiry = %d, want within [5, 10]", got)
	}
}

// TestGetCacheMissDBMissWritesNullData verifies the cache penetration
// countermeasure: a database miss caches the NullData placeholder and is
// reported as ErrorDataNotExist.
func TestGetCacheMissDBMissWritesNullData(t *testing.T) {
	cache := newFakeCache()
	cache.getErr = ErrorCacheMiss
	db := &fakeDB{getErr: ErrorDBMiss}
	s := NewService(cache, db, WithLogger(&testLogger{}))

	useCache, err := s.Get(context.Background(), &fakeObject{key: "k1"})
	if !errors.Is(err, ErrorDataNotExist) {
		t.Fatalf("get error = %v, want %v", err, ErrorDataNotExist)
	}
	if useCache {
		t.Error("useCache = true, want false")
	}
	if got, want := len(cache.putWhenEnableCalls), 1; got != want {
		t.Fatalf("cache writes = %d, want %d", got, want)
	}
	if got := cache.putWhenEnableCalls[0].value; got != NullData {
		t.Errorf("cached value = %q, want the NullData placeholder %q", got, NullData)
	}
}

// TestGetDBErrorPropagates verifies that a database error other than a miss is
// propagated and nothing is written into the cache.
func TestGetDBErrorPropagates(t *testing.T) {
	cache := newFakeCache()
	cache.getErr = ErrorCacheMiss
	db := &fakeDB{getErr: errFake}
	s := NewService(cache, db, WithLogger(&testLogger{}))

	useCache, err := s.Get(context.Background(), &fakeObject{key: "k1"})
	if !errors.Is(err, errFake) {
		t.Fatalf("get error = %v, want %v", err, errFake)
	}
	if useCache {
		t.Error("useCache = true, want false")
	}
	if got := len(cache.putWhenEnableCalls); got != 0 {
		t.Errorf("cache writes = %d, want 0", got)
	}
}

// TestGetCacheWriteErrorIsSwallowed verifies that a failed cache write does not
// turn a successful read into an error.
func TestGetCacheWriteErrorIsSwallowed(t *testing.T) {
	cache := newFakeCache()
	cache.getErr = ErrorCacheMiss
	cache.putEnableErr = errFake
	db := &fakeDB{getBody: "db-body"}
	s := NewService(cache, db, WithLogger(&testLogger{}))
	obj := &fakeObject{key: "k1", body: "serialized"}

	useCache, err := s.Get(context.Background(), obj)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if useCache {
		t.Error("useCache = true, want false")
	}
	if got, want := obj.gotRead, "db-body"; got != want {
		t.Errorf("deserialized body = %q, want %q", got, want)
	}
}
