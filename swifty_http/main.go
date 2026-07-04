//go:build ignore

package main

import (
	"net/http"

	swifty "github.com/hangtiancheng/swifty.go/swifty_http"
)

func main() {
	app := swifty.Default()
	app.Get("/", func(ctx *swifty.Context, next func()) {
		ctx.Status = http.StatusOK
		ctx.String("Hello World\n")
	})
	app.Get("/panic", func(ctx *swifty.Context, next func()) {
		names := []string{"swifty"}
		ctx.Status = http.StatusOK
		ctx.String("%s", names[100])
	})
	app.Get("/events", func(ctx *swifty.Context, next func()) {
		sse := ctx.SSE()
		sse.Event("greeting", "hello")
		sse.Data("world")
		sse.Done()
	})

	app.Listen(":9999")
}
