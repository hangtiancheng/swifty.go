package mysql

import (
	"errors"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// NewDB opens a gorm MySQL client for the given DSN.
func NewDB(dsn string) (*DB, error) {
	// TranslateError maps driver errors (e.g. duplicate entry) to
	// gorm.ErrDuplicatedKey so that no MySQL driver import is needed here.
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, err
	}
	return &DB{db: db}, nil
}

// IsDuplicateEntryErr reports whether err is a duplicate-entry (unique key
// conflict) error.
func IsDuplicateEntryErr(err error) bool {
	return errors.Is(err, gorm.ErrDuplicatedKey)
}
