package pool

import (
	"context"
	"runtime/debug"
	"strings"

	"github.com/hangtiancheng/swifty.go/demo/redis_demo/log"

	"github.com/bytedance/gopkg/util/gopool"
)

func init() {
	gopool.SetCap(50000)
	gopool.SetPanicHandler(func(_ context.Context, e interface{}) {
		stackInfo := strings.Replace(string(debug.Stack()), "\n", "", -1)
		log.GetDefaultLogger().Errorf("recover info: %v, stack info: %s", e, stackInfo)
	})
}

func Submit(task func()) {
	gopool.Go(task)
}
