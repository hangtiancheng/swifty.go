// Package example_test contains integration tests for the consistent_cache
// service. It is an external test package: the sample Object implementation
// lives in the example package and is imported from here.
//
// The tests require a live MySQL and Redis instance. They attempt a quick TCP
// connection first and are skipped when either service is unreachable. The
// target addresses can be overridden with the environment variables
// CONSISTENT_CACHE_TEST_REDIS_ADDR, CONSISTENT_CACHE_TEST_REDIS_PASSWORD and
// CONSISTENT_CACHE_TEST_MYSQL_DSN.
package example_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/hangtiancheng/swifty.go/apps/consistent_cache"
	"github.com/hangtiancheng/swifty.go/apps/consistent_cache/example"
	"github.com/hangtiancheng/swifty.go/apps/consistent_cache/internal/mysql"
	"github.com/hangtiancheng/swifty.go/apps/consistent_cache/internal/redis"
)

const defaultMySQLDSN = "root@tcp(127.0.0.1:3306)/consistent_cache?charset=utf8mb4&parseTime=True&loc=Local"

func redisAddr() string {
	if v := os.Getenv("CONSISTENT_CACHE_TEST_REDIS_ADDR"); v != "" {
		return v
	}
	return "127.0.0.1:6379"
}

func redisPassword() string {
	return os.Getenv("CONSISTENT_CACHE_TEST_REDIS_PASSWORD")
}

func mysqlDSN() string {
	if v := os.Getenv("CONSISTENT_CACHE_TEST_MYSQL_DSN"); v != "" {
		return v
	}
	return defaultMySQLDSN
}

// dialTCP performs a quick TCP dial to check that a service is reachable.
func dialTCP(t *testing.T, addr string) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Skipf("skipping test: service at %s is unreachable: %v", addr, err)
	}
	_ = conn.Close()
}

// pingRedis performs a quick PING to check that Redis is reachable and usable.
func pingRedis(t *testing.T) {
	t.Helper()
	client := goredis.NewClient(&goredis.Options{
		Addr:     redisAddr(),
		Password: redisPassword(),
	})
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("skipping test: redis at %s is unreachable: %v", redisAddr(), err)
	}
}

// mysqlAddr extracts the host:port from a DSN of the form
// user:password@tcp(host:port)/database.
func mysqlAddr(dsn string) (string, error) {
	const marker = "tcp("
	i := strings.Index(dsn, marker)
	if i < 0 {
		return "", fmt.Errorf("invalid DSN %q: missing tcp(...) address", dsn)
	}
	rest := dsn[i+len(marker):]
	j := strings.Index(rest, ")")
	if j < 0 {
		return "", fmt.Errorf("invalid DSN %q: unterminated tcp(...) address", dsn)
	}
	return rest[:j], nil
}

// requireServices skips the test when Redis or MySQL cannot be reached.
func requireServices(t *testing.T) {
	t.Helper()
	pingRedis(t)
	addr, err := mysqlAddr(mysqlDSN())
	if err != nil {
		t.Fatalf("parse mysql DSN: %v", err)
	}
	dialTCP(t, addr)
}

// newMySQLDB connects to MySQL, skipping the test when the database is not usable.
func newMySQLDB(t *testing.T) *mysql.DB {
	t.Helper()
	db, err := mysql.NewDB(mysqlDSN())
	if err != nil {
		t.Skipf("skipping test: mysql at %s is not usable: %v", mysqlDSN(), err)
	}
	return db
}

func newService(t *testing.T) *consistent_cache.Service {
	t.Helper()
	requireServices(t)

	// Cache module.
	cache := redis.NewRedisCache(&redis.Config{
		Address:  redisAddr(),
		Password: redisPassword(),
	})
	// Database module.
	db := newMySQLDB(t)
	return consistent_cache.NewService(cache, db,
		consistent_cache.WithCacheExpireSeconds(120),
		consistent_cache.WithDisableExpireSeconds(1),
	)
}

func TestConsistentCache(t *testing.T) {
	requireServices(t)
	service := consistent_cache.NewService(
		// Cache module.
		redis.NewRedisCache(&redis.Config{
			// Redis address.
			Address: redisAddr(),
			// Redis password.
			Password: redisPassword(),
		}),
		// Database module.
		newMySQLDB(t),
		// Cache expiry time of 120s.
		consistent_cache.WithCacheExpireSeconds(120),
		// Random jitter on the cache expiry time to prevent a cache avalanche.
		consistent_cache.WithCacheExpireRandomMode(),
		// The write-cache disable mark is re-enabled after a 1s delay.
		consistent_cache.WithDisableExpireSeconds(1),
	)
	ctx := context.Background()
	exp := example.Example{
		Key_: "test",
		Data: "test",
	}
	// Write operation.
	if err := service.Put(ctx, &exp); err != nil {
		t.Fatalf("put: %v", err)
	}

	// Read operation.
	expReceiver := example.Example{
		Key_: "test",
	}
	if _, err := service.Get(ctx, &expReceiver); err != nil {
		t.Fatalf("get: %v", err)
	}

	// The data that was read, and whether the cache was used.
	t.Logf("read data: %s", expReceiver.Data)
}

// TestConsistentCacheCorrectness verifies: 1. data correctness, 2. the cache hit rate.
func TestConsistentCacheCorrectness(t *testing.T) {
	// Build the cache consistency service instance.
	service := newService(t)
	// Context.
	ctx := context.Background()

	// Spawn 100 goroutines that write concurrently while keeping a local backup
	// of every written value.
	// Common data prefix.
	prefix := time.Now().String() + "-"
	// This channel receives the data submitted by the writer goroutines so the
	// local backup can be built.
	datac := make(chan *example.Example)
	// Asynchronously spawn 100 goroutines that write concurrently.
	go func() {
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				k := prefix + strconv.Itoa(rand.IntN(100))
				v := prefix + strconv.Itoa(rand.IntN(100))
				data := example.Example{
					Key_: k,
					Data: v,
				}
				// Perform the write through the consistency service.
				if err := service.Put(ctx, &data); err != nil {
					t.Errorf("put: %v", err)
					return
				}
				// After a successful write, send the data through the channel
				// so the reader goroutine can back it up locally.
				datac <- &data
			}()
		}
		wg.Wait()
		close(datac)
	}()

	// Receive the data submitted by the writer goroutines through the channel
	// and back up all written values per key locally.
	values := make(map[string]map[string]struct{}, 100)
	for data := range datac {
		if values[data.Key_] == nil {
			values[data.Key_] = make(map[string]struct{})
		}
		values[data.Key_][data.Data] = struct{}{}
	}

	// Wait one second so that the disable marks of the write operations expire.
	<-time.After(time.Second)

	// Number of reads served from the cache.
	var useCacheCnt int
	// Expected number of reads served from the cache.
	var expectUseCacheCnt int
	querySet := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		k := prefix + strconv.Itoa(rand.IntN(100))
		data := example.Example{
			Key_: k,
		}
		if _, ok := querySet[k]; ok {
			expectUseCacheCnt++
		}
		querySet[k] = struct{}{}

		// Perform the read through the consistency cache service.
		useCache, err := service.Get(ctx, &data)
		if err != nil && !errors.Is(err, consistent_cache.ErrorDataNotExist) {
			t.Errorf("get: %v", err)
			continue
		}

		if useCache {
			useCacheCnt++
		}

		// Compare the read result against the locally backed up values.
		written, ok := values[data.Key_]
		notExist := errors.Is(err, consistent_cache.ErrorDataNotExist)
		if ok == notExist {
			t.Errorf("key %q: data existence mismatch: written locally=%t, data not exist=%t", data.Key_, ok, notExist)
		}
		if !ok {
			continue
		}

		if _, seen := written[data.Data]; !seen {
			t.Errorf("key %q: read data %q was never written", data.Key_, data.Data)
		}
	}

	// Verify that the number of cache hits matches the expectation.
	if useCacheCnt != expectUseCacheCnt {
		t.Errorf("cache hit count: got %d, want %d", useCacheCnt, expectUseCacheCnt)
	}
}

// TestConsistentCacheReadWrite runs reads and writes concurrently. It
// verifies: 1. the disable mechanism works as expected, 2. the read results are correct.
func TestConsistentCacheReadWrite(t *testing.T) {
	// Build the cache consistency service instance.
	service := newService(t)

	ctx := context.Background()

	// Common data prefix.
	prefix := time.Now().String()

	// Concurrency control and data passing between goroutines.
	var wg sync.WaitGroup
	datac := make(chan *example.Example)

	// Range of the values that are written.
	startV, endV := 1, 5
	// Register all writers and readers up front so the WaitGroup counter can
	// never reach zero prematurely.
	wg.Add((endV - startV + 1) + 10*(endV-startV+1))

	// Spawn several goroutines that write the same key with values taken from
	// the range [startV, endV].
	go func() {
		for i := startV; i <= endV; i++ {
			go func(i int) {
				defer wg.Done()
				k := prefix
				v := prefix + strconv.Itoa(i)
				data := example.Example{
					Key_: k,
					Data: v,
				}
				// Perform the write through the consistency service.
				if err := service.Put(ctx, &data); err != nil {
					t.Errorf("put: %v", err)
					return
				}
				// After a successful write, send the data through the channel
				// so it can be backed up locally.
				datac <- &data
			}(i)
		}
	}()

	// Spawn twice as many reader goroutines that read the same key.
	go func() {
		for i := 0; i < 10*(endV-startV+1); i++ {
			go func() {
				defer wg.Done()
				data := example.Example{
					Key_: prefix,
				}
				// Perform the read through the consistency service.
				useCache, err := service.Get(ctx, &data)
				if err != nil && !errors.Is(err, consistent_cache.ErrorDataNotExist) {
					t.Errorf("get: %v", err)
					return
				}
				if errors.Is(err, consistent_cache.ErrorDataNotExist) {
					return
				}
				// The cache is not expected to be used while the write-flow
				// disable mark is active.
				if useCache {
					t.Error("read during the disable window unexpectedly used the cache")
				}
				// The data is expected to be one of the written values
				// in [startV, endV].
				suffix, ok := strings.CutPrefix(data.Data, prefix)
				if !ok {
					t.Errorf("read data %q does not carry the test prefix", data.Data)
					return
				}
				gotData, err := strconv.Atoi(suffix)
				if err != nil || gotData < startV || gotData > endV {
					t.Errorf("read data %q is not one of the written values in [%d,%d]", data.Data, startV, endV)
				}
			}()
		}
	}()

	// Receive the data submitted by the writer goroutines through the channel
	// and back it up locally.
	datas := make([]*example.Example, 0, endV-startV+1)
	for i := startV; i <= endV; i++ {
		data := <-datac
		datas = append(datas, data)
	}

	// Once everything has settled, read the final correct result.
	data := example.Example{
		Key_: prefix,
	}
	useCache, err := service.Get(ctx, &data)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	// The cache is not expected to be used yet.
	if useCache {
		t.Error("first read after the writes unexpectedly used the cache")
	}
	// The result is expected to be the value of the last write.
	if data.Data != datas[len(datas)-1].Data {
		t.Errorf("read data %q, want the last written value %q", data.Data, datas[len(datas)-1].Data)
	}

	wg.Wait()

	// After one second, read twice: the first read misses the cache and the
	// second one hits it.
	<-time.After(time.Second)
	if useCache, err = service.Get(ctx, &data); err != nil {
		t.Fatalf("get: %v", err)
	}
	// The cache is not expected to be used on the first read.
	if useCache {
		t.Error("first read after the delay unexpectedly used the cache")
	}

	if useCache, err = service.Get(ctx, &data); err != nil {
		t.Fatalf("get: %v", err)
	}
	// The second read is expected to be served from the cache.
	if !useCache {
		t.Error("second read unexpectedly did not use the cache")
	}
	// The result should still equal the value of the latest write.
	if data.Data != datas[len(datas)-1].Data {
		t.Errorf("read data %q, want the last written value %q", data.Data, datas[len(datas)-1].Data)
	}
}
