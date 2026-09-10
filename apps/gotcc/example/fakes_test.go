package example_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/hangtiancheng/swifty.go/apps/gotcc"
	"github.com/hangtiancheng/swifty.go/apps/gotcc/example/dao"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// fakeRedis is an in-memory pkg.RedisClient fake with fault injection
// hooks. It also implements the redis_lock.LockClient operations with the
// same semantics as the Lua scripts (ownership checked release and renew),
// so the real redis_lock.RedisLock logic runs against it.
type fakeRedis struct {
	mu    sync.Mutex
	store map[string]string

	getErr     func(key string) error
	setErr     func(key, value string) error
	setNXErr   func(key, value string) error
	setNXReply map[string]int64
	delErr     func(key string) error
	setNEXErr  func(key string) error
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{
		store:      make(map[string]string),
		setNXReply: make(map[string]int64),
	}
}

func (f *fakeRedis) Get(ctx context.Context, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		if err := f.getErr(key); err != nil {
			return "", err
		}
	}
	value, ok := f.store[key]
	if !ok {
		return "", redis.Nil
	}
	return value, nil
}

func (f *fakeRedis) Set(ctx context.Context, key, value string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setErr != nil {
		if err := f.setErr(key, value); err != nil {
			return -1, err
		}
	}
	f.store[key] = value
	return 1, nil
}

func (f *fakeRedis) SetNX(ctx context.Context, key, value string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setNXErr != nil {
		if err := f.setNXErr(key, value); err != nil {
			return -1, err
		}
	}
	if reply, ok := f.setNXReply[key]; ok {
		if reply == 1 {
			f.store[key] = value
		}
		return reply, nil
	}
	if _, ok := f.store[key]; ok {
		return 0, nil
	}
	f.store[key] = value
	return 1, nil
}

func (f *fakeRedis) Del(ctx context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.delErr != nil {
		if err := f.delErr(key); err != nil {
			return err
		}
	}
	delete(f.store, key)
	return nil
}

func (f *fakeRedis) Ping(ctx context.Context) error {
	return nil
}

func (f *fakeRedis) SetNEX(ctx context.Context, key, value string, expireSeconds int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setNEXErr != nil {
		if err := f.setNEXErr(key); err != nil {
			return -1, err
		}
	}
	if _, ok := f.store[key]; ok {
		return 0, nil
	}
	f.store[key] = value
	return 1, nil
}

func (f *fakeRedis) Eval(ctx context.Context, src string, keyCount int, keysAndArgs []interface{}) (interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if keyCount < 1 || keyCount > len(keysAndArgs) {
		return nil, errors.New("fake redis: invalid eval call")
	}
	key, _ := keysAndArgs[0].(string)
	var token string
	if len(keysAndArgs) > keyCount {
		token, _ = keysAndArgs[keyCount].(string)
	}

	current, ok := f.store[key]
	switch {
	case strings.Contains(src, "'del'"):
		// Release semantics: delete only when the caller owns the lock.
		if ok && current == token {
			delete(f.store, key)
			return int64(1), nil
		}
		return int64(0), nil
	case strings.Contains(src, "'expire'"):
		// Renew semantics: extend only when the caller owns the lock.
		if ok && current == token {
			return int64(1), nil
		}
		return int64(0), nil
	default:
		return int64(0), nil
	}
}

// fakeUpdater is the write handle handed to LockAndDo callbacks by
// fakeTXRecordDAO.
type fakeUpdater struct {
	dao *fakeTXRecordDAO
}

func (u *fakeUpdater) UpdateTXRecord(ctx context.Context, record *dao.TXRecordPO) error {
	u.dao.mu.Lock()
	defer u.dao.mu.Unlock()
	stored, ok := u.dao.records[record.ID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	*stored = *record
	return nil
}

// fakeTXRecordDAO is an in-memory dao.TXRecordDAO fake. It ignores the
// query options (it always returns every record); the option handling of
// the real DAO is covered by the live-MySQL integration test.
type fakeTXRecordDAO struct {
	mu      sync.Mutex
	records map[uint]*dao.TXRecordPO
	nextID  uint
}

func newFakeTXRecordDAO() *fakeTXRecordDAO {
	return &fakeTXRecordDAO{
		records: make(map[uint]*dao.TXRecordPO),
		nextID:  1,
	}
}

func (f *fakeTXRecordDAO) GetTXRecords(ctx context.Context, opts ...dao.QueryOption) ([]*dao.TXRecordPO, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	records := make([]*dao.TXRecordPO, 0, len(f.records))
	for _, record := range f.records {
		cp := *record
		records = append(records, &cp)
	}
	return records, nil
}

func (f *fakeTXRecordDAO) CreateTXRecord(ctx context.Context, record *dao.TXRecordPO) (uint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.nextID
	f.nextID++
	record.ID = id
	cp := *record
	f.records[id] = &cp
	return id, nil
}

func (f *fakeTXRecordDAO) UpdateComponentStatus(ctx context.Context, id uint, componentID string, status string) error {
	return f.LockAndDo(ctx, id, func(ctx context.Context, updater dao.TXRecordUpdater, record *dao.TXRecordPO) error {
		statuses := make(map[string]*dao.ComponentTryStatus)
		if err := json.Unmarshal([]byte(record.ComponentTryStatuses), &statuses); err != nil {
			return err
		}
		componentStatus, ok := statuses[componentID]
		if !ok {
			return fmt.Errorf("invalid component: %s", componentID)
		}
		if componentStatus.TryStatus == status {
			return nil
		}
		if componentStatus.TryStatus != gotcc.TryHanging.String() {
			return fmt.Errorf("invalid status: %s of component: %s", componentStatus.TryStatus, componentID)
		}
		componentStatus.TryStatus = status
		body, err := json.Marshal(statuses)
		if err != nil {
			return err
		}
		record.ComponentTryStatuses = string(body)
		return updater.UpdateTXRecord(ctx, record)
	})
}

func (f *fakeTXRecordDAO) UpdateTXRecord(ctx context.Context, record *dao.TXRecordPO) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored, ok := f.records[record.ID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	*stored = *record
	return nil
}

func (f *fakeTXRecordDAO) LockAndDo(ctx context.Context, id uint, do func(ctx context.Context, dao dao.TXRecordUpdater, record *dao.TXRecordPO) error) error {
	f.mu.Lock()
	stored, ok := f.records[id]
	if !ok {
		f.mu.Unlock()
		return gorm.ErrRecordNotFound
	}
	cp := *stored
	f.mu.Unlock()

	updater := &fakeUpdater{dao: f}
	return do(ctx, updater, &cp)
}
