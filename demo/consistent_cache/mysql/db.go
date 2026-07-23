package mysql

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/hangtiancheng/swifty.go/demo/consistent_cache"
)

type tabler interface {
	TableName() string
}

// DB implements consistent_cache.DB backed by gorm.
type DB struct {
	db *gorm.DB
}

func NewDB(dsn string) *DB {
	return &DB{db: getDB(dsn)}
}

// Put writes obj to the database. It emulates upsert: try insert first, fall back to update on unique-key conflict.
func (d *DB) Put(ctx context.Context, obj consistent_cache.Object) error {
	db := d.db
	tabler, ok := obj.(tabler)
	if ok {
		db = db.Table(tabler.TableName())
	}

	err := db.WithContext(ctx).Create(obj).Error
	if err == nil {
		return nil
	}

	if IsDuplicateEntryErr(err) {
		return db.WithContext(ctx).Debug().Where(fmt.Sprintf("`%s` = ?", obj.KeyColumn()), obj.Key()).Updates(obj).Error
	}
	return err
}

// Get loads obj from the database by its key column.
func (d *DB) Get(ctx context.Context, obj consistent_cache.Object) error {
	db := d.db
	tabler, ok := obj.(tabler)
	if ok {
		db = db.Table(tabler.TableName())
	}

	err := db.WithContext(ctx).Where(fmt.Sprintf("`%s` = ?", obj.KeyColumn()), obj.Key()).First(obj).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return consistent_cache.ErrorDBMiss
	}
	return err
}
