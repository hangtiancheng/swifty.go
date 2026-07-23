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

package database

import (
	"context"
	"fmt"
	"time"

	"github.com/hangtiancheng/swifty.go/demo/redis_demo/handler"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/lib/pool"
)

type DBExecutor struct {
	ctx    context.Context
	cancel context.CancelFunc
	ch     chan *Command

	cmdHandlers map[CmdType]CmdHandler
	dataStore   DataStore

	gcTicker *time.Ticker
}

func NewDBExecutor(dataStore DataStore) Executor {
	ctx, cancel := context.WithCancel(context.Background())
	e := DBExecutor{
		dataStore: dataStore,
		ch:        make(chan *Command),
		ctx:       ctx,
		cancel:    cancel,
		gcTicker:  time.NewTicker(time.Minute),
	}
	e.cmdHandlers = map[CmdType]CmdHandler{
		CmdTypeExpire:   e.dataStore.Expire,
		CmdTypeExpireAt: e.dataStore.ExpireAt,

		// string
		CmdTypeGet:  e.dataStore.Get,
		CmdTypeSet:  e.dataStore.Set,
		CmdTypeMGet: e.dataStore.MGet,
		CmdTypeMSet: e.dataStore.MSet,

		// list
		CmdTypeLPush:  e.dataStore.LPush,
		CmdTypeLPop:   e.dataStore.LPop,
		CmdTypeRPush:  e.dataStore.RPush,
		CmdTypeRPop:   e.dataStore.RPop,
		CmdTypeLRange: e.dataStore.LRange,

		// set
		CmdTypeSAdd:      e.dataStore.SAdd,
		CmdTypeSIsMember: e.dataStore.SIsMember,
		CmdTypeSRem:      e.dataStore.SRem,

		// hash
		CmdTypeHSet: e.dataStore.HSet,
		CmdTypeHGet: e.dataStore.HGet,
		CmdTypeHDel: e.dataStore.HDel,

		// sorted set
		CmdTypeZAdd:          e.dataStore.ZAdd,
		CmdTypeZRangeByScore: e.dataStore.ZRangeByScore,
		CmdTypeZRem:          e.dataStore.ZRem,
	}

	pool.Submit(e.run)
	return &e
}

func (e *DBExecutor) Entrance() chan<- *Command {
	return e.ch
}

func (e *DBExecutor) ValidCommand(cmd CmdType) bool {
	_, valid := e.cmdHandlers[cmd] // map is read-only; no concurrency concern
	return valid
}

func (e *DBExecutor) Close() {
	e.cancel()
}

func (e *DBExecutor) run() {
	for {
		select {
		case <-e.ctx.Done():
			return

		// Batch-expire keys every 1 minute.
		case <-e.gcTicker.C:
			e.dataStore.GC()

		case cmd := <-e.ch:
			cmdFunc, ok := e.cmdHandlers[cmd.cmd]
			if !ok {
				cmd.receiver <- handler.NewErrReply(fmt.Sprintf("unknown command '%s'", cmd.cmd))
				continue
			}

			e.dataStore.ExpirePreprocess(string(cmd.args[0])) // lazy deletion of expired keys
			cmd.receiver <- cmdFunc(cmd)
		}
	}
}
