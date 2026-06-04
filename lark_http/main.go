//go:build ignore

package main

import (
	"net/http"

	lark "github.com/hangtiancheng/lark-go/lark_http"
)

func main() {
	app := lark.Default()
	app.Get("/", func(ctx *lark.Context, next func()) {
		ctx.Status = http.StatusOK
		ctx.String("Hello World\n")
	})
	app.Get("/panic", func(ctx *lark.Context, next func()) {
		names := []string{"lark"}
		ctx.Status = http.StatusOK
		ctx.String("%s", names[100])
	})
	app.Get("/events", func(ctx *lark.Context, next func()) {
		sse := ctx.SSE()
		sse.Event("greeting", "hello")
		sse.Data("world")
		sse.Done()
	})

	app.Listen(":9999")
}
