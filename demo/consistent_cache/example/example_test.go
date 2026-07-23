package example

import (
	"context"
	"errors"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/hangtiancheng/swifty.go/demo/consistent_cache"
	"github.com/hangtiancheng/swifty.go/demo/consistent_cache/mysql"
	"github.com/hangtiancheng/swifty.go/demo/consistent_cache/redis"
)

const (
	redisAddress  = "please fill in redis address"
	redisPassword = "please fill in redis password"

	mysqlDSN = "please fill in mysql dsn"
)

func newService() *consistent_cache.Service {
	cache := redis.NewRedisCache(&redis.Config{
		Address:  redisAddress,
		Password: redisPassword,
	})
	db := mysql.NewDB(mysqlDSN)
	return consistent_cache.NewService(cache, db,
		consistent_cache.WithCacheExpireSeconds(120),
		consistent_cache.WithDisableExpireSeconds(1),
	)
}

func Test_consistent_Cache(t *testing.T) {
	service := consistent_cache.NewService(
		redis.NewRedisCache(&redis.Config{
			Address:  redisAddress,
			Password: redisPassword,
		}),
		mysql.NewDB(mysqlDSN),
		consistent_cache.WithCacheExpireSeconds(120),
		consistent_cache.WithCacheExpireRandomMode(),
		consistent_cache.WithDisableExpireSeconds(1),
	)
	ctx := context.Background()
	exp := Example{
		Key_: "test",
		Data: "test",
	}
	if err := service.Put(ctx, &exp); err != nil {
		t.Error(err)
		return
	}

	expReceiver := Example{
		Key_: "test",
	}
	if _, err := service.Get(ctx, &expReceiver); err != nil {
		t.Error(err)
		return
	}

	t.Logf("read data: %s, ", expReceiver.Data)
}

// Verifies: 1) data correctness 2) cache hit ratio.
func Test_Consistent_Cache_Correct(t *testing.T) {
	service := newService()
	ctx := context.Background()
	rander := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 100 concurrent writers, with a local backup of every written record.
	prefix := time.Now().String() + "-"
	datac := make(chan *Example)
	go func() {
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				k := prefix + strconv.Itoa(rander.Intn(100))
				v := prefix + strconv.Itoa(rander.Intn(100))
				data := Example{
					Key_: k,
					Data: v,
				}
				if err := service.Put(ctx, &data); err != nil {
					t.Error(err)
					return
				}
				datac <- &data
			}()
		}
		wg.Wait()
		close(datac)
	}()

	// Collect written data into a local backup map.
	mp := make(map[string]string, 500)
	for data := range datac {
		mp[data.Key_] = data.Data
	}

	// Wait for the write-path disable markers to expire.
	<-time.After(time.Second)

	var useCacheCnt int
	var expectUseCacheCnt int
	querySet := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		k := strconv.Itoa(rander.Intn(100))
		data := Example{
			Key_: prefix + k,
		}
		if _, ok := querySet[prefix+k]; ok {
			expectUseCacheCnt++
		}
		querySet[prefix+k] = struct{}{}

		useCache, err := service.Get(ctx, &data)
		if err != nil && !errors.Is(err, consistent_cache.ErrorDataNotExist) {
			t.Error(err)
			continue
		}

		if useCache {
			useCacheCnt++
		}

		expect, ok := mp[data.Key_]
		if errors.Is(err, consistent_cache.ErrorDataNotExist) != !ok {
			t.Errorf("expected data-not-exist=%v, got %v", !ok, errors.Is(err, consistent_cache.ErrorDataNotExist))
		}
		if !ok {
			continue
		}

		if data.Data != expect {
			t.Errorf("expected data=%s, got %s", expect, data.Data)
		}
	}

	if useCacheCnt != expectUseCacheCnt {
		t.Errorf("expected useCacheCnt=%d, got %d", expectUseCacheCnt, useCacheCnt)
	}
}

// Concurrent read/write. Verifies: 1) disable mechanism works 2) read result correctness.
func Test_Consistent_Cache_Read_Write(t *testing.T) {
	service := newService()

	ctx := context.Background()

	prefix := time.Now().String()

	var wg sync.WaitGroup
	datac := make(chan *Example)

	// Value range for writers.
	startV, endV := 1, 5
	// Multiple writers target the same key with values in [startV, endV].
	go func() {
		for i := startV; i <= endV; i++ {
			i := i // shadow
			wg.Add(1)
			go func() {
				defer wg.Done()
				k := prefix
				v := prefix + strconv.Itoa(i)
				data := Example{
					Key_: k,
					Data: v,
				}
				if err := service.Put(ctx, &data); err != nil {
					t.Error(err)
				}
				datac <- &data
			}()
		}
	}()

	// Double the readers targeting the same key.
	go func() {
		for i := 0; i < 10*(endV-startV+1); i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				data := Example{
					Key_: prefix,
				}
				useCache, err := service.Get(ctx, &data)
				if err != nil && !errors.Is(err, consistent_cache.ErrorDataNotExist) {
					t.Error(err)
					return
				}
				if errors.Is(err, consistent_cache.ErrorDataNotExist) {
					return
				}
				// During concurrent writes the cache is not expected to be used.
				if useCache {
					t.Errorf("expected useCache=false, got true")
				}
				gotData, err := strconv.Atoi(data.Data)
				if err != nil {
					t.Error(err)
					return
				}
				if gotData < startV || gotData > endV {
					t.Errorf("expected gotData in [%d, %d], got %d", startV, endV, gotData)
				}
			}()
		}
	}()

	// Collect the written records.
	datas := make([]*Example, 0, 5)
	for i := startV; i <= endV; i++ {
		data := <-datac
		datas = append(datas, data)
	}

	// After writes settle, read the final value.
	data := Example{
		Key_: prefix,
	}
	useCache, err := service.Get(ctx, &data)
	if err != nil {
		t.Error(err)
		return
	}
	if useCache {
		t.Errorf("expected useCache=false, got true")
	}
	if data.Data != datas[len(datas)-1].Data {
		t.Errorf("expected data=%s, got %s", datas[len(datas)-1].Data, data.Data)
	}

	wg.Wait()

	// One second later, read twice: first miss, second hit.
	<-time.After(time.Second)
	if useCache, err = service.Get(ctx, &data); err != nil {
		t.Error(err)
		return
	}
	if useCache {
		t.Errorf("expected useCache=false, got true")
	}

	if useCache, err = service.Get(ctx, &data); err != nil {
		t.Error(err)
		return
	}
	if !useCache {
		t.Errorf("expected useCache=true, got false")
	}
	if data.Data != datas[len(datas)-1].Data {
		t.Errorf("expected data=%s, got %s", datas[len(datas)-1].Data, data.Data)
	}
}
