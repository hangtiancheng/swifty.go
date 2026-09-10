package pool

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGoWorkerPoolBoundsConcurrency(t *testing.T) {
	const size = 3
	p := NewGoWorkerPool(size)

	var active, maxActive atomic.Int64
	for range 30 {
		if err := p.Submit(func() {
			cur := active.Add(1)
			for {
				seen := maxActive.Load()
				if cur <= seen || maxActive.CompareAndSwap(seen, cur) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			active.Add(-1)
		}); err != nil {
			t.Fatalf("Submit: %v", err)
		}
	}
	p.Wait()

	if got := maxActive.Load(); got > size {
		t.Fatalf("max concurrent tasks = %d, want <= %d", got, size)
	}
}

func TestGoWorkerPoolRecoversPanic(t *testing.T) {
	p := NewGoWorkerPool(2)

	done := make(chan struct{})
	if err := p.Submit(func() {
		defer close(done)
		panic("boom")
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	<-done
	p.Wait()

	// The pool must stay usable after a recovered panic.
	if err := p.Submit(func() {}); err != nil {
		t.Fatalf("Submit after panic: %v", err)
	}
	p.Wait()
}

func TestNewGoWorkerPoolRejectsNonPositiveSize(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected a panic for a non-positive pool size")
		}
	}()
	NewGoWorkerPool(0)
}

func TestGoWorkerPoolWaitBlocksUntilDone(t *testing.T) {
	p := NewGoWorkerPool(4)

	var wg sync.WaitGroup
	wg.Go(func() {
		time.Sleep(50 * time.Millisecond)
	})
	if err := p.Submit(func() {
		wg.Wait()
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	finished := make(chan struct{})
	go func() {
		p.Wait()
		close(finished)
	}()
	select {
	case <-finished:
		t.Fatal("Wait returned before the task finished")
	case <-time.After(20 * time.Millisecond):
	}
	p.Wait()
}
