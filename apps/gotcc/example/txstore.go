package example

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/gotcc"
	expdao "github.com/hangtiancheng/swifty.go/apps/gotcc/example/dao"
	"github.com/hangtiancheng/swifty.go/apps/gotcc/example/pkg"
	"github.com/hangtiancheng/swifty.go/apps/redis_lock"
)

// MockTXStore is a MySQL backed gotcc.TXStore, with a Redis distributed
// lock guarding the monitor task.
type MockTXStore struct {
	client pkg.RedisClient
	dao    TXRecordDAO

	// mux guards monitorLock.
	mux sync.Mutex
	// monitorLock is the distributed lock instance shared by Lock and
	// Unlock so that both operate on the same token and watchdog.
	monitorLock *redis_lock.RedisLock
}

// NewMockTXStore builds a TXStore on top of the given DAO and Redis client.
func NewMockTXStore(dao TXRecordDAO, client pkg.RedisClient) *MockTXStore {
	return &MockTXStore{
		dao:    dao,
		client: client,
	}
}

// CreateTX creates a transaction record with one hanging try status per
// component and returns the new transaction id.
func (m *MockTXStore) CreateTX(ctx context.Context, components ...gotcc.TCCComponent) (string, error) {
	componentTryStatuses := make(map[string]*expdao.ComponentTryStatus, len(components))
	for _, component := range components {
		componentTryStatuses[component.ID()] = &expdao.ComponentTryStatus{
			ComponentID: component.ID(),
			TryStatus:   gotcc.TryHanging.String(),
		}
	}

	statusesBody, _ := json.Marshal(componentTryStatuses)
	txID, err := m.dao.CreateTXRecord(ctx, &expdao.TXRecordPO{
		Status:               gotcc.TXHanging.String(),
		ComponentTryStatuses: string(statusesBody),
	})
	if err != nil {
		return "", err
	}

	return strconv.FormatUint(uint64(txID), 10), nil
}

// TXUpdate records the try-phase response of one component.
func (m *MockTXStore) TXUpdate(ctx context.Context, txID string, componentID string, accept bool) error {
	id, err := strconv.ParseUint(txID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid txid: %s, err: %w", txID, err)
	}
	status := gotcc.TXFailure.String()
	if accept {
		status = gotcc.TXSuccessful.String()
	}
	return m.dao.UpdateComponentStatus(ctx, uint(id), componentID, status)
}

// GetHangingTXs returns every transaction that has not finished yet.
func (m *MockTXStore) GetHangingTXs(ctx context.Context) ([]*gotcc.Transaction, error) {
	records, err := m.dao.GetTXRecords(ctx, expdao.WithStatus(gotcc.TXHanging))
	if err != nil {
		return nil, err
	}

	txs := make([]*gotcc.Transaction, 0, len(records))
	for _, record := range records {
		componentTryStatuses, err := expdao.ParseComponentTryStatuses(record.ComponentTryStatuses)
		if err != nil {
			return nil, err
		}
		components := make([]*gotcc.ComponentTryEntity, 0, len(componentTryStatuses))
		for _, component := range componentTryStatuses {
			components = append(components, &gotcc.ComponentTryEntity{
				ComponentID: component.ComponentID,
				TryStatus:   gotcc.ComponentTryStatus(component.TryStatus),
			})
		}

		txs = append(txs, &gotcc.Transaction{
			TXID:       strconv.FormatUint(uint64(record.ID), 10),
			Status:     gotcc.TXHanging,
			CreatedAt:  record.CreatedAt,
			Components: components,
		})
	}

	return txs, nil
}

// Lock acquires the distributed lock guarding the monitor task.
func (m *MockTXStore) Lock(ctx context.Context, expireDuration time.Duration) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	if m.monitorLock == nil {
		m.monitorLock = redis_lock.NewRedisLock(pkg.BuildTXRecordLockKey(), m.client, redis_lock.WithExpireSeconds(int64(expireDuration.Seconds())))
	}
	return m.monitorLock.Lock(ctx)
}

// Unlock releases the monitor task lock.
func (m *MockTXStore) Unlock(ctx context.Context) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	if m.monitorLock == nil {
		return nil
	}
	return m.monitorLock.Unlock(ctx)
}

// TXSubmit commits the final state of a transaction.
func (m *MockTXStore) TXSubmit(ctx context.Context, txID string, success bool) error {
	id, err := strconv.ParseUint(txID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid txid: %s, err: %w", txID, err)
	}

	do := func(ctx context.Context, dao expdao.TXRecordUpdater, record *expdao.TXRecordPO) error {
		if success {
			if record.Status == gotcc.TXFailure.String() {
				return fmt.Errorf("invalid tx status: %s, txid: %s", record.Status, txID)
			}
			record.Status = gotcc.TXSuccessful.String()
		} else {
			if record.Status == gotcc.TXSuccessful.String() {
				return fmt.Errorf("invalid tx status: %s, txid: %s", record.Status, txID)
			}
			record.Status = gotcc.TXFailure.String()
		}
		return dao.UpdateTXRecord(ctx, record)
	}
	return m.dao.LockAndDo(ctx, uint(id), do)
}

// GetTX returns a single transaction by id.
func (m *MockTXStore) GetTX(ctx context.Context, txID string) (*gotcc.Transaction, error) {
	id, err := strconv.ParseUint(txID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid txid: %s, err: %w", txID, err)
	}

	records, err := m.dao.GetTXRecords(ctx, expdao.WithID(uint(id)))
	if err != nil {
		return nil, err
	}
	if len(records) != 1 {
		return nil, errors.New("get tx failed")
	}

	componentTryStatuses, err := expdao.ParseComponentTryStatuses(records[0].ComponentTryStatuses)
	if err != nil {
		return nil, err
	}

	components := make([]*gotcc.ComponentTryEntity, 0, len(componentTryStatuses))
	for _, tryItem := range componentTryStatuses {
		components = append(components, &gotcc.ComponentTryEntity{
			ComponentID: tryItem.ComponentID,
			TryStatus:   gotcc.ComponentTryStatus(tryItem.TryStatus),
		})
	}
	return &gotcc.Transaction{
		TXID:       txID,
		Status:     gotcc.TXStatus(records[0].Status),
		Components: components,
		CreatedAt:  records[0].CreatedAt,
	}, nil
}

// TXRecordDAO abstracts the persistence of the transaction records so the
// store can be tested with an in-memory fake.
type TXRecordDAO interface {
	GetTXRecords(ctx context.Context, opts ...expdao.QueryOption) ([]*expdao.TXRecordPO, error)
	CreateTXRecord(ctx context.Context, record *expdao.TXRecordPO) (uint, error)
	UpdateComponentStatus(ctx context.Context, id uint, componentID string, status string) error
	UpdateTXRecord(ctx context.Context, record *expdao.TXRecordPO) error
	LockAndDo(ctx context.Context, id uint, do func(ctx context.Context, dao expdao.TXRecordUpdater, record *expdao.TXRecordPO) error) error
}
