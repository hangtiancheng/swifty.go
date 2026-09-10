package concurrency

import (
	"context"
	"sync"
)

// SafeChan is a channel guarded by a context and safe single-shot Close.
type SafeChan struct {
	sync.Once
	ctx   context.Context
	close func()
	ch    chan any
}

func NewSafeChan(size int) *SafeChan {
	s := SafeChan{
		ch: make(chan any, size),
	}
	s.ctx, s.close = context.WithCancel(context.Background())
	return &s
}

// Put stores an element without blocking: it drops the element when the channel
// is full or the channel has been closed.
func (s *SafeChan) Put(element any) {
	select {
	case <-s.ctx.Done():
	case s.ch <- element:
	default:
	}
}

func (s *SafeChan) GetChan() chan any {
	return s.ch
}

func (s *SafeChan) Get() any {
	return <-s.ch
}

func (s *SafeChan) Close() {
	s.Do(func() {
		s.close()
		close(s.ch)
	})
}
