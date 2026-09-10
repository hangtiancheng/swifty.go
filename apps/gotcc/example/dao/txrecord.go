package dao

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hangtiancheng/swifty.go/apps/gotcc"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TXRecordPO is the persistence object of one transaction record.
type TXRecordPO struct {
	gorm.Model
	// Status is the gotcc.TXStatus of the transaction.
	Status string `gorm:"status"`
	// ComponentTryStatuses is the JSON encoded map of the per-component
	// try statuses.
	ComponentTryStatuses string `gorm:"component_try_statuses"`
}

// TableName implements gorm's tabler interface.
func (t TXRecordPO) TableName() string {
	return "tx_record"
}

// ComponentTryStatus is one component's try status inside a transaction
// record.
type ComponentTryStatus struct {
	ComponentID string `json:"componentID"`
	TryStatus   string `json:"tryStatus"`
}

// ParseComponentTryStatuses decodes the JSON body stored in
// TXRecordPO.ComponentTryStatuses.
func ParseComponentTryStatuses(body string) (map[string]*ComponentTryStatus, error) {
	statuses := make(map[string]*ComponentTryStatus)
	if err := json.Unmarshal([]byte(body), &statuses); err != nil {
		return nil, fmt.Errorf("parse component try statuses failed: %w", err)
	}
	return statuses, nil
}

// TXRecordDAO persists transaction records in MySQL via GORM.
type TXRecordDAO struct {
	db *gorm.DB
}

// NewTXRecordDAO builds a DAO on top of the given GORM handle.
func NewTXRecordDAO(db *gorm.DB) *TXRecordDAO {
	return &TXRecordDAO{
		db: db,
	}
}

// GetTXRecords queries transaction records filtered by the given options.
func (t *TXRecordDAO) GetTXRecords(ctx context.Context, opts ...QueryOption) ([]*TXRecordPO, error) {
	db := t.db.WithContext(ctx).Model(&TXRecordPO{})
	for _, opt := range opts {
		db = opt(db)
	}

	var records []*TXRecordPO
	return records, db.Scan(&records).Error
}

// CreateTXRecord inserts a new transaction record and returns its
// auto-generated id.
func (t *TXRecordDAO) CreateTXRecord(ctx context.Context, record *TXRecordPO) (uint, error) {
	if err := t.db.WithContext(ctx).Model(&TXRecordPO{}).Create(record).Error; err != nil {
		return 0, err
	}
	return record.ID, nil
}

// UpdateComponentStatus transitions the try status of one component of the
// given transaction from hanging to the given status.
func (t *TXRecordDAO) UpdateComponentStatus(ctx context.Context, id uint, componentID string, status string) error {
	return t.LockAndDo(ctx, id, func(ctx context.Context, dao TXRecordUpdater, record *TXRecordPO) error {
		newBody, err := applyComponentStatus(record.ComponentTryStatuses, componentID, status)
		if err != nil {
			return fmt.Errorf("%w, txid: %d", err, id)
		}
		if newBody == record.ComponentTryStatuses {
			return nil
		}
		record.ComponentTryStatuses = newBody
		return dao.UpdateTXRecord(ctx, record)
	})
}

// applyComponentStatus transitions the try status of one component inside
// the JSON encoded statuses body. It returns the new body, which is
// identical to the input when the transition is a no-op.
func applyComponentStatus(body, componentID, status string) (string, error) {
	statuses, err := ParseComponentTryStatuses(body)
	if err != nil {
		return "", err
	}

	componentStatus, ok := statuses[componentID]
	if !ok {
		return "", fmt.Errorf("invalid component: %s", componentID)
	}
	if componentStatus.TryStatus == status {
		// Already in the requested state, nothing to do.
		return body, nil
	}

	if componentStatus.TryStatus != gotcc.TryHanging.String() {
		return "", fmt.Errorf("invalid status: %s of component: %s", componentStatus.TryStatus, componentID)
	}

	componentStatus.TryStatus = status
	newBody, err := json.Marshal(statuses)
	if err != nil {
		return "", fmt.Errorf("marshal component try statuses failed: %w", err)
	}
	return string(newBody), nil
}

// UpdateTXRecord persists the mutable fields of the given record.
func (t *TXRecordDAO) UpdateTXRecord(ctx context.Context, record *TXRecordPO) error {
	return t.db.WithContext(ctx).Updates(record).Error
}

// TXRecordUpdater is the write handle handed to LockAndDo callbacks. It is
// an interface so that callers can be tested with an in-memory fake.
type TXRecordUpdater interface {
	UpdateTXRecord(ctx context.Context, record *TXRecordPO) error
}

// LockAndDo runs do inside a transaction after acquiring a row-level write
// lock (SELECT ... FOR UPDATE) on the record with the given id.
func (t *TXRecordDAO) LockAndDo(ctx context.Context, id uint, do func(ctx context.Context, dao TXRecordUpdater, record *TXRecordPO) error) error {
	return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Acquire the row write lock.
		var record TXRecordPO
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&record, id).Error; err != nil {
			return err
		}

		txDAO := NewTXRecordDAO(tx)
		return do(ctx, txDAO, &record)
	})
}
