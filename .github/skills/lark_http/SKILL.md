---
name: lark-http
description: >
  Lightweight HTTP framework (lark_http module). Use this skill when working on
  HTTP routing, middleware chains, Context request/response handling, SSE streaming,
  router groups, trie-based route matching, or any code that imports
  github.com/hangtiancheng/lark_http. Also use it when the user asks about the
  deferred response pattern, Koa-style middleware, or lark_http's Engine lifecycle.
---

# lark_http

A lightweight, Koa-inspired HTTP framework for Go with trie-based routing,
middleware chains, deferred response rendering, Server-Sent Events support,
and router group prefixing.

Module path: `github.com/hangtiancheng/lark_http`

Source root: `lark_http/`

## Architecture overview

```
Engine (embeds *RouterGroup, implements http.Handler)
  |-- router (trie-based, method -> *node tree)
  |-- groups []*RouterGroup (prefix + middlewares)
  |-- htmlTemplates / funcMap

Request lifecycle:
  ServeHTTP -> collect matching group middlewares
            -> newContext(w, req)
            -> router.handle(c)  // appends route handler
            -> c.Next()          // runs middleware chain
            -> c.respond()       // deferred response flush
```

The deferred response pattern means handlers set `c.Status`, `c.Body`, and `c.Type`
during the middleware chain, and the actual HTTP write happens once at the end in
`c.respond()`. This allows downstream middleware to modify the response after the
handler returns (like Koa's onion model).

## Core types

### Engine

```go
func New() *Engine
func Default() *Engine   // New() + Logger() + Recovery()

func (e *Engine) Run(addr string) error
func (e *Engine) Shutdown(ctx context.Context) error
func (e *Engine) SetFuncMap(funcMap template.FuncMap)
func (e *Engine) LoadHTMLGlob(pattern string)
func (e *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request)
```

`Engine` embeds `*RouterGroup`, so all route registration methods are available
directly on the engine.

### Context

Per-request context carrying the request, response writer, deferred response state,
and middleware chain.

```go
type H map[string]interface{}
type HandlerFunc func(*Context)

type Context struct {
    Req    *http.Request
    Writer http.ResponseWriter
    Status int                      // deferred status code
    Body   interface{}              // deferred response body
    Type   string                   // Content-Type override
    State  map[string]interface{}   // middleware data sharing
    Params map[string]string        // route parameters
}
```

Middleware control:

| Method               | Description                              |
| -------------------- | ---------------------------------------- |
| `Next()`             | Advance to the next handler in the chain |
| `Abort()`            | Skip remaining handlers                  |
| `Throw(status, msg)` | Set error status + body + abort          |

Request accessors:

| Method          | Description                               |
| --------------- | ----------------------------------------- |
| `Query(key)`    | URL query parameter                       |
| `Param(key)`    | Route path parameter (`:name` or `*name`) |
| `PostForm(key)` | Form value                                |
| `Get(header)`   | Request header                            |
| `BindJSON(out)` | Decode JSON request body                  |
| `Path()`        | Request URL path                          |
| `Method()`      | HTTP method                               |

Response setters (deferred -- written at end of chain):

| Method                 | Description                  |
| ---------------------- | ---------------------------- |
| `JSON(obj)`            | Set body as JSON             |
| `String(fmt, vals...)` | Set body as formatted string |
| `Data([]byte)`         | Set body as raw bytes        |
| `HTML(name, data)`     | Render named HTML template   |
| `Redirect(url)`        | 302 redirect                 |
| `SetStatus(code)`      | Override status code         |
| `Set(header, value)`   | Set response header          |

The `respond()` method (called automatically) dispatches on the `Body` type:
`htmlPayload` -> template, `[]byte` -> raw, `string` -> text, `io.Reader` -> stream copy,
default -> JSON marshal. If `c.flushed` is true (e.g., after SSE or static file serving),
`respond()` is skipped.

### RouterGroup

Route grouping with shared prefix and middleware stack.

```go
func (g *RouterGroup) Group(prefix string) *RouterGroup
func (g *RouterGroup) Use(middlewares ...HandlerFunc)

// Route registration (all HTTP methods)
func (g *RouterGroup) GET(pattern string, handler HandlerFunc)
func (g *RouterGroup) POST(pattern string, handler HandlerFunc)
func (g *RouterGroup) PUT(pattern string, handler HandlerFunc)
func (g *RouterGroup) DELETE(pattern string, handler HandlerFunc)
func (g *RouterGroup) PATCH(pattern string, handler HandlerFunc)
func (g *RouterGroup) HEAD(pattern string, handler HandlerFunc)
func (g *RouterGroup) OPTIONS(pattern string, handler HandlerFunc)
func (g *RouterGroup) Any(pattern string, handler HandlerFunc)

// Static file serving
func (g *RouterGroup) Static(relativePath, root string)
```

### SSEWriter

Server-Sent Events support, obtained via `c.SSE()`.

```go
func (c *Context) SSE() *SSEWriter
```

Calling `SSE()` sets `c.flushed = true`, writes SSE headers (`text/event-stream`,
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

- `:name` -- matches a single path segment, accessible via `c.Param("name")`
- `*name` -- matches the rest of the path (wildcard), accessible via `c.Param("name")`

Examples:

- `/users/:id` matches `/users/42` with `Param("id") == "42"`
- `/files/*filepath` matches `/files/css/style.css` with `Param("filepath") == "css/style.css"`

## Built-in middleware

### Logger

```go
func Logger() HandlerFunc
```

Logs `[status] URI in duration` after the handler chain completes.

### Recovery

```go
func Recovery() HandlerFunc
```

Recovers from panics, logs a stack trace, and returns 500 with
`{"message": "Internal Server Error"}`.

## Typical usage

```go
e := lark_http.Default()

api := e.Group("/api")
api.Use(authMiddleware)

api.GET("/users/:id", func(c *lark_http.Context) {
    id := c.Param("id")
    user := findUser(id)
    c.SetStatus(200)
    c.JSON(user)
})

api.POST("/chat", func(c *lark_http.Context) {
    sse := c.SSE()
    stop := sse.Heartbeat(15 * time.Second)
    defer stop()

    for chunk := range generateResponse(c.Req.Context()) {
        sse.Data(chunk)
    }
    sse.Done()
})

e.Run(":8080")
```

## File map

| File          | Purpose                                                     |
| ------------- | ----------------------------------------------------------- |
| `lark.go`     | Engine constructor, ServeHTTP, Run, Shutdown                |
| `context.go`  | Context type, middleware control, request accessors         |
| `response.go` | Deferred response methods (JSON, String, HTML, etc.)        |
| `group.go`    | RouterGroup with prefix, middleware, and route registration |
| `router.go`   | Trie-based router dispatch                                  |
| `trie.go`     | Trie node insert/search/travel                              |
| `sse.go`      | SSEWriter for Server-Sent Events                            |
| `logger.go`   | Logger middleware                                           |
| `recovery.go` | Panic recovery middleware                                   |
| `main.go`     | Example / demo entry point                                  |

## Dependencies

None beyond the Go standard library.
