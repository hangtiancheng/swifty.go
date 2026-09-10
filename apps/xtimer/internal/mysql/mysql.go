package mysql

import (
	"errors"
	"fmt"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/config"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const DuplicateEntryErrCode = 1062

// Client is a gorm backed database client.
type Client struct {
	*gorm.DB
}

// GetClient builds a database client from the configuration.
func GetClient(confProvider *config.MysqlConfProvider) (*Client, error) {
	conf := confProvider.Get()
	db, err := gorm.Open(mysql.Open(conf.DSN), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("failed to connect database, err: %w", err))
	}
	_db, err := db.DB()
	if err != nil {
		panic(err)
	}
	_db.SetMaxOpenConns(conf.MaxOpenConns) // maximum number of open connections in the pool
	_db.SetMaxIdleConns(conf.MaxIdleConns) // maximum number of idle connections kept in the pool
	return &Client{DB: db}, nil
}

func NewClient(db *gorm.DB) *Client {
	return &Client{
		DB: db,
	}
}

func IsDuplicateEntryErr(err error) bool {
	var mysqlErr *mysqldriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == DuplicateEntryErrCode
}
