// Copyright (c) 2026 hangtiancheng
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package persist

import (
	"context"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hangtiancheng/swifty.go/demo/redis_demo/handler"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/lib/pool"
)

// always | everysec | no
type appendSyncStrategy string

func (a appendSyncStrategy) string() string {
	return string(a)
}

const (
	alwaysAppendSyncStrategy   appendSyncStrategy = "always"
	everysecAppendSyncStrategy appendSyncStrategy = "everysec"
	noAppendSyncStrategy       appendSyncStrategy = "no"
)

type aofPersister struct {
	ctx    context.Context
	cancel context.CancelFunc

	buffer                 chan [][]byte
	aofFile                *os.File
	aofFileName            string
	appendFsync            appendSyncStrategy
	autoAofRewriteAfterCmd int64
	aofCounter             atomic.Int64

	mu   sync.Mutex
	once sync.Once
}

func newAofPersister(thinker Thinker) (handler.Persister, error) {
	aofFileName := thinker.AppendFileName()
	aofFile, err := os.OpenFile(aofFileName, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := aofPersister{
		ctx:         ctx,
		cancel:      cancel,
		buffer:      make(chan [][]byte, 1<<10),
		aofFile:     aofFile,
		aofFileName: aofFileName,
	}

	if autoAofRewriteAfterCmd := thinker.AutoAofRewriteAfterCmd(); autoAofRewriteAfterCmd > 1 {
		a.autoAofRewriteAfterCmd = int64(autoAofRewriteAfterCmd)
	}

	switch thinker.AppendFsync() {
	case alwaysAppendSyncStrategy.string():
		a.appendFsync = alwaysAppendSyncStrategy
	case everysecAppendSyncStrategy.string():
		a.appendFsync = everysecAppendSyncStrategy
	default:
		a.appendFsync = noAppendSyncStrategy // default strategy
	}

	pool.Submit(a.run)
	return &a, nil
}

func (a *aofPersister) Reloader() (io.ReadCloser, error) {
	file, err := os.Open(a.aofFileName)
	if err != nil {
		return nil, err
	}
	_, _ = file.Seek(0, io.SeekStart)
	return file, nil
}

func (a *aofPersister) PersistCmd(ctx context.Context, cmd [][]byte) {
	if handler.IsLoadingPattern(ctx) {
		return
	}
	a.buffer <- cmd
}

func (a *aofPersister) Close() {
	a.once.Do(func() {
		a.cancel()
		_ = a.aofFile.Close()
	})
}

func (a *aofPersister) run() {
	if a.appendFsync == everysecAppendSyncStrategy {
		pool.Submit(a.fsyncEverySecond)
	}

	for {
		select {
		case <-a.ctx.Done():
			// log
			return
		case cmd := <-a.buffer:
			a.writeAof(cmd)
			a.aofTick()
		}
	}
}

// aofTick tracks the number of AOF commands executed.
func (a *aofPersister) aofTick() {
	if a.autoAofRewriteAfterCmd <= 1 {
		return
	}

	if ticked := a.aofCounter.Add(1); ticked < int64(a.autoAofRewriteAfterCmd) {
		return
	}

	// Threshold reached: deduct the counter and trigger a rewrite.
	_ = a.aofCounter.Add(-a.autoAofRewriteAfterCmd)
	pool.Submit(func() {
		if err := a.rewriteAOF(); err != nil {
			// log
		}
	})
}

func (a *aofPersister) fsyncEverySecond() {
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-a.ctx.Done():
			// log
			return
		case <-ticker.C:
			if err := a.fsync(); err != nil {
				// log
			}
		}
	}
}

func (a *aofPersister) writeAof(cmd [][]byte) {
	a.mu.Lock()
	defer a.mu.Unlock()

	persistCmd := handler.NewMultiBulkReply(cmd)
	if _, err := a.aofFile.Write(persistCmd.ToBytes()); err != nil {
		// log
		return
	}

	if a.appendFsync != alwaysAppendSyncStrategy {
		return
	}

	if err := a.fsyncLocked(); err != nil {
		// log
	}
}

func (a *aofPersister) fsync() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.fsyncLocked()
}

func (a *aofPersister) fsyncLocked() error {
	return a.aofFile.Sync()
}
