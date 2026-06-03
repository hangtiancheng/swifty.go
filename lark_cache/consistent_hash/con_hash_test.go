package consistent_hash

import (
	"strconv"
	"testing"
	"time"
)

func TestMapAddGetRemoveAndStats(t *testing.T) {
	m := New(WithConfig(&Config{
		DefaultReplicas:      3,
		MinReplicas:          1,
		MaxReplicas:          10,
		LoadBalanceThreshold: 0.25,
		HashFunc: func(data []byte) uint32 {
			i, _ := strconv.Atoi(string(data))
			return uint32(i)
		},
	}))

	if got := m.Get("1"); got != "" {
		t.Fatalf("empty ring returned %q", got)
	}
	if err := m.Add(); err == nil {
		t.Fatal("Add without nodes should fail")
	}
	if err := m.Add("6", "4", "2", ""); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	for _, key := range []string{"2", "11", "23", "27"} {
		if got := m.Get(key); got == "" {
			t.Fatalf("Get(%q) returned an empty node", key)
		}
	}
	stats := m.GetStats()
	if len(stats) == 0 {
		t.Fatal("expected stats after Get calls")
	}
	if err := m.Remove("4"); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if err := m.Remove("missing"); err == nil {
		t.Fatal("Remove missing node should fail")
	}
	if err := m.Remove(""); err == nil {
		t.Fatal("Remove empty node should fail")
	}
}

func TestMapRebalanceDoesNotDeadlock(t *testing.T) {
	m := New(WithConfig(&Config{
		DefaultReplicas:      5,
		MinReplicas:          2,
		MaxReplicas:          20,
		LoadBalanceThreshold: 0.01,
		HashFunc:             DefaultConfig.HashFunc,
	}))
	if err := m.Add("a", "b", "c"); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	for range 1200 {
		_ = m.Get("key-a")
	}

	done := make(chan struct{})
	go func() {
		m.checkAndRebalance()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("checkAndRebalance deadlocked")
	}
	if got := m.Get("key"); got == "" {
		t.Fatal("ring returned empty node after rebalance")
	}
}
