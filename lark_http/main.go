//go:build ignore

package main

import (
	"net/http"

	lark "github.com/hangtiancheng/lark_http"
)

func main() {
	r := lark.Default()
	r.GET("/", func(c *lark.Context) {
		c.Status = http.StatusOK
		c.String("Hello World\n")
	})
	r.GET("/panic", func(c *lark.Context) {
		names := []string{"lark"}
		c.Status = http.StatusOK
		c.String("%s", names[100])
	})
	r.GET("/events", func(c *lark.Context) {
		sse := c.SSE()
		sse.Event("greeting", "hello")
		sse.Data("world")
		sse.Done()
	})

	r.Run(":9999")
}
