package redis

import "testing"

func TestNewClientDefaults(t *testing.T) {
	// Constructor must not dial; it only builds the client configuration.
	c := NewClient("tcp", "127.0.0.1:6379", "")
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.Client == nil {
		t.Fatal("underlying go-redis client is nil")
	}
	if c.Options().PoolSize != DefaultMaxActive {
		t.Errorf("PoolSize = %d, want default %d", c.Options().PoolSize, DefaultMaxActive)
	}
	if c.Options().MaxIdleConns != DefaultMaxIdle {
		t.Errorf("MaxIdleConns = %d, want default %d", c.Options().MaxIdleConns, DefaultMaxIdle)
	}
	if c.Options().ConnMaxIdleTime.Seconds() != float64(DefaultIdleTimeoutSeconds) {
		t.Errorf("ConnMaxIdleTime = %v, want default %d seconds", c.Options().ConnMaxIdleTime, DefaultIdleTimeoutSeconds)
	}

	c2 := NewClient("tcp", "127.0.0.1:6379", "pwd",
		WithMaxIdle(5),
		WithIdleTimeoutSeconds(30),
		WithMaxActive(42),
		WithWaitMode(),
	)
	if c2.Options().PoolSize != 42 || c2.Options().MaxIdleConns != 5 || c2.Options().Password != "pwd" {
		t.Errorf("custom options were not applied: %+v", c2.Options())
	}
}

func TestNewClientEmptyAddressPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for empty address")
		}
	}()
	NewClient("tcp", "", "")
}
