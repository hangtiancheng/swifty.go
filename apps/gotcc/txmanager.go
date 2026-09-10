package gotcc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hangtiancheng/swifty.go/apps/gotcc/internal/log"
)

// TXManager is the transaction coordinator of the TCC framework. It wires
// together the two building blocks:
//  1. the TXStore transaction-log storage module, and
//  2. the TCCComponent registry,
//
// and drives the try-confirm/cancel two-phase commit flow for every
// transaction.
type TXManager struct {
	ctx            context.Context
	stop           context.CancelFunc
	opts           *Options
	txStore        TXStore
	registryCenter *registryCenter
}

// NewTXManager creates a transaction coordinator backed by the given
// TXStore. The internal monitor task starts immediately.
func NewTXManager(txStore TXStore, opts ...Option) *TXManager {
	ctx, cancel := context.WithCancel(context.Background())
	txManager := TXManager{
		opts:           &Options{},
		txStore:        txStore,
		registryCenter: newRegistryCenter(),
		ctx:            ctx,
		stop:           cancel,
	}

	for _, opt := range opts {
		opt(txManager.opts)
	}

	repair(txManager.opts)

	go txManager.run()
	return &txManager
}

// Stop stops the monitor task of the coordinator.
func (t *TXManager) Stop() {
	t.stop()
}

// Register adds a TCCComponent to the coordinator.
func (t *TXManager) Register(component TCCComponent) error {
	return t.registryCenter.register(component)
}

// Transaction starts a new distributed transaction across the components
// referenced by reqs. It returns the transaction id and whether the
// two-phase commit succeeded from the caller's point of view.
func (t *TXManager) Transaction(ctx context.Context, reqs ...*RequestEntity) (string, bool, error) {
	tctx, cancel := context.WithTimeout(ctx, t.opts.Timeout)
	defer cancel()

	// Resolve all requested components.
	componentEntities, err := t.getComponents(tctx, reqs...)
	if err != nil {
		return "", false, err
	}

	// 1. Create the transaction record and obtain the global transaction id.
	txID, err := t.txStore.CreateTX(tctx, componentEntities.ToComponents()...)
	if err != nil {
		return "", false, err
	}

	// 2. Two-phase commit: try, then confirm/cancel.
	return txID, t.twoPhaseCommit(ctx, txID, componentEntities), nil
}

// backOffTick doubles the tick and caps it at eight times MonitorTick.
func (t *TXManager) backOffTick(tick time.Duration) time.Duration {
	tick <<= 1
	if threshold := t.opts.MonitorTick << 3; tick > threshold {
		return threshold
	}
	return tick
}

// run is the monitor loop: it periodically locks the store, fetches the
// hanging transactions and advances their progress.
func (t *TXManager) run() {
	var tick time.Duration
	var err error
	for {
		// After a failure, back off the tick so that retry storms do not
		// hammer the store.
		if err == nil {
			tick = t.opts.MonitorTick
		} else {
			tick = t.backOffTick(tick)
		}
		select {
		case <-t.ctx.Done():
			return

		case <-time.After(tick):
			// Lock the store so that monitor tasks of different nodes of a
			// distributed deployment do not run concurrently.
			if err = t.txStore.Lock(t.ctx, t.opts.MonitorTick); err != nil {
				// Lock acquisition failure (most likely held by another
				// node) must not escalate the back off.
				err = nil
				continue
			}

			// Fetch the transactions that are still hanging.
			var txs []*Transaction
			if txs, err = t.txStore.GetHangingTXs(t.ctx); err != nil {
				_ = t.txStore.Unlock(t.ctx)
				continue
			}

			err = t.batchAdvanceProgress(txs)
			_ = t.txStore.Unlock(t.ctx)
		}
	}
}

// batchAdvanceProgress advances every given transaction concurrently and
// returns the first error encountered, if any.
func (t *TXManager) batchAdvanceProgress(txs []*Transaction) error {
	errCh := make(chan error)
	go func() {
		// Advance all transactions concurrently.
		var wg sync.WaitGroup
		for _, tx := range txs {
			// shadow
			wg.Go(func() {
				// Each goroutine advances one transaction.
				if err := t.advanceProgress(tx); err != nil {
					// Forward the error to the collector below.
					errCh <- err
				}
			})
		}
		wg.Wait()
		close(errCh)
	}()

	var firstErr error
	// Block here until every goroutine finished and errCh was closed.
	for err := range errCh {
		// Keep the first error only.
		if firstErr != nil {
			continue
		}
		firstErr = err
	}

	return firstErr
}

// advanceProgressByTXID advances the transaction with the given id.
func (t *TXManager) advanceProgressByTXID(txID string) error {
	// Fetch the transaction record.
	tx, err := t.txStore.GetTX(t.ctx, txID)
	if err != nil {
		return err
	}
	return t.advanceProgress(tx)
}

// advanceProgress drives a single transaction to its next state.
func (t *TXManager) advanceProgress(tx *Transaction) error {
	// Infer the transaction state from the try results of its components.
	txStatus := tx.getStatus(time.Now().Add(-t.opts.Timeout))
	// Hanging transactions are left alone for now.
	if txStatus == TXHanging {
		return nil
	}

	// Depending on the outcome, the second phase is confirm or cancel.
	success := txStatus == TXSuccessful
	var confirmOrCancel func(ctx context.Context, component TCCComponent) (*TCCResp, error)
	var txAdvanceProgress func(ctx context.Context) error
	if success {
		confirmOrCancel = func(ctx context.Context, component TCCComponent) (*TCCResp, error) {
			// Second-phase confirm of the component.
			return component.Confirm(ctx, tx.TXID)
		}
		txAdvanceProgress = func(ctx context.Context) error {
			// Mark the transaction record as successful.
			return t.txStore.TXSubmit(ctx, tx.TXID, true)
		}

	} else {
		confirmOrCancel = func(ctx context.Context, component TCCComponent) (*TCCResp, error) {
			// Second-phase cancel of the component.
			return component.Cancel(ctx, tx.TXID)
		}

		txAdvanceProgress = func(ctx context.Context) error {
			// Mark the transaction record as failed.
			return t.txStore.TXSubmit(ctx, tx.TXID, false)
		}
	}

	for _, component := range tx.Components {
		// Resolve the corresponding TCC component.
		components, err := t.registryCenter.getComponents(component.ComponentID)
		if err != nil {
			return fmt.Errorf("get tcc component %s failed: %w", component.ComponentID, err)
		}
		if len(components) == 0 {
			return fmt.Errorf("tcc component %s not registered", component.ComponentID)
		}
		// Execute the second-phase confirm or cancel.
		resp, err := confirmOrCancel(t.ctx, components[0])
		if err != nil {
			return err
		}
		if !resp.ACK {
			return fmt.Errorf("component: %s ack failed", component.ComponentID)
		}
	}

	// Once the second phase completed, commit the transaction state.
	return txAdvanceProgress(t.ctx)
}

// twoPhaseCommit performs the caller-visible part of the two-phase commit:
// it fans the try requests out concurrently, cancels everything on the
// first failure and immediately advances the transaction afterwards.
func (t *TXManager) twoPhaseCommit(ctx context.Context, txID string, componentEntities ComponentEntities) bool {
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Abort the whole flow and cancel as soon as a single try fails.
	errCh := make(chan error, len(componentEntities))
	go func() {
		// Run the try phase of all components concurrently.
		var wg sync.WaitGroup
		for _, componentEntity := range componentEntities {
			// shadow
			wg.Go(func() {
				resp, err := componentEntity.Component.Try(cctx, &TCCReq{
					ComponentID: componentEntity.Component.ID(),
					TXID:        txID,
					Data:        componentEntity.Request,
				})
				// A failed or rejected try means the transaction must be
				// cancelled; the cancel itself is handled by the
				// advanceProgressByTXID flow below.
				if err != nil || !resp.ACK {
					log.ErrorContextf(cctx, "tx try failed, tx id: %s, component id: %s, err: %v", txID, componentEntity.Component.ID(), err)
					// Record the rejection in the transaction log.
					if _err := t.txStore.TXUpdate(cctx, txID, componentEntity.Component.ID(), false); _err != nil {
						log.ErrorContextf(cctx, "tx update failed, tx id: %s, component id: %s, err: %v", txID, componentEntity.Component.ID(), _err)
					}
					errCh <- fmt.Errorf("component: %s try failed", componentEntity.Component.ID())
					return
				}
				// A try that succeeded but cannot be recorded in the
				// transaction log counts as a failure as well.
				if err = t.txStore.TXUpdate(cctx, txID, componentEntity.Component.ID(), true); err != nil {
					log.ErrorContextf(cctx, "tx update failed, tx id: %s, component id: %s, err: %v", txID, componentEntity.Component.ID(), err)
					errCh <- err
				}
			})
		}

		wg.Wait()
		close(errCh)
	}()

	successful := true
	if err := <-errCh; err != nil {
		// The first failure aborts all remaining try requests.
		cancel()
		successful = false
	}

	// Run the second phase. Even if it fails here, the monitor task will
	// pick the transaction up later and drive it to completion.
	if err := t.advanceProgressByTXID(txID); err != nil {
		log.ErrorContextf(ctx, "advance tx progress failed, txid: %s, err: %v", txID, err)
	}
	return successful
}

// getComponents validates the requests and resolves them into
// ComponentEntities.
func (t *TXManager) getComponents(ctx context.Context, reqs ...*RequestEntity) (ComponentEntities, error) {
	if len(reqs) == 0 {
		return nil, errors.New("empty request")
	}

	// Validate that every referenced component is known and unique.
	idToReq := make(map[string]*RequestEntity, len(reqs))
	componentIDs := make([]string, 0, len(reqs))
	for _, req := range reqs {
		if _, ok := idToReq[req.ComponentID]; ok {
			return nil, fmt.Errorf("duplicate component: %s", req.ComponentID)
		}
		idToReq[req.ComponentID] = req
		componentIDs = append(componentIDs, req.ComponentID)
	}

	components, err := t.registryCenter.getComponents(componentIDs...)
	if err != nil {
		return nil, err
	}
	if len(componentIDs) != len(components) {
		return nil, errors.New("invalid component ids")
	}

	entities := make(ComponentEntities, 0, len(components))
	for _, component := range components {
		entities = append(entities, &ComponentEntity{
			Request:   idToReq[component.ID()].Request,
			Component: component,
		})
	}

	return entities, nil
}
