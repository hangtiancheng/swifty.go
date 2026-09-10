package pkg

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// NewDB opens a GORM MySQL connection for the given DSN.
func NewDB(dsn string, opts ...gorm.Option) (*gorm.DB, error) {
	return gorm.Open(mysql.Open(dsn), opts...)
}
