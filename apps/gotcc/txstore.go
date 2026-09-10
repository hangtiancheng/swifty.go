package gotcc

import (
	"context"
	"time"
)

// TXStore persists the transaction log. Implementations are expected to be
// shared across instances of a distributed deployment, i.e. Lock must be a
// distributed lock.
type TXStore interface {
	// CreateTX creates a new transaction record and returns the globally
	// unique transaction id.
	CreateTX(ctx context.Context, components ...TCCComponent) (txID string, err error)
	// TXUpdate records the try-phase response of a single component.
	TXUpdate(ctx context.Context, txID string, componentID string, accept bool) error
	// TXSubmit commits the final transaction outcome (success or failure).
	TXSubmit(ctx context.Context, txID string, success bool) error
	// GetHangingTXs returns every transaction that has not finished yet.
	GetHangingTXs(ctx context.Context) ([]*Transaction, error)
	// GetTX returns a single transaction by id.
	GetTX(ctx context.Context, txID string) (*Transaction, error)
	// Lock acquires the store-wide (distributed) lock for the monitor task.
	Lock(ctx context.Context, expireDuration time.Duration) error
	// Unlock releases the store-wide lock.
	Unlock(ctx context.Context) error
}
