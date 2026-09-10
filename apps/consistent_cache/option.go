package consistent_cache

import (
	"math/rand/v2"

	"github.com/hangtiancheng/swifty.go/apps/consistent_cache/internal/log"
)

type Options struct {
	// cacheExpireSeconds is the cache expiry time in seconds.
	cacheExpireSeconds int64
	// cacheExpireRandomMode enables random jitter on the cache expiry time.
	cacheExpireRandomMode bool
	// disableExpireSeconds is the expiry time of the read-flow write-cache
	// disable mark, in seconds.
	disableExpireSeconds int64
	// enableDelayMilis is how long after the write-flow disable operation the
	// enable operation is performed, in milliseconds.
	enableDelayMilis int64
	// logger is used for log output.
	logger Logger
}

// CacheExpireSeconds returns the effective cache expiry time in seconds.
func (o *Options) CacheExpireSeconds() int64 {
	if !o.cacheExpireRandomMode {
		return o.cacheExpireSeconds
	}

	// The expiry time is a random value between 1x and 2x the configured expiry.
	return o.cacheExpireSeconds + rand.Int64N(o.cacheExpireSeconds+1)
}

type Option func(*Options)

const (
	// DefaultCacheExpireSeconds is the default cache expiry time (60 s).
	DefaultCacheExpireSeconds = 60
	// DefaultDisableExpireSeconds is the default write-cache disable time (10 s).
	DefaultDisableExpireSeconds = 10
	// DefaultEnableDelayMilis is the default delayed enable time (1 s).
	DefaultEnableDelayMilis = 1000
)

func WithCacheExpireSeconds(cacheExpireSeconds int64) Option {
	return func(o *Options) {
		o.cacheExpireSeconds = cacheExpireSeconds
	}
}

func WithCacheExpireRandomMode() Option {
	return func(o *Options) {
		o.cacheExpireRandomMode = true
	}
}

func WithDisableExpireSeconds(disableExpireSeconds int64) Option {
	return func(o *Options) {
		o.disableExpireSeconds = disableExpireSeconds
	}
}

func WithEnableDelayMilis(enableDelayMilis int64) Option {
	return func(o *Options) {
		o.enableDelayMilis = enableDelayMilis
	}
}

func WithLogger(logger Logger) Option {
	return func(o *Options) {
		o.logger = logger
	}
}

func repair(o *Options) {
	if o.cacheExpireSeconds <= 0 {
		o.cacheExpireSeconds = DefaultCacheExpireSeconds
	}

	if o.disableExpireSeconds <= 0 {
		o.disableExpireSeconds = DefaultDisableExpireSeconds
	}

	if o.enableDelayMilis <= 0 {
		o.enableDelayMilis = DefaultEnableDelayMilis
	}

	if o.logger == nil {
		o.logger = log.GetLogger()
	}
}
