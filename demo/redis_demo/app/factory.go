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

package app

import (
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/database"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/datastore"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/handler"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/log"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/persist"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/protocol"
	"github.com/hangtiancheng/swifty.go/demo/redis_demo/server"

	"go.uber.org/dig"
)

var container = dig.New()

func init() {
	/**
	   Miscellaneous
	**/
	// Config loading
	_ = container.Provide(SetUpConfig)
	_ = container.Provide(PersistThinker)
	// Logger
	_ = container.Provide(log.GetDefaultLogger)

	/**
	   Storage engine
	**/
	// Persistence
	_ = container.Provide(persist.NewPersister)
	// Storage medium
	_ = container.Provide(datastore.NewKVStore)
	// Executor
	_ = container.Provide(database.NewDBExecutor)
	// Trigger
	_ = container.Provide(database.NewDBTrigger)

	/**
	   Logic layer
	**/
	// Protocol parser
	_ = container.Provide(protocol.NewParser)
	// Command handler
	_ = container.Provide(handler.NewHandler)

	/**
	   Server
	**/
	_ = container.Provide(server.NewServer)
}

func ConstructServer() (*server.Server, error) {
	var h server.Handler
	if err := container.Invoke(func(_h server.Handler) {
		h = _h
	}); err != nil {
		return nil, err
	}

	var l log.Logger
	if err := container.Invoke(func(_l log.Logger) {
		l = _l
	}); err != nil {
		return nil, err
	}
	return server.NewServer(h, l), nil
}
