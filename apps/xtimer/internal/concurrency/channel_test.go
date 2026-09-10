package concurrency

import "testing"

func TestSafeChanPutGet(t *testing.T) {
	s := NewSafeChan(2)
	defer s.Close()

	s.Put(1)
	s.Put("two")

	if got := s.Get(); got != 1 {
		t.Fatalf("first Get = %v, want 1", got)
	}
	if got := s.Get(); got != "two" {
		t.Fatalf(`second Get = %v, want "two"`, got)
	}
}

func TestSafeChanDropsWhenFull(t *testing.T) {
	s := NewSafeChan(1)
	defer s.Close()

	s.Put(1)
	s.Put(2) // dropped because the channel is full

	if got := s.Get(); got != 1 {
		t.Fatalf("Get = %v, want 1", got)
	}
	select {
	case got := <-s.GetChan():
		t.Fatalf("overflowed element was dropped, got %v", got)
	default:
	}
}

func TestSafeChanPutAfterClose(t *testing.T) {
	s := NewSafeChan(1)
	s.Close()

	// Must neither panic nor block.
	s.Put(1)
}

func TestSafeChanCloseIsIdempotent(t *testing.T) {
	s := NewSafeChan(1)
	s.Close()
	s.Close()
}
