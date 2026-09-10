package mysql

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/hangtiancheng/swifty.go/apps/consistent_cache"
)

type tabler interface {
	TableName() string
}

// DB is the gorm implementation of the database module.
type DB struct {
	db *gorm.DB
}

// Put writes the object into the database.
func (d *DB) Put(ctx context.Context, obj consistent_cache.Object) error {
	db := d.db
	if t, ok := obj.(tabler); ok {
		db = db.Table(t.TableName())
	}

	// The upsert effect is achieved through two non-atomic actions:
	// 1. try to create the record;
	// 2. if a unique key conflict occurs, fall back to an update.
	err := db.WithContext(ctx).Create(obj).Error
	if err == nil {
		return nil
	}

	// On a unique key conflict, fall back to an update. Select("*") makes the
	// struct update write every column: without it, gorm skips zero-valued
	// fields, so the record would not be fully overwritten (e.g. clearing a
	// data field back to its empty value would be silently dropped).
	if IsDuplicateEntryErr(err) {
		return db.WithContext(ctx).Where(fmt.Sprintf("`%s` = ?", obj.KeyColumn()), obj.Key()).Select("*").Updates(obj).Error
	}
	// Return any other error directly.
	return err
}

// Get reads the data of obj.Key() from the database into obj.
func (d *DB) Get(ctx context.Context, obj consistent_cache.Object) error {
	db := d.db
	if t, ok := obj.(tabler); ok {
		db = db.Table(t.TableName())
	}

	err := db.WithContext(ctx).Where(fmt.Sprintf("`%s` = ?", obj.KeyColumn()), obj.Key()).First(obj).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return consistent_cache.ErrorDBMiss
	}
	return err
}
