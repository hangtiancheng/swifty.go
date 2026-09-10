package consistent_cache

import (
	"context"
	"errors"
)

var (
	ErrorDataNotExist = errors.New("data not exist")
	ErrorCacheMiss    = errors.New("cache miss")
	ErrorDBMiss       = errors.New("db miss")
)

const NullData = "Err_Syntax_Null_Data"

// Cache is the abstraction of the cache module.
type Cache interface {
	// Enable re-enables the read-flow write-cache mechanism for the key
	// (enabled by default). The mechanism becomes enabled again delayMilis
	// milliseconds after the call.
	Enable(ctx context.Context, key string, delayMilis int64) error
	// Disable disables the read-flow write-cache mechanism for the key.
	Disable(ctx context.Context, key string, expireSeconds int64) error
	// Get reads the cached value of the key.
	Get(ctx context.Context, key string) (string, error)
	// Del removes the cached value of the key.
	Del(ctx context.Context, key string) error
	// PutWhenEnable writes value for the key only when the read-flow
	// write-cache mechanism for the key is enabled (enabled by default).
	PutWhenEnable(ctx context.Context, key, value string, expireSeconds int64) (bool, error)
}

// DB is the abstraction of the database module.
type DB interface {
	// Put writes the object into the database.
	Put(ctx context.Context, obj Object) error
	// Get reads the data from the database.
	Get(ctx context.Context, obj Object) error
}

// Object is a single data record accessed by a read or write operation.
type Object interface {
	// KeyColumn returns the column name of the key.
	KeyColumn() string
	// Key returns the value of the key.
	Key() string

	// Write serializes the object into a string.
	Write() (string, error)
	// Read deserializes the string body into the object.
	Read(body string) error
}

// Logger is the abstraction of the logging module.
type Logger interface {
	Errorf(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Infof(format string, v ...interface{})
	Debugf(format string, v ...interface{})
}
