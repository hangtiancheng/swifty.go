package load_balance

import (
	"sync"
	"testing"

	"github.com/hangtiancheng/swifty.go/swifty_rpc/internal/registry"
)

func instances() []registry.Instance {
	return []registry.Instance{{Addr: "a"}, {Addr: "b"}, {Addr: "c"}}
}

func TestRoundRobin(t *testing.T) {
	rr := NewRR()
	if got := rr.Select(nil); got.Addr != "" {
		t.Fatalf("empty select = %+v", got)
	}
	list := instances()
	want := []string{"a", "b", "c", "a"}
	for _, addr := range want {
		if got := rr.Select(list).Addr; got != addr {
			t.Fatalf("Select = %q, want %q", got, addr)
		}
	}
}

func TestRandom(t *testing.T) {
	r := NewRandom()
	if got := r.Select(nil); got.Addr != "" {
		t.Fatalf("empty select = %+v", got)
	}
	list := instances()
	for i := 0; i < 20; i++ {
		if got := r.Select(list).Addr; got == "" {
			t.Fatal("random select returned empty instance")
		}
	}
}

func TestWeightedRR(t *testing.T) {
	list := instances()
	w := NewWeightedRR([]int{1, 2, 3})
	got := make(map[string]int)
	for i := 0; i < 6; i++ {
		got[w.Select(list).Addr]++
	}
	if got["a"] != 1 || got["b"] != 2 || got["c"] != 3 {
		t.Fatalf("weighted distribution = %v", got)
	}
	if got := NewWeightedRR([]int{1}).Select(list); got.Addr != "" {
		t.Fatalf("mismatched weight count returned %+v", got)
	}
	if got := NewWeightedRR([]int{0, -1, 0}).Select(list); got.Addr != "" {
		t.Fatalf("zero weights returned %+v", got)
	}
}

func TestConcurrentBalancers(t *testing.T) {
	rr := NewRR()
	random := NewRandom()
	list := instances()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = rr.Select(list)
		}()
		go func() {
			defer wg.Done()
			_ = random.Select(list)
		}()
	}
	wg.Wait()
}
