package swifty_http

import (
	"log"
	"time"
)

func Logger() Middleware {
	return func(ctx *Context, next func()) {
		t := time.Now()
		next()
		status := ctx.Status
		if ctx.flushed {
			status = 101
		}
		log.Printf("[%d] %s in %v", status, ctx.Request.RequestURI, time.Since(t))
	}
}
