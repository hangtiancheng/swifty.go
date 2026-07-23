package example

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hangtiancheng/swifty.go/demo/tcc_demo"
	expdao "github.com/hangtiancheng/swifty.go/demo/tcc_demo/example/dao"
	"github.com/hangtiancheng/swifty.go/demo/tcc_demo/example/pkg"

	"github.com/demdxx/gocast"
	"github.com/hangtiancheng/swifty.go/demo/redis_lock"
)

type MockTXStore struct {
	client *redis_lock.Client
	dao    TXRecordDAO
}

func NewMockTXStore(dao TXRecordDAO, client *redis_lock.Client) *MockTXStore {
	return &MockTXStore{
		dao:    dao,
		client: client,
	}
}

func (m *MockTXStore) CreateTX(ctx context.Context, components ...tcc_demo.TCCComponent) (string, error) {
	// 创建一项内容，里面以唯一事务 id 为 key
	componentTryStatuses := make(map[string]*expdao.ComponentTryStatus, len(components))
	for _, component := range components {
		componentTryStatuses[component.ID()] = &expdao.ComponentTryStatus{
			ComponentID: component.ID(),
			TryStatus:   tcc_demo.TryHanging.String(),
		}
	}

	statusesBody, _ := json.Marshal(componentTryStatuses)
	txID, err := m.dao.CreateTXRecord(ctx, &expdao.TXRecordPO{
		Status:               tcc_demo.TXHanging.String(),
		ComponentTryStatuses: string(statusesBody),
	})
	if err != nil {
		return "", err
	}

	return gocast.ToString(txID), nil
}

func (m *MockTXStore) TXUpdate(ctx context.Context, txID string, componentID string, accept bool) error {
	_txID := gocast.ToUint(txID)
	status := tcc_demo.TXFailure.String()
	if accept {
		status = tcc_demo.TXSuccessful.String()
	}
	return m.dao.UpdateComponentStatus(ctx, _txID, componentID, status)
}

func (m *MockTXStore) GetHangingTXs(ctx context.Context) ([]*tcc_demo.Transaction, error) {
	records, err := m.dao.GetTXRecords(ctx, expdao.WithStatus(tcc_demo.TryHanging))
	if err != nil {
		return nil, err
	}

	txs := make([]*tcc_demo.Transaction, 0, len(records))
	for _, record := range records {
		componentTryStatuses := make(map[string]*expdao.ComponentTryStatus)
		_ = json.Unmarshal([]byte(record.ComponentTryStatuses), &componentTryStatuses)
		components := make([]*tcc_demo.ComponentTryEntity, 0, len(componentTryStatuses))
		for _, component := range componentTryStatuses {
			components = append(components, &tcc_demo.ComponentTryEntity{
				ComponentID: component.ComponentID,
				TryStatus:   tcc_demo.ComponentTryStatus(component.TryStatus),
			})
		}

		txs = append(txs, &tcc_demo.Transaction{
			TXID:       gocast.ToString(record.ID),
			Status:     tcc_demo.TXHanging,
			CreatedAt:  record.CreatedAt,
			Components: components,
		})
	}

	return txs, nil
}

func (m *MockTXStore) Lock(ctx context.Context, expireDuration time.Duration) error {
	lock := redis_lock.NewRedisLock(pkg.BuildTXRecordLockKey(), m.client, redis_lock.WithExpireSeconds(int64(expireDuration.Seconds())))
	return lock.Lock(ctx)
}

func (m *MockTXStore) Unlock(ctx context.Context) error {
	lock := redis_lock.NewRedisLock(pkg.BuildTXRecordLockKey(), m.client)
	return lock.Unlock(ctx)
}

// 提交事务的最终状态
func (m *MockTXStore) TXSubmit(ctx context.Context, txID string, success bool) error {
	do := func(ctx context.Context, dao *expdao.TXRecordDAO, record *expdao.TXRecordPO) error {
		if success {
			if record.Status == tcc_demo.TXFailure.String() {
				return fmt.Errorf("invalid tx status: %s, txid: %s", record.Status, txID)
			}
			record.Status = tcc_demo.TXSuccessful.String()
		} else {
			if record.Status == tcc_demo.TXSuccessful.String() {
				return fmt.Errorf("invalid tx status: %s, txid: %s", record.Status, txID)
			}
			record.Status = tcc_demo.TXFailure.String()
		}
		return dao.UpdateTXRecord(ctx, record)
	}
	return m.dao.LockAndDo(ctx, gocast.ToUint(txID), do)
}

// 获取指定的一笔事务
func (m *MockTXStore) GetTX(ctx context.Context, txID string) (*tcc_demo.Transaction, error) {
	records, err := m.dao.GetTXRecords(ctx, expdao.WithID(gocast.ToUint(txID)))
	if err != nil {
		return nil, err
	}
	if len(records) != 1 {
		return nil, errors.New("get tx failed")
	}

	componentTryStatuses := make(map[string]*expdao.ComponentTryStatus)
	_ = json.Unmarshal([]byte(records[0].ComponentTryStatuses), &componentTryStatuses)

	components := make([]*tcc_demo.ComponentTryEntity, 0, len(componentTryStatuses))
	for _, tryItem := range componentTryStatuses {
		components = append(components, &tcc_demo.ComponentTryEntity{
			ComponentID: tryItem.ComponentID,
			TryStatus:   tcc_demo.ComponentTryStatus(tryItem.TryStatus),
		})
	}
	return &tcc_demo.Transaction{
		TXID:       txID,
		Status:     tcc_demo.TXStatus(records[0].Status),
		Components: components,
		CreatedAt:  records[0].CreatedAt,
	}, nil
}

type TXRecordDAO interface {
	GetTXRecords(ctx context.Context, opts ...expdao.QueryOption) ([]*expdao.TXRecordPO, error)
	CreateTXRecord(ctx context.Context, record *expdao.TXRecordPO) (uint, error)
	UpdateComponentStatus(ctx context.Context, id uint, componentID string, status string) error
	UpdateTXRecord(ctx context.Context, record *expdao.TXRecordPO) error
	LockAndDo(ctx context.Context, id uint, do func(ctx context.Context, dao *expdao.TXRecordDAO, record *expdao.TXRecordPO) error) error
}
