package redis

import (
	"testing"
	"time"
)

// TestCacheDisableKey verifies the disable key mapping.
func TestCacheDisableKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{
			name: "regular key",
			key:  "user:1",
			want: "Enable_Lock_Key_{user:1}",
		},
		{
			name: "empty key",
			key:  "",
			want: "Enable_Lock_Key_{}",
		},
	}

	c := &Cache{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.disableKey(tt.key); got != tt.want {
				t.Errorf("disableKey(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

// TestDurationConversions verifies the duration helpers.
func TestDurationConversions(t *testing.T) {
	if got, want := secondsToDuration(30), 30*time.Second; got != want {
		t.Errorf("secondsToDuration(30) = %v, want %v", got, want)
	}
	if got, want := millisToDuration(1500), 1500*time.Millisecond; got != want {
		t.Errorf("millisToDuration(1500) = %v, want %v", got, want)
	}
}
