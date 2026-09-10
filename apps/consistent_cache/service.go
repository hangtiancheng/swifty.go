package consistent_cache

import (
	"context"
	"errors"
	"time"
)

// Service is the cache consistency service.
type Service struct {
	// options
	opts *Options
	// cache module
	cache Cache
	// database module
	db DB
}

// NewService builds a cache consistency service. Concrete implementations of
// the cache and database modules are provided by the caller.
func NewService(cache Cache, db DB, opts ...Option) *Service {
	s := Service{
		cache: cache,
		db:    db,
		opts:  &Options{},
	}

	for _, opt := range opts {
		opt(s.opts)
	}

	repair(s.opts)
	return &s
}

// Put performs a write operation.
func (s *Service) Put(ctx context.Context, obj Object) error {
	// 1. Disable the read-flow write-cache mechanism for the key.
	if err := s.cache.Disable(ctx, obj.Key(), s.opts.disableExpireSeconds); err != nil {
		return err
	}

	defer func() {
		go func() {
			tctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := s.cache.Enable(tctx, obj.Key(), s.opts.enableDelayMilis); err != nil {
				s.opts.logger.Errorf("enable fail, key: %s, err: %v", obj.Key(), err)
			}
		}()
	}()

	// 2. Delete the cache entry of the key.
	if err := s.cache.Del(ctx, obj.Key()); err != nil {
		return err
	}

	// 3. Write the data into the database.
	return s.db.Put(ctx, obj)
}

// Get performs a read operation. useCache reports whether the value was served
// from the cache.
func (s *Service) Get(ctx context.Context, obj Object) (useCache bool, err error) {
	// 1. Read the cache.
	v, err := s.cache.Get(ctx, obj.Key())
	// 2. Propagate any error other than a cache miss directly.
	if err != nil && !errors.Is(err, ErrorCacheMiss) {
		return false, err
	}

	// 3. A cached value was found.
	if err == nil {
		// 3.1 The value is NullData, a placeholder written to prevent cache penetration.
		if v == NullData {
			return true, ErrorDataNotExist
		}
		// 3.2 A regular cached value.
		return true, obj.Read(v)
	}

	// 4. Cache miss: read the database.
	if err = s.db.Get(ctx, obj); err != nil && !errors.Is(err, ErrorDBMiss) {
		return false, err
	}

	// 5. The data is not in the database either; try to write NullData into the
	// cache to prevent cache penetration.
	if errors.Is(err, ErrorDBMiss) {
		if ok, err := s.cache.PutWhenEnable(ctx, obj.Key(), NullData, s.opts.CacheExpireSeconds()); err != nil {
			s.opts.logger.Errorf("put null data into cache fail, key: %s, err: %v", obj.Key(), err)
		} else {
			s.opts.logger.Infof("put null data into cache resp, key: %s, ok: %t", obj.Key(), ok)
		}

		return false, ErrorDataNotExist
	}

	// 6. The data was read successfully; write it into the cache.
	v, err = obj.Write()
	if err != nil {
		return false, err
	}
	if ok, err := s.cache.PutWhenEnable(ctx, obj.Key(), v, s.opts.CacheExpireSeconds()); err != nil {
		s.opts.logger.Errorf("put data into cache fail, key: %s, data: %v, err: %v", obj.Key(), v, err)
	} else {
		s.opts.logger.Infof("put data into cache resp, key: %s, v: %v, ok: %t", obj.Key(), v, ok)
	}

	// 7. Return the value that was read.
	return false, nil
}
