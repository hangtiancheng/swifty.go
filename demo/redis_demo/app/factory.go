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
