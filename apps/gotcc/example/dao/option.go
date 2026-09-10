package dao

import (
	"github.com/hangtiancheng/swifty.go/apps/gotcc"
	"gorm.io/gorm"
)

// QueryOption narrows down a TXRecordPO query.
type QueryOption func(db *gorm.DB) *gorm.DB

// WithID filters by the transaction record id.
func WithID(id uint) QueryOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("id = ?", id)
	}
}

// WithStatus filters by the transaction status.
func WithStatus(status gotcc.TXStatus) QueryOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("status = ?", status.String())
	}
}
