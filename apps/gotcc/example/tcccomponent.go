// Package example shows how to wire a full TCC flow on top of gotcc:
// Redis backed TCC components plus a MySQL backed transaction-log store.
package example

import (
	"context"
	"errors"
	"fmt"

	"github.com/hangtiancheng/swifty.go/apps/gotcc"
	"github.com/hangtiancheng/swifty.go/apps/gotcc/example/pkg"
	"github.com/hangtiancheng/swifty.go/apps/redis_lock"
	"github.com/redis/go-redis/v9"
)

// TXStatus is the state of one transaction as seen by the component side.
type TXStatus string

// String implements fmt.Stringer.
func (t TXStatus) String() string {
	return string(t)
}

const (
	// TXTried means the try operation has been executed.
	TXTried TXStatus = "tried"
	// TXConfirmed means the confirm operation has been executed.
	TXConfirmed TXStatus = "confirmed"
	// TXCanceled means the cancel operation has been executed.
	TXCanceled TXStatus = "canceled"
)

// DataStatus is the state of the business data touched by a transaction.
type DataStatus string

// String implements fmt.Stringer.
func (d DataStatus) String() string {
	return string(d)
}

const (
	// DataFrozen means the business data is reserved by a pending
	// transaction.
	DataFrozen DataStatus = "frozen"
	// DataSuccessful means the business data has been committed.
	DataSuccessful DataStatus = "successful"
)

// MockComponent is a Redis backed TCC component. Its try phase freezes the
// business data, confirm commits it and cancel releases the reservation.
type MockComponent struct {
	id     string
	client pkg.RedisClient
}

// NewMockComponent builds a component with the given id. The client is used
// both for the state machine keys and for the per-transaction locks.
func NewMockComponent(id string, client pkg.RedisClient) *MockComponent {
	return &MockComponent{
		id:     id,
		client: client,
	}
}

// ID returns the unique component id.
func (m *MockComponent) ID() string {
	return m.id
}

// Try executes the first phase of the two-phase commit.
func (m *MockComponent) Try(ctx context.Context, req *gotcc.TCCReq) (*gotcc.TCCResp, error) {
	// Serialize all operations of one transaction on this component.
	lock := redis_lock.NewRedisLock(pkg.BuildTXLockKey(m.id, req.TXID), m.client)
	if err := lock.Lock(ctx); err != nil {
		return nil, err
	}
	defer func() {
		_ = lock.Unlock(ctx)
	}()

	// Idempotency dedup on the transaction id.
	txStatus, err := m.client.Get(ctx, pkg.BuildTXKey(m.id, req.TXID))
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	res := gotcc.TCCResp{
		ComponentID: m.id,
		TXID:        req.TXID,
	}
	switch txStatus {
	case TXTried.String(), TXConfirmed.String():
		// Repeated try request, answer with a successful ack.
		res.ACK = true
		return &res, nil
	case TXCanceled.String():
		// Cancel arrived before the try request: reject.
		return &res, nil
	default:
	}

	// Execute the try phase: freeze the business data.
	bizID := toString(req.Data["biz_id"])
	// Store the relation between the transaction and the business id.
	if _, err = m.client.Set(ctx, pkg.BuildTXDetailKey(m.id, req.TXID), bizID); err != nil {
		return nil, err
	}

	// The data must be frozen from scratch: SetNX fails when the data is
	// already frozen by another transaction.
	reply, err := m.client.SetNX(ctx, pkg.BuildDataKey(m.id, req.TXID, bizID), DataFrozen.String())
	if err != nil {
		return nil, err
	}
	if reply != 1 {
		return &res, nil
	}

	// Update the transaction status.
	if _, err = m.client.Set(ctx, pkg.BuildTXKey(m.id, req.TXID), TXTried.String()); err != nil {
		return nil, err
	}

	// The try phase succeeded.
	res.ACK = true
	return &res, nil
}

// Confirm executes the second-phase confirm.
func (m *MockComponent) Confirm(ctx context.Context, txID string) (*gotcc.TCCResp, error) {
	// Serialize all operations of one transaction on this component.
	lock := redis_lock.NewRedisLock(pkg.BuildTXLockKey(m.id, txID), m.client)
	if err := lock.Lock(ctx); err != nil {
		return nil, err
	}
	defer func() {
		_ = lock.Unlock(ctx)
	}()

	// 1. The transaction must have been tried before.
	txStatus, err := m.client.Get(ctx, pkg.BuildTXKey(m.id, txID))
	if err != nil {
		return nil, err
	}

	res := gotcc.TCCResp{
		ComponentID: m.id,
		TXID:        txID,
	}
	switch txStatus {
	case TXConfirmed.String():
		// Already confirmed, answer idempotently with a successful ack.
		res.ACK = true
		return &res, nil
	case TXTried.String():
		// Only a tried transaction may be confirmed.
	default:
		// Any other state: reject.
		return &res, nil
	}

	// Fetch the business id of the transaction.
	bizID, err := m.client.Get(ctx, pkg.BuildTXDetailKey(m.id, txID))
	if err != nil {
		return nil, err
	}

	// 2. The data must have been frozen before.
	dataStatus, err := m.client.Get(ctx, pkg.BuildDataKey(m.id, txID, bizID))
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	if dataStatus != DataFrozen.String() {
		// Illegal data state (including a missing data key): reject.
		return &res, nil
	}

	// Move the data to the successful state.
	if _, err = m.client.Set(ctx, pkg.BuildDataKey(m.id, txID, bizID), DataSuccessful.String()); err != nil {
		return nil, err
	}

	// Mark the transaction as confirmed. A failure here does not block the
	// main flow.
	_, _ = m.client.Set(ctx, pkg.BuildTXKey(m.id, txID), TXConfirmed.String())

	// Confirm succeeded, answer with a successful ack.
	res.ACK = true
	return &res, nil
}

// Cancel executes the second-phase cancel.
func (m *MockComponent) Cancel(ctx context.Context, txID string) (*gotcc.TCCResp, error) {
	// Serialize all operations of one transaction on this component.
	lock := redis_lock.NewRedisLock(pkg.BuildTXLockKey(m.id, txID), m.client)
	if err := lock.Lock(ctx); err != nil {
		return nil, err
	}
	defer func() {
		_ = lock.Unlock(ctx)
	}()

	// Every transaction that has not been confirmed yet is canceled.
	txStatus, err := m.client.Get(ctx, pkg.BuildTXKey(m.id, txID))
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	// Confirm before cancel is an illegal state transition.
	if txStatus == TXConfirmed.String() {
		return nil, fmt.Errorf("invalid tx status: %s, txid: %s", txStatus, txID)
	}

	// Fetch the business id of the transaction.
	bizID, err := m.client.Get(ctx, pkg.BuildTXDetailKey(m.id, txID))
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	if errors.Is(err, redis.Nil) {
		// The try phase never ran for this transaction, so there is
		// nothing to roll back. Acknowledge the cancel, otherwise the
		// monitor task would retry this transaction forever.
		_, _ = m.client.Set(ctx, pkg.BuildTXKey(m.id, txID), TXCanceled.String())
		return &gotcc.TCCResp{
			ACK:         true,
			ComponentID: m.id,
			TXID:        txID,
		}, nil
	}

	// Delete the frozen data record.
	if err = m.client.Del(ctx, pkg.BuildDataKey(m.id, txID, bizID)); err != nil {
		return nil, err
	}

	// Mark the transaction as canceled.
	_, _ = m.client.Set(ctx, pkg.BuildTXKey(m.id, txID), TXCanceled.String())

	return &gotcc.TCCResp{
		ACK:         true,
		ComponentID: m.id,
		TXID:        txID,
	}, nil
}

// toString converts a request field into a string.
func toString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprintf("%v", t)
	}
}
