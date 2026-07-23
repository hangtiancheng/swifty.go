package database

import (
	"context"
	"fmt"
	"sync"

	"github.com/hangtiancheng/swifty.go/demo/redis_demo/handler"
)

type DBTrigger struct {
	once     sync.Once
	executor Executor
}

func NewDBTrigger(executor Executor) handler.DB {
	return &DBTrigger{executor: executor}
}

func (d *DBTrigger) Do(ctx context.Context, cmdLine [][]byte) handler.Reply {
	if len(cmdLine) < 2 {
		return handler.NewErrReply(fmt.Sprintf("invalid cmd line: %v", cmdLine))
	}

	cmdType := CmdType(cmdLine[0])
	if !d.executor.ValidCommand(cmdType) {
		return handler.NewErrReply(fmt.Sprintf("unknown cmd '%s'", cmdLine[0]))
	}

	cmd := Command{
		ctx:      ctx,
		cmd:      cmdType,
		args:     cmdLine[1:],
		receiver: make(CmdReceiver),
	}

	// Dispatch to the executor.
	d.executor.Entrance() <- &cmd

	// Listen on the channel until a reply is received.
	return <-cmd.Receiver()
}

func (d *DBTrigger) Close() {
	d.once.Do(d.executor.Close)
}
