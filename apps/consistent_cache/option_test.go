package consistent_cache

import (
	"testing"
)

// TestNewServiceDefaults verifies the default option values applied by repair.
func TestNewServiceDefaults(t *testing.T) {
	s := NewService(nil, nil)
	opts := s.opts

	if got, want := opts.cacheExpireSeconds, int64(DefaultCacheExpireSeconds); got != want {
		t.Errorf("default cacheExpireSeconds = %d, want %d", got, want)
	}
	if got, want := opts.disableExpireSeconds, int64(DefaultDisableExpireSeconds); got != want {
		t.Errorf("default disableExpireSeconds = %d, want %d", got, want)
	}
	if got, want := opts.enableDelayMilis, int64(DefaultEnableDelayMilis); got != want {
		t.Errorf("default enableDelayMilis = %d, want %d", got, want)
	}
	if opts.logger == nil {
		t.Error("default logger is nil")
	}
}

// TestOptionsCacheExpireSeconds verifies CacheExpireSeconds in fixed mode.
func TestOptionsCacheExpireSeconds(t *testing.T) {
	tests := []struct {
		name          string
		configured    int64
		randomMode    bool
		wantFixed     int64
		wantMinJitter int64
		wantMaxJitter int64
	}{
		{
			name:          "fixed mode returns the configured value",
			configured:    120,
			randomMode:    false,
			wantFixed:     120,
			wantMinJitter: 0,
			wantMaxJitter: 0,
		},
		{
			name:          "random mode jitters between 1x and 2x",
			configured:    60,
			randomMode:    true,
			wantFixed:     0,
			wantMinJitter: 60,
			wantMaxJitter: 120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &Options{
				cacheExpireSeconds:    tt.configured,
				cacheExpireRandomMode: tt.randomMode,
			}

			for range 1000 {
				got := opts.CacheExpireSeconds()
				if tt.randomMode {
					if got < tt.wantMinJitter || got > tt.wantMaxJitter {
						t.Fatalf("CacheExpireSeconds() = %d, want within [%d, %d]", got, tt.wantMinJitter, tt.wantMaxJitter)
					}
				} else if got != tt.wantFixed {
					t.Fatalf("CacheExpireSeconds() = %d, want %d", got, tt.wantFixed)
				}
			}
		})
	}
}

// TestOptionsCacheExpireSecondsRandomProducesVariety verifies that the random
// mode does not always return the same value and is safe for concurrent use.
func TestOptionsCacheExpireSecondsRandomProducesVariety(t *testing.T) {
	opts := &Options{cacheExpireSeconds: 64, cacheExpireRandomMode: true}

	seen := make(map[int64]struct{})
	for range 1000 {
		seen[opts.CacheExpireSeconds()] = struct{}{}
	}
	if len(seen) < 2 {
		t.Errorf("random mode produced %d distinct values, want more than one", len(seen))
	}
}

// TestOptionsCacheExpireSecondsNonPositiveDoesNotPanic verifies that random
// mode with a non-positive configured expiry does not panic (rand.Int64N
// panics on a non-positive bound) and returns the configured value as is.
func TestOptionsCacheExpireSecondsNonPositiveDoesNotPanic(t *testing.T) {
	for _, configured := range []int64{0, -1, -100} {
		opts := &Options{cacheExpireSeconds: configured, cacheExpireRandomMode: true}
		if got, want := opts.CacheExpireSeconds(), configured; got != want {
			t.Errorf("CacheExpireSeconds() = %d, want %d", got, want)
		}
	}
}

// TestWithLogger verifies that the logger option is honored.
func TestWithLogger(t *testing.T) {
	l := &testLogger{}
	s := NewService(nil, nil, WithLogger(l))
	if s.opts.logger != l {
		t.Errorf("opts.logger = %T, want %T", s.opts.logger, l)
	}
}

type testLogger struct{}

func (*testLogger) Errorf(format string, v ...any) {}
func (*testLogger) Warnf(format string, v ...any)  {}
func (*testLogger) Infof(format string, v ...any)  {}
func (*testLogger) Debugf(format string, v ...any) {}
