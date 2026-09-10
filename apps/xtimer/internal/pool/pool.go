package pool

import (
	"sync"

	"github.com/hangtiancheng/swifty.go/apps/xtimer/internal/log"
)

// WorkerPool is a bounded goroutine pool.
type WorkerPool interface {
	Submit(func()) error
}

// GoWorkerPool runs submitted functions on goroutines, bounding the number of
// concurrently running tasks with a counting-semaphore channel. Submit blocks
// until a slot frees up, matching the behaviour of the pool implementation
// used previously.
type GoWorkerPool struct {
	sem chan struct{}
	wg  sync.WaitGroup
}

// NewGoWorkerPool creates a pool allowing at most size concurrently running tasks.
func NewGoWorkerPool(size int) *GoWorkerPool {
	if size <= 0 {
		panic("worker pool size must be positive")
	}
	return &GoWorkerPool{
		sem: make(chan struct{}, size),
	}
}

// Submit runs f on a new goroutine once a worker slot is available. Panics
// inside f are recovered and logged so a single bad task cannot take down the
// whole process.
func (g *GoWorkerPool) Submit(f func()) error {
	// Blocks until a slot is available.
	g.sem <- struct{}{}

	g.wg.Go(func() {
		defer func() {
			<-g.sem
		}()
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("recovered from panic in worker pool task: %v", r)
			}
		}()
		f()
	})
	return nil
}

// Wait blocks until every submitted task finishes.
func (g *GoWorkerPool) Wait() {
	g.wg.Wait()
}
