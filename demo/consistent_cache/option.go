package consistent_cache

import (
	"math/rand"
	"time"

	"github.com/hangtiancheng/swifty.go/demo/consistent_cache/lib/log"
)

type Options struct {
	// cacheExpireSeconds is the cache TTL in seconds.
	cacheExpireSeconds int64
	// cacheExpireRandomMode adds jitter to the TTL to avoid cache stampede.
	cacheExpireRandomMode bool
	// disableExpireSeconds is the TTL of the read-path disable marker, in seconds.
	disableExpireSeconds int64
	// enableDelayMilis is the delay applied to re-enabling the read path after a write, in milliseconds.
	enableDelayMilis int64
	// rander is the RNG used for TTL jitter.
	rander *rand.Rand
	// logger handles diagnostic output.
	logger Logger
}

func (o *Options) CacheExpireSeconds() int64 {
	if !o.cacheExpireRandomMode {
		return o.cacheExpireSeconds
	}

	// Jitter between 1x and 2x the base TTL.
	return o.cacheExpireSeconds + o.rander.Int63n(o.cacheExpireSeconds+1)
}

type Option func(*Options)

const (
	// DefaultCacheExpireSeconds is the default cache TTL (60s).
	DefaultCacheExpireSeconds = 60
	// DefaultDisableExpireSeconds is the default disable-marker TTL (10s).
	DefaultDisableExpireSeconds = 10
	// DefaultEnableDelayMilis is the default re-enable delay (1s).
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
		o.rander = rand.New(rand.NewSource(time.Now().UnixNano()))
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
