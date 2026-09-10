package gotcc

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/gotcc/internal/uuid"
)

// mockTXStore is an in-memory TXStore fake used by the coordinator tests.
type mockTXStore struct {
	mutex sync.Mutex
	txs   map[string]*Transaction
}

func newMockTXStore() TXStore {
	return &mockTXStore{
		txs: make(map[string]*Transaction),
	}
}

// CreateTX creates an in-memory transaction record.
func (m *mockTXStore) CreateTX(ctx context.Context, components ...TCCComponent) (string, error) {
	txid := uuid.New()
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, ok := m.txs[txid]; ok {
		return "", fmt.Errorf("duplicate txid: %s", txid)
	}

	componentTryEntities := make([]*ComponentTryEntity, 0, len(components))
	for _, component := range components {
		componentTryEntities = append(componentTryEntities, &ComponentTryEntity{
			ComponentID: component.ID(),
			TryStatus:   TryHanging,
		})
	}

	m.txs[txid] = &Transaction{
		TXID:       txid,
		Status:     TXHanging,
		CreatedAt:  time.Now(),
		Components: componentTryEntities,
	}

	return txid, nil
}

// TXUpdate records the try-phase response of one component.
func (m *mockTXStore) TXUpdate(ctx context.Context, txID string, componentID string, accept bool) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	tx, ok := m.txs[txID]
	if !ok {
		return fmt.Errorf("[TXUpdate] invalid txid: %s", txID)
	}
	for _, component := range tx.Components {
		if component.ComponentID != componentID {
			continue
		}
		if component.TryStatus != TryHanging {
			return fmt.Errorf("invalid component status: %s, component id: %s, txid: %s", component.TryStatus, componentID, txID)
		}
		if accept {
			component.TryStatus = TrySuccessful
		} else {
			component.TryStatus = TryFailure
		}
		return nil
	}
	return fmt.Errorf("[TXUpdate] invalid component id: %s for txid: %s", componentID, txID)
}

// TXSubmit commits the final state of a transaction.
func (m *mockTXStore) TXSubmit(ctx context.Context, txID string, success bool) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	tx, ok := m.txs[txID]
	if !ok {
		return fmt.Errorf("[TXSubmit] invalid txid: %s", txID)
	}
	if success {
		if tx.Status != TXHanging && tx.Status != TXSuccessful {
			return fmt.Errorf("invalid tx status: %s, txid: %s", tx.Status, txID)
		}
		tx.Status = TXSuccessful
	} else {
		if tx.Status != TXHanging && tx.Status != TXFailure {
			return fmt.Errorf("invalid tx status: %s, txid: %s", tx.Status, txID)
		}
		tx.Status = TXFailure
	}
	return nil
}

// GetHangingTXs returns all transactions that are still hanging.
func (m *mockTXStore) GetHangingTXs(ctx context.Context) ([]*Transaction, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	var hangingTXs []*Transaction
	for _, tx := range m.txs {
		if tx.Status != TXHanging {
			continue
		}
		hangingTXs = append(hangingTXs, cloneTransaction(tx))
	}
	return hangingTXs, nil
}

// GetTX returns a single transaction by id.
func (m *mockTXStore) GetTX(ctx context.Context, txID string) (*Transaction, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	tx, ok := m.txs[txID]
	if !ok {
		return nil, fmt.Errorf("[GetTX] invalid txid: %s", txID)
	}
	return cloneTransaction(tx), nil
}

// cloneTransaction returns a deep copy so that callers never share memory
// with the mutable store state, mirroring the snapshot read semantics of a
// real database backed store.
func cloneTransaction(tx *Transaction) *Transaction {
	components := make([]*ComponentTryEntity, 0, len(tx.Components))
	for _, component := range tx.Components {
		cp := *component
		components = append(components, &cp)
	}
	cp := *tx
	cp.Components = components
	return &cp
}

// Lock is a no-op: the in-memory store needs no distributed lock.
func (m *mockTXStore) Lock(ctx context.Context, expireDuration time.Duration) error {
	return nil
}

// Unlock is a no-op for the same reason.
func (m *mockTXStore) Unlock(ctx context.Context) error {
	return nil
}

// Component state machine used by mockComponent.
type componentStatus string

const (
	statusTried     componentStatus = "tried"
	statusConfirmed componentStatus = "confirmed"
	statusCanceled  componentStatus = "canceled"
)

// mockComponent is an in-memory TCCComponent fake.
type mockComponent struct {
	id            string
	mutex         sync.Mutex
	statusMachine map[string]componentStatus
}

func newMockComponent(id string) TCCComponent {
	return &mockComponent{
		id:            id,
		statusMachine: make(map[string]componentStatus),
	}
}

// ID returns the unique component id.
func (m *mockComponent) ID() string {
	return m.id
}

// Try executes the first phase of the two-phase commit.
func (m *mockComponent) Try(ctx context.Context, req *TCCReq) (*TCCResp, error) {
	resp := TCCResp{
		ComponentID: m.id,
		TXID:        req.TXID,
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.statusMachine[req.TXID] == statusCanceled {
		return &resp, nil
	}

	if req.Data["reject_flag"] == true {
		m.statusMachine[req.TXID] = statusCanceled
		return &resp, nil
	}

	if req.Data["hanging_flag"] == true {
		<-time.After(time.Second)
		return &resp, nil
	}

	if m.statusMachine[req.TXID] != statusConfirmed {
		m.statusMachine[req.TXID] = statusTried
	}

	resp.ACK = true
	return &resp, nil
}

// Confirm executes the second-phase confirm.
func (m *mockComponent) Confirm(ctx context.Context, txID string) (*TCCResp, error) {
	resp := TCCResp{
		ComponentID: m.id,
		TXID:        txID,
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.statusMachine[txID] != statusTried && m.statusMachine[txID] != statusConfirmed {
		return &resp, nil
	}

	resp.ACK = true
	m.statusMachine[txID] = statusConfirmed
	return &resp, nil
}

// Cancel executes the second-phase cancel.
func (m *mockComponent) Cancel(ctx context.Context, txID string) (*TCCResp, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.statusMachine[txID] == statusConfirmed {
		return nil, errors.New("invalid status machine: [confirmed] when canceling")
	}

	m.statusMachine[txID] = statusCanceled
	return &TCCResp{
		ComponentID: m.id,
		ACK:         true,
		TXID:        txID,
	}, nil
}

func TestTXManagerTransactionSuccess(t *testing.T) {
	txManager := NewTXManager(newMockTXStore())
	defer txManager.Stop()

	// Register 5 components.
	componentsCnt := 5
	componentReqs := make([]*RequestEntity, 0, componentsCnt)
	ctx := context.Background()
	for i := range componentsCnt {
		componentID := strconv.Itoa(i)
		if err := txManager.Register(newMockComponent(componentID)); err != nil {
			t.Fatal(err)
		}
		componentReqs = append(componentReqs, &RequestEntity{
			ComponentID: componentID,
		})
	}

	txid, ok, err := txManager.Transaction(ctx, componentReqs...)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("transaction should succeed")
	}

	tx, err := txManager.txStore.GetTX(ctx, txid)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status != TXSuccessful {
		t.Fatalf("tx status = %s, want %s", tx.Status, TXSuccessful)
	}
}

// TestTXManagerTransactionFailure verifies the rollback path of a
// distributed transaction.
func TestTXManagerTransactionFailure(t *testing.T) {
	txManager := NewTXManager(newMockTXStore())
	defer txManager.Stop()

	// Register 5 components, all rejecting their try phase.
	componentsCnt := 5
	componentReqs := make([]*RequestEntity, 0, componentsCnt)
	ctx := context.Background()
	for i := range componentsCnt {
		componentID := strconv.Itoa(i)
		if err := txManager.Register(newMockComponent(componentID)); err != nil {
			t.Fatal(err)
		}
		componentReqs = append(componentReqs, &RequestEntity{
			ComponentID: componentID,
			Request: map[string]any{
				"reject_flag": true,
			},
		})
	}

	txid, ok, err := txManager.Transaction(ctx, componentReqs...)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("transaction should fail")
	}

	tx, err := txManager.txStore.GetTX(ctx, txid)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status != TXFailure {
		t.Fatalf("tx status = %s, want %s", tx.Status, TXFailure)
	}
}

func TestTXManagerTransactionConcurrent(t *testing.T) {
	txManager := NewTXManager(newMockTXStore(), WithMonitorTick(0), WithTimeout(0))
	defer txManager.Stop()

	// Register 10 components.
	componentsCnt := 10
	for i := range componentsCnt {
		if err := txManager.Register(newMockComponent(strconv.Itoa(i))); err != nil {
			t.Fatal(err)
		}
	}

	// Run 100 concurrent transactions, each touching 3 random components.
	ctx := context.Background()
	concurrentTXs := 100
	componentReqCnt := 3
	var wg sync.WaitGroup
	for range concurrentTXs {
		wg.Go(func() {
			rander := rand.New(rand.NewSource(time.Now().UnixNano()))
			componentSet := make(map[string]struct{}, componentReqCnt)
			for len(componentSet) < componentReqCnt {
				componentSet[strconv.Itoa(rander.Intn(componentsCnt))] = struct{}{}
			}

			componentReqs := make([]*RequestEntity, 0, componentReqCnt)
			for componentID := range componentSet {
				componentReqs = append(componentReqs, &RequestEntity{
					ComponentID: componentID,
				})
			}

			txid, ok, err := txManager.Transaction(ctx, componentReqs...)
			if err != nil {
				t.Error(err)
				return
			}
			if !ok {
				t.Error("transaction should succeed")
				return
			}
			tx, err := txManager.txStore.GetTX(ctx, txid)
			if err != nil {
				t.Error(err)
				return
			}
			if tx.Status != TXSuccessful {
				t.Errorf("tx status = %s, want %s", tx.Status, TXSuccessful)
			}
		})
	}

	wg.Wait()
}

func TestTXManagerTransactionAdvanceProgress(t *testing.T) {
	txManager := NewTXManager(newMockTXStore(), WithMonitorTick(100*time.Millisecond))
	defer txManager.Stop()

	// Register 5 components whose try requests hang for one second and are
	// then rejected, forcing the monitor task to roll the transaction back.
	componentsCnt := 5
	componentReqs := make([]*RequestEntity, 0, componentsCnt)
	ctx := context.Background()
	for i := range componentsCnt {
		componentID := strconv.Itoa(i)
		if err := txManager.Register(newMockComponent(componentID)); err != nil {
			t.Fatal(err)
		}
		componentReqs = append(componentReqs, &RequestEntity{
			ComponentID: componentID,
			Request: map[string]any{
				"hanging_flag": true,
			},
		})
	}

	txid, ok, err := txManager.Transaction(ctx, componentReqs...)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("transaction should fail")
	}

	tx, err := txManager.txStore.GetTX(ctx, txid)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status != TXFailure {
		t.Fatalf("tx status = %s, want %s", tx.Status, TXFailure)
	}
}

func TestTXManagerBackOffTick(t *testing.T) {
	txManager := NewTXManager(newMockTXStore(), WithMonitorTick(time.Second))
	defer txManager.Stop()

	got := txManager.backOffTick(time.Second)
	if got != 2*time.Second {
		t.Fatalf("backOffTick = %s, want %s", got, 2*time.Second)
	}
	got = txManager.backOffTick(got)
	if got != 4*time.Second {
		t.Fatalf("backOffTick = %s, want %s", got, 4*time.Second)
	}
	got = txManager.backOffTick(got)
	if got != 8*time.Second {
		t.Fatalf("backOffTick = %s, want %s", got, 8*time.Second)
	}
	got = txManager.backOffTick(got)
	if got != 8*time.Second {
		t.Fatalf("backOffTick = %s, want %s (capped)", got, 8*time.Second)
	}
}

func TestRegistryCenterRegisterDuplicate(t *testing.T) {
	txManager := NewTXManager(newMockTXStore())
	defer txManager.Stop()

	if err := txManager.Register(newMockComponent("a")); err != nil {
		t.Fatal(err)
	}
	err := txManager.Register(newMockComponent("a"))
	if err == nil {
		t.Fatal("registering a duplicate component id must fail")
	}
}

// slowComponent is a TCCComponent whose try phase blocks until its context
// is done.
type slowComponent struct {
	id string
}

func (s *slowComponent) ID() string { return s.id }

func (s *slowComponent) Try(ctx context.Context, req *TCCReq) (*TCCResp, error) {
	select {
	case <-ctx.Done():
		return &TCCResp{ComponentID: s.id, TXID: req.TXID}, ctx.Err()
	case <-time.After(10 * time.Second):
		return &TCCResp{ComponentID: s.id, ACK: true, TXID: req.TXID}, nil
	}
}

func (s *slowComponent) Confirm(ctx context.Context, txID string) (*TCCResp, error) {
	return &TCCResp{ComponentID: s.id, ACK: true, TXID: txID}, nil
}

func (s *slowComponent) Cancel(ctx context.Context, txID string) (*TCCResp, error) {
	return &TCCResp{ComponentID: s.id, ACK: true, TXID: txID}, nil
}

// TestTXManagerTransactionTimeout verifies that the transaction timeout
// bounds the try phase: a component whose try blocks longer than the
// timeout must not keep the caller waiting, and the transaction must be
// rolled back.
func TestTXManagerTransactionTimeout(t *testing.T) {
	txManager := NewTXManager(newMockTXStore(), WithTimeout(200*time.Millisecond))
	defer txManager.Stop()

	if err := txManager.Register(&slowComponent{id: "slow"}); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	txid, ok, err := txManager.Transaction(context.Background(), &RequestEntity{
		ComponentID: "slow",
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("transaction should fail")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Transaction returned after %s, the timeout must bound the try phase", elapsed)
	}

	tx, err := txManager.txStore.GetTX(context.Background(), txid)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status != TXFailure {
		t.Fatalf("tx status = %s, want %s", tx.Status, TXFailure)
	}
}
