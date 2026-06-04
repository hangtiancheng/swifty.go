---
name: lark-http
description: >
  Lightweight HTTP framework (lark_http module). Use this skill when working on
  HTTP routing, middleware chains, Context request/response handling, SSE streaming,
  router groups, trie-based route matching, or any code that imports
  github.com/hangtiancheng/lark-go/lark_http. Also use it when the user asks about the
  deferred response pattern, Koa-style middleware, or lark_http's Application lifecycle.
---

# lark_http

A lightweight, Koa-inspired HTTP framework for Go with trie-based routing,
Koa-style middleware (ctx, next), deferred response rendering, Server-Sent Events
support, and router prefixing.

Module path: `github.com/hangtiancheng/lark-go/lark_http`

Source root: `lark_http/`

## Architecture overview

```
Application (top-level, implements http.Handler)
  |-- root *Router (default router, prefix="")
  |-- router (trie-based, method -> *node tree)
  |-- routers []*Router (all routers with prefix + middlewares)
  |-- htmlTemplates / funcMap

Request lifecycle:
  ServeHTTP -> collect matching router middlewares
            -> newContext(w, req)
            -> router.handle(ctx, middlewares)
                -> compose(middlewares, routeHandler)(ctx)
            -> ctx.respond()  // deferred response flush
```

The deferred response pattern means handlers set `ctx.Status`, `ctx.Body`, and `ctx.Type`
during the middleware chain, and the actual HTTP write happens once at the end in
`ctx.respond()`. This allows downstream middleware to modify the response after the
handler returns (Koa's onion model).

Middleware uses the `func(ctx *Context, next func())` signature. Call `next()` to pass
control downstream; omit the call to short-circuit.

## Core types

### Application

```go
func New() *Application
func Default() *Application   // New() + Logger() + Recovery()

func (app *Application) Listen(addr string) error
func (app *Application) Shutdown(ctx context.Context) error
func (app *Application) SetFuncMap(funcMap template.FuncMap)
func (app *Application) LoadHTMLGlob(pattern string)
func (app *Application) ServeHTTP(w http.ResponseWriter, req *http.Request)

// Route registration (delegates to root Router)
func (app *Application) Use(middlewares ...Middleware)
func (app *Application) Router(prefix string) *Router
func (app *Application) Get(pattern string, handler Middleware)
func (app *Application) Post(pattern string, handler Middleware)
func (app *Application) Put(pattern string, handler Middleware)
func (app *Application) Delete(pattern string, handler Middleware)
func (app *Application) Patch(pattern string, handler Middleware)
func (app *Application) Head(pattern string, handler Middleware)
func (app *Application) Options(pattern string, handler Middleware)
func (app *Application) All(pattern string, handler Middleware)
func (app *Application) Static(relativePath, root string)
```

### Context

Per-request context carrying the request, response writer, deferred response state,
and route params.

```go
type H map[string]interface{}
type Middleware func(ctx *Context, next func())

type Context struct {
    Request *http.Request
    Writer  http.ResponseWriter
    Path    string                   // request URL path
    Method  string                   // HTTP method
    Status  int                      // deferred status code
    Body    interface{}              // deferred response body
    Type    string                   // Content-Type override
    State   map[string]interface{}   // middleware data sharing
    Params  map[string]string        // route parameters
}
```

Middleware control:

| Pattern                  | Description                              |
| ------------------------ | ---------------------------------------- |
| call `next()`            | Advance to the next handler in the chain |
| don't call `next()`     | Skip remaining handlers (short-circuit)  |
| `ctx.Throw(status, msg)` | Set error status + body                 |

Request accessors:

| Method          | Description                               |
| --------------- | ----------------------------------------- |
| `Query(key)`    | URL query parameter                       |
| `Param(key)`    | Route path parameter (`:name` or `*name`) |
| `PostForm(key)` | Form value                                |
| `Get(header)`   | Request header                            |
| `BindJSON(out)` | Decode JSON request body                  |

Response setters (deferred -- written at end of chain):

| Method                 | Description                  |
| ---------------------- | ---------------------------- |
| `JSON(obj)`            | Set body as JSON             |
| `String(fmt, vals...)` | Set body as formatted string |
| `Data([]byte)`         | Set body as raw bytes        |
| `HTML(name, data)`     | Render named HTML template   |
| `Redirect(url)`        | 302 redirect                 |
| `Set(header, value)`   | Set response header          |

The `respond()` method (called automatically) dispatches on the `Body` type:
`htmlPayload` -> template, `[]byte` -> raw, `string` -> text, `io.Reader` -> stream copy,
default -> JSON marshal. If `ctx.flushed` is true (e.g., after SSE or static file serving),
`respond()` is skipped.

### Router

Route grouping with shared prefix and middleware stack.

```go
func (r *Router) Router(prefix string) *Router
func (r *Router) Use(middlewares ...Middleware)

// Route registration
func (r *Router) Get(pattern string, handler Middleware)
func (r *Router) Post(pattern string, handler Middleware)
func (r *Router) Put(pattern string, handler Middleware)
func (r *Router) Delete(pattern string, handler Middleware)
func (r *Router) Patch(pattern string, handler Middleware)
func (r *Router) Head(pattern string, handler Middleware)
func (r *Router) Options(pattern string, handler Middleware)
func (r *Router) All(pattern string, handler Middleware)

// Static file serving
func (r *Router) Static(relativePath, root string)
```

### SSEWriter

Server-Sent Events support, obtained via `ctx.SSE()`.

```go
func (ctx *Context) SSE() *SSEWriter
```

Calling `SSE()` sets `ctx.flushed = true`, writes SSE headers (`text/event-stream`,
`no-cache`, `keep-alive`), and returns an `SSEWriter`.

SSEWriter methods:

| Method                          | Description                                          |
| ------------------------------- | ---------------------------------------------------- |
| `Event(event, data)`            | Send a named event                                   |
| `Data(data)`                    | Send unnamed data                                    |
| `JSON(event, obj)`              | Send JSON-serialized event                           |
| `ID(id)`                        | Send event ID                                        |
| `Retry(ms)`                     | Set client retry interval                            |
| `Comment(text)`                 | Send comment (keepalive)                             |
| `Heartbeat(interval) -> stopFn` | Background keepalive goroutine                       |
| `Stream(ch <-chan string)`      | Consume a channel until closed or client disconnects |
| `Closed() -> <-chan struct{}`   | Client disconnect signal                             |
| `Flush()`                       | Manual flush                                         |
| `Done()`                        | Send `[DONE]` sentinel                               |

All write methods are mutex-protected for concurrent safety.

## Routing

The router uses a trie (prefix tree) per HTTP method.

Path parameter syntax:

- `:name` -- matches a single path segment, accessible via `ctx.Param("name")`
- `*name` -- matches the rest of the path (wildcard), accessible via `ctx.Param("name")`

Examples:

- `/users/:id` matches `/users/42` with `Param("id") == "42"`
- `/files/*filepath` matches `/files/css/style.css` with `Param("filepath") == "css/style.css"`

## Built-in middleware

### Logger

```go
func Logger() Middleware
```

Logs `[status] URI in duration` after the handler chain completes.

### Recovery

```go
func Recovery() Middleware
```

Recovers from panics, logs a stack trace, and returns 500 with
`{"message": "Internal Server Error"}`.

## Typical usage

```go
app := lark_http.Default()

api := app.Router("/api")
api.Use(authMiddleware)

api.Get("/users/:id", func(ctx *lark_http.Context, next func()) {
    id := ctx.Param("id")
    user := findUser(id)
    ctx.Status = 200
    ctx.JSON(user)
})

api.Post("/chat", func(ctx *lark_http.Context, next func()) {
    sse := ctx.SSE()
    stop := sse.Heartbeat(15 * time.Second)
    defer stop()

    for chunk := range generateResponse(ctx.Request.Context()) {
        sse.Data(chunk)
    }
    sse.Done()
})

app.Listen(":8080")
```

## File map

| File          | Purpose                                              |
| ------------- | ---------------------------------------------------- |
| `lark.go`     | Application constructor, ServeHTTP, Listen, Shutdown |
| `context.go`  | Context type, request accessors, Throw               |
| `response.go` | Deferred response methods (JSON, String, HTML, etc.) |
| `group.go`    | Router with prefix, middleware, and route registration |
| `router.go`   | Trie-based router dispatch + compose                 |
| `trie.go`     | Trie node insert/search/travel                       |
| `sse.go`      | SSEWriter for Server-Sent Events                     |
| `logger.go`   | Logger middleware                                    |
| `recovery.go` | Panic recovery middleware                            |
| `main.go`     | Example / demo entry point                           |

## Dependencies

None beyond the Go standard library.
