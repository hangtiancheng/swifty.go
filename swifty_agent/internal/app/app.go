// Package app provides the HTTP application layer for the Swifty Chatbot.
// It wires up routes, middleware, and request handlers using the swifty_http framework.
package app

import (
	"net/http"

	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
	"github.com/hangtiancheng/swifty.go/swifty_http"
)

// App encapsulates the HTTP application and its dependencies.
type App struct {
	engine *swifty_http.Application
	cfg    *config.Config
}

// New creates a new App with all routes and middleware configured.
func New(cfg *config.Config) *App {
	engine := swifty_http.Default()
	engine.Use(corsMiddleware)

	a := &App{engine: engine, cfg: cfg}

	api := engine.Router("/api")
	api.Post("/chat", a.handleChat)
	api.Post("/chat_stream", a.handleChatStream)
	api.Post("/upload", a.handleFileUpload)
	api.Post("/ai_ops", a.handleAIOps)

	return a
}

// Engine returns the underlying swifty_http Application for starting the server.
func (a *App) Engine() *swifty_http.Application {
	return a.engine
}

// corsMiddleware adds CORS headers to all responses and handles preflight requests.
func corsMiddleware(ctx *swifty_http.Context, next func()) {
	ctx.Set("Access-Control-Allow-Origin", "*")
	ctx.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH,OPTIONS")
	ctx.Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
	if ctx.Method == http.MethodOptions {
		ctx.Status = http.StatusNoContent
		return
	}
	next()
}
