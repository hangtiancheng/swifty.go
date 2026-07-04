---
name: swifty-http
description: >
  Lightweight, Koa-inspired HTTP framework implemented in the `swifty_http` package at
  `github.com/hangtiancheng/swifty.go/swifty_http`. Use this skill when working on HTTP
  routing, middleware chains, the `*Context` request/response API, Server-Sent Events
  streaming, WebSocket connections, router groups with prefixes, trie-based path
  matching, the deferred response pattern (`ctx.Status` / `ctx.Body` / `ctx.Type`),
  or the `Application` lifecycle (`Listen`, `Shutdown`). Strong trigger tokens include
  `swifty_http`, `swifty.New()`, `swifty.Default()`, `app.Use(`, `app.Router(`,
  `ctx.SSE()`, `ctx.Upgrade(`, `WSConn`, `ctx.JSON(`, `ctx.Throw(`, `ctx.BindJSON(`,
  `swifty.H{`, `swifty.Middleware`, `*swifty.Context`, `Logger()`, `Recovery()`,
  `TextMessage`, `BinaryMessage`, `ErrWSClosed`, and the import path
  `github.com/hangtiancheng/swifty.go/swifty_http`.

  Do NOT use this skill for: plain `net/http` servers with no `swifty_http` import;
  other Go HTTP frameworks such as `gin`, `echo`, `fiber`, `chi`, or `gorilla/mux`;
  reverse-proxy or load-balancer configuration (nginx, envoy, traefik); Node.js
  Koa code; gRPC services (use the `swifty-rpc` skill); or generic Go questions that
  do not touch the framework.
---

# swifty_http

A lightweight, Koa-inspired HTTP framework for Go with trie-based routing,
Koa-style middleware (ctx, next), deferred response rendering, Server-Sent Events
support, a zero-dependency WebSocket implementation with event-driven and
request/response APIs, and router prefixing.

Module path: `github.com/hangtiancheng/swifty.go/swifty_http`

Source root: `swifty_http/`

## Architecture overview

```
Application (top-level, implements http.Handler)
  |-- root *Router         (default router, prefix="")
  |-- router               (per-method trie: method -> *node)
  |-- routers []*Router    (every registered Router, in declaration order)
  |-- htmlTemplates / funcMap

Request lifecycle:
  ServeHTTP -> walk app.routers in declaration order
            -> for each router whose prefix matches via matchRouterPath,
               append its middleware to the chain (parents first, children after)
            -> newContext(w, req); ctx.app = app
            -> router.handle(ctx, middlewares)
                -> resolve route via trie; synthesize a notFound handler on miss
                -> compose(middlewares, routeHandler)(ctx)
            -> ctx.respond()                 // skipped when ctx.flushed == true
                                             // (set by ctx.SSE(), ctx.Upgrade(),
                                             //  and Router.Static)

WebSocket lifecycle:
  ctx.Upgrade(opts) -> validate Connection/Upgrade/Key headers
                    -> negotiate subprotocol (optional)
                    -> hijack the net.Conn via http.NewResponseController
                    -> set ctx.flushed = true
                    -> write 101 handshake response directly to conn
                    -> return *WSConn
  WSConn.Listen()   -> blocking read loop dispatching to OnMessage/OnClose/OnError
  WSConn.ReadMessage() -> manual per-message read (alternative to Listen)
```

Key invariants an agent must respect:

- Handlers and middleware do not write to `ctx.Writer` directly. They set
  `ctx.Status`, `ctx.Body`, `ctx.Type`, and call `ctx.Set(header, value)`. The
  actual HTTP write happens once in `ctx.respond()` at the end of the chain.
  This is what enables Koa's onion model: code after `next()` in an outer
  middleware sees the response the inner handler chose, and can mutate it.
- Middleware signature is `func(ctx *Context, next func())`. Call `next()` to
  pass control downstream; omit the call to short-circuit the remaining chain.
- Calling `next()` more than once inside a single middleware is currently
  allowed and will execute the downstream chain multiple times. Do not rely on
  Koa's single-call guarantee.
- `ctx.flushed = true` tells `respond()` to skip rendering. `SSE()`, `Upgrade()`,
  and `Static` set this flag because they own the bytes themselves.

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
type H map[string]interface{}            // convenience alias for inline JSON literals
type Middleware func(ctx *Context, next func())

type Context struct {
    Request *http.Request
    Writer  http.ResponseWriter
    Path    string                   // request URL path
    Method  string                   // HTTP method
    Status  int                      // deferred status code (default http.StatusNotFound)
    Body    interface{}              // deferred response body; nil means no body
    Type    string                   // Content-Type for the deferred response
    State   map[string]interface{}   // per-request key/value bag for middleware
    Params  map[string]string        // route parameters captured by :name / *name
}
```

Field-level rules an agent must honor:

- `Status` defaults to `http.StatusNotFound`. If `Body` is set and `Status` was
  never explicitly assigned, `respond()` promotes it to `http.StatusOK`. Do not
  emit defensive resets such as `ctx.Status = 0`; the framework tracks an
  internal `statusSet` flag and you will break the auto-200 rule.
- `Type` is set automatically by `JSON` (`application/json`), `String`
  (`text/plain`), and `HTML` (`text/html`). Assign it directly only when you
  need to override the inferred Content-Type before the chain completes.
- `Body` accepts `htmlPayload`, `[]byte`, `string`, `io.Reader`, or any
  JSON-serializable value. The runtime selects the renderer by type switch.
- `State` is plain `map[string]interface{}` with no internal locking. If you
  fan out goroutines from a handler (for example to run multiple SSE producers),
  synchronize access yourself.
- `Params` is populated by the router before the chain starts; treat it as
  read-only.

Middleware control:

| Pattern                  | Description                                                 |
| ------------------------ | ----------------------------------------------------------- |
| call `next()`            | Advance to the next handler in the chain                    |
| omit `next()`            | Short-circuit the remaining chain                           |
| `ctx.Throw(status, msg)` | Set status and a JSON `{"message": msg}` body; marks status |

`ctx.Throw` also sets the internal `statusSet` flag, so a subsequent response
setter that does not touch `Status` will keep the throw status. To overwrite
the body after a throw, assign `ctx.Status` explicitly.

Request accessors:

| Method                                                                | Description                                                                       |
| --------------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| `Query(key string) string`                                            | URL query parameter                                                               |
| `Param(key string) string`                                            | Route path parameter (`:name` or `*name`)                                         |
| `PostForm(key string) string`                                         | Form value (`application/x-www-form-urlencoded` or `multipart/form-data`)         |
| `Get(header string) string`                                           | Request header                                                                    |
| `BindJSON(out interface{}) error`                                     | Decode JSON request body via `json.NewDecoder`; closes `Request.Body` via `defer` |
| `FormFile(key string) (multipart.File, *multipart.FileHeader, error)` | Retrieve uploaded file from multipart form                                        |

`BindJSON` is destructive: it consumes and closes the request body. Call it at
most once per request and do not expect downstream middleware to re-read the
body. There is no first-class accessor for the raw body; if you need it, read
`ctx.Request.Body` yourself before any `BindJSON` call.

Response setters (deferred -- written by `respond()` at end of chain):

| Method                                         | Description                                                                             |
| ---------------------------------------------- | --------------------------------------------------------------------------------------- |
| `JSON(obj interface{})`                        | Sets `Type = "application/json"` and `Body = obj`                                       |
| `String(format string, values ...interface{})` | Sets `Type = "text/plain"` and `Body = fmt.Sprintf(format, values...)`                  |
| `Data(data []byte)`                            | Sets `Body = bytes`; Content-Type is whatever `Type` already holds (may be empty)       |
| `HTML(name string, data interface{})`          | Sets `Type = "text/html"` and `Body = htmlPayload{name, data}`; requires `LoadHTMLGlob` |
| `Redirect(url string)`                         | Sets `Status = 302` and the `Location` header; always 302, overwrites any prior status  |
| `Set(header, value string)`                    | Buffers a response header; flushed by `respond()` or by `ctx.SSE()`                     |

`respond()` dispatches on the runtime type of `Body`:

- `htmlPayload` -> renders the named template via `app.htmlTemplates`
- `[]byte` -> written verbatim
- `string` -> written as UTF-8 text
- `io.Reader` -> copied to the writer via `io.Copy`
- anything else -> `json.Marshal` and write as JSON

`respond()` returns early if `ctx.flushed == true`. `ctx.SSE()`, `ctx.Upgrade()`,
and `Router.Static` set this flag because they own the response bytes. Do not call
response setters after either of those paths takes over.

### Router

Route grouping with a shared path prefix and its own middleware stack.

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
func (r *Router) All(pattern string, handler Middleware)   // registers GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS

// Static file serving
func (r *Router) Static(relativePath, root string)
```

Grouping rules:

- `Router(prefix)` concatenates the new prefix onto the parent's prefix and
  appends the new router to `app.routers`. Middleware stacks are independent
  per Router; a parent's `Use(...)` does not retroactively attach to children
  that were created earlier, but the request-time collector walks all routers
  whose prefix matches the path and concatenates their middleware in
  declaration order.
- Prefix matching is path-segment aware: a Router with prefix `/v1` matches
  exactly `/v1` and any path that starts with `/v1/`. It will not match
  `/v10`. This is enforced by `matchRouterPath` and is the main reason to use
  Router grouping rather than ad-hoc path checks in middleware.
- Registering the same `method + pattern` twice silently overwrites the first
  handler. There is no duplicate detection; do not rely on the second call
  failing.
- `Static(relativePath, root)` mounts an `http.FileServer` under
  `path.Join(r.prefix, relativePath, "/*filepath")` as a GET route. Headers
  set via `ctx.Set(...)` before the static handler runs are not flushed to
  the underlying writer; if you need custom headers on static responses,
  serve the file yourself or wrap the static route in middleware that writes
  directly to `ctx.Writer.Header()`. A missing file produces status 404 with
  an empty body (the response body is not populated by the static handler).

### SSEWriter

Server-Sent Events support, obtained via `ctx.SSE()`.

```go
func (ctx *Context) SSE() *SSEWriter
```

`SSE()` performs the following side effects in order:

1. Type-asserts `ctx.Writer` to `http.Flusher`; if the assertion fails (for
   example under `httptest.ResponseRecorder` without a flusher shim), the
   writer is stored as `nil` and `Flush` becomes a no-op. Writes still
   succeed but may be buffered until the handler returns. If your test
   harness needs eager delivery, wrap the recorder in a flusher or run
   against a real `http.Server`.
2. Sets `ctx.flushed = true`, telling `respond()` to skip rendering.
3. Sets `Content-Type: text/event-stream`, `Cache-Control: no-cache`, and
   `Connection: keep-alive` on the underlying writer.
4. Flushes any headers previously buffered via `ctx.Set(...)`.
5. Writes status `200 OK` via `WriteHeader` and returns the `SSEWriter`.

Call `ctx.SSE()` inside the handler, after any `ctx.Set(...)` calls you want
flushed and before any response setter such as `ctx.JSON`; mixing SSE with
the deferred response API is unsupported.

SSEWriter methods:

```go
func (w *SSEWriter) Event(event string, data string)
func (w *SSEWriter) Data(data string)
func (w *SSEWriter) JSON(event string, obj interface{})
func (w *SSEWriter) ID(id string)
func (w *SSEWriter) Retry(ms int)
func (w *SSEWriter) Comment(text string)
func (w *SSEWriter) Heartbeat(interval time.Duration) func()
func (w *SSEWriter) Stream(ch <-chan string)
func (w *SSEWriter) Closed() <-chan struct{}
func (w *SSEWriter) Flush()
func (w *SSEWriter) Done()
```

| Method                                | Description                                                                |
| ------------------------------------- | -------------------------------------------------------------------------- |
| `Event(event, data string)`           | Writes `event: <event>\n` followed by one or more `data:` lines and `\n\n` |
| `Data(data string)`                   | Writes the data block; multiline input becomes multiple `data:` lines      |
| `JSON(event string, obj interface{})` | `json.Marshal` then write; if `event == ""` the `event:` line is omitted   |
| `ID(id string)`                       | Writes `id: <id>\n` (a field line, not a complete event)                   |
| `Retry(ms int)`                       | Writes `retry: <ms>\n\n`                                                   |
| `Comment(text string)`                | Writes one `: ` comment line per `\n`-separated input line                 |
| `Heartbeat(interval) func()`          | Spawns a goroutine that writes `: keepalive\n\n` every `interval`          |
| `Stream(ch <-chan string)`            | Forwards channel values via `Data` until `ch` closes or client disconnects |
| `Closed() <-chan struct{}`            | Returns `ctx.Request.Context().Done()` for client-disconnect detection     |
| `Flush()`                             | Manually invokes the underlying `http.Flusher`                             |
| `Done()`                              | Writes `data: [DONE]\n\n`, the conventional OpenAI-compatible sentinel     |

Concurrency and lifetime contracts:

- All write methods are guarded by an internal `sync.Mutex`. It is safe to
  call `SSEWriter` methods from multiple goroutines (for example a producer
  goroutine plus the heartbeat goroutine).
- `Heartbeat`'s returned stop function is not idempotent. Calling it more
  than once will panic with `close of closed channel`. Call it exactly once,
  typically via `defer stop()`.
- `ID` and `Retry` write standalone field lines. SSE clients dispatch an
  event only after a `data:` field is followed by a blank line. Call `ID`
  immediately before the `Event`, `Data`, or `JSON` call you want it bundled
  with; do not expect `ID` alone to deliver to the client.
- `Stream` returns when `ch` closes or when `ctx.Request.Context().Done()`
  fires. It does not drain `ch` or cancel the upstream producer. When you
  consume `sse.Closed()`, cancel your producer explicitly to avoid goroutine
  leaks.
- `Event` does not check for an empty `event` argument; pass a non-empty
  name or use `Data` / `JSON("", obj)` for unnamed events.

### WebSocket

Zero-dependency WebSocket implementation supporting both event-driven (Node.js
ws-style) and synchronous read/write APIs.

Constants:

```go
const (
    TextMessage   = 1
    BinaryMessage = 2
    CloseMessage  = 8
    PingMessage   = 9
    PongMessage   = 10
)
```

Errors:

```go
var (
    ErrWSClosed       = errors.New("websocket: connection closed")
    ErrWSInvalidFrame = errors.New("websocket: invalid frame")
)
```

UpgradeOptions:

```go
type UpgradeOptions struct {
    ReadBufferSize  int
    WriteBufferSize int
    CheckOrigin     func(r *http.Request) bool
    Subprotocols    []string
}
```

Upgrade entry point:

```go
func (ctx *Context) Upgrade(opts *UpgradeOptions) (*WSConn, error)
```

`Upgrade` performs the following:

1. If `opts.CheckOrigin` is set and returns false, calls `ctx.Throw(403, ...)`
   and returns an error.
2. Validates the `Connection: upgrade` and `Upgrade: websocket` headers and the
   `Sec-WebSocket-Key` header. Returns an error for any missing header.
3. Negotiates a subprotocol if `opts.Subprotocols` is non-empty.
4. Hijacks the connection via `http.NewResponseController(ctx.Writer).Hijack()`.
   If hijack fails (for example when using `httptest.ResponseRecorder`), returns
   an error.
5. Sets `ctx.flushed = true`.
6. Clears the connection deadline.
7. Constructs a `bufio.Reader` with `ReadBufferSize` (default 4096).
8. Writes the HTTP 101 switching protocols response with the computed accept key
   and optional subprotocol header directly to the connection.
9. Returns the `*WSConn`.

WSConn methods -- event-driven API (Node.js ws-style):

```go
func (ws *WSConn) OnMessage(fn func(messageType int, data []byte))
func (ws *WSConn) OnClose(fn func(code int, text string))
func (ws *WSConn) OnError(fn func(err error))
func (ws *WSConn) OnPing(fn func(data []byte))
func (ws *WSConn) OnPong(fn func(data []byte))
func (ws *WSConn) Listen()
```

`Listen` runs a blocking read loop. It dispatches frames to the registered event
handlers. Ping frames receive an automatic pong reply. Close frames trigger a
close reply, close the `Closed()` channel, invoke `OnClose`, and return. Any
read error invokes `OnError`, closes the `Closed()` channel, and returns.

WSConn methods -- synchronous read/write API:

```go
func (ws *WSConn) ReadMessage() (messageType int, data []byte, err error)
func (ws *WSConn) ReadJSON(v any) error
func (ws *WSConn) WriteMessage(messageType int, data []byte) error
func (ws *WSConn) WriteJSON(v any) error
func (ws *WSConn) Send(text string) error
func (ws *WSConn) WriteText(text string) error
func (ws *WSConn) WriteBinary(data []byte) error
func (ws *WSConn) Ping() error
func (ws *WSConn) Close() error
func (ws *WSConn) CloseWithMessage(code int, text string) error
```

WSConn utility methods:

```go
func (ws *WSConn) Closed() <-chan struct{}
func (ws *WSConn) SetReadDeadline(t time.Time) error
func (ws *WSConn) SetWriteDeadline(t time.Time) error
func (ws *WSConn) Heartbeat(interval time.Duration) func()
func (ws *WSConn) NetConn() net.Conn
```

WebSocket concurrency and lifetime contracts:

- `WriteMessage`, `WriteJSON`, `Send`, `WriteText`, `WriteBinary`, and `Ping`
  are serialized by an internal `sync.Mutex` (`writeMu`). Concurrent writes
  from multiple goroutines are safe.
- `ReadMessage` and `ReadJSON` are serialized by a separate `sync.Mutex`
  (`readMu`). Concurrent reads from multiple goroutines are safe.
- `Listen` does not acquire the read mutex. Do not call `ReadMessage` /
  `ReadJSON` while `Listen` is running; choose one API or the other.
- `Close` and `CloseWithMessage` close the `Closed()` channel exactly once via
  `sync.Once`, then write a close frame and close the underlying `net.Conn`.
- `Heartbeat` returns a stop function that closes an internal channel. Calling
  the stop function more than once panics. Call it exactly once via `defer`.
- `maxFrameSize` is 65536 bytes. Frames larger than this cause `ReadMessage` /
  `Listen` to return `ErrWSInvalidFrame`.
- Server frames are unmasked (per RFC 6455). Client frames must be masked;
  the implementation correctly unmasks incoming masked frames.
- The `WriteBufferSize` field in `UpgradeOptions` is declared but not used by
  the current implementation. Writes go directly to the underlying `net.Conn`
  without additional buffering.

## Routing

The router maintains one prefix trie per HTTP method. Lookup is O(segments)
in the request path.

Path parameter syntax:

- `:name` matches a single path segment; accessible via `ctx.Param("name")`.
- `*name` matches the remainder of the path; accessible via `ctx.Param("name")`.

Matching rules an agent must respect:

- Literal segments win over wildcard segments at the same trie level. Given
  both `/files/exact` and `/files/*filepath`, a request to `/files/exact`
  hits the literal route.
- A `*name` wildcard must be the last segment. `parsePattern` stops scanning
  at the first `*` and silently discards anything after it, so `/a/*b/c` is
  registered as `/a/*b`. The wildcard value joins the remaining segments
  with `/`.
- Multiple consecutive slashes are collapsed by `parsePattern`. `//p//:name//`
  registers the same route as `/p/:name`.
- Registering the same `method + pattern` twice overwrites the previously
  registered handler without warning. Detect duplicates in your own
  registration code if you need that guarantee.
- Pattern matching is per HTTP method. A `GET /users/:id` route does not
  serve `POST /users/42`; the trie returns no match and a synthesized
  notFound handler runs through the full middleware chain.

Examples:

- `/users/:id` matches `/users/42` with `Param("id") == "42"`.
- `/files/*filepath` matches `/files/css/style.css` with
  `Param("filepath") == "css/style.css"`.

## Built-in middleware

### Logger

```go
func Logger() Middleware
```

Records `time.Now()` before calling `next()`, then prints
`[<status>] <RequestURI> in <duration>` through the standard `log` package
once the chain returns. If `ctx.flushed` is true (SSE or WebSocket), the
logged status is hardcoded to 101. Output is plain text, not structured. If
you need structured logging, replace this middleware with one that wraps your
own logger; do not chain a second middleware on top hoping to override the
format.

### Recovery

```go
func Recovery() Middleware
```

Wraps `next()` in a `defer`/`recover`. On panic it logs the message plus a
trace built from `runtime.Callers(3, ...)` (skipping `Callers`, `trace`, and
the deferred closure), sets `ctx.Status = http.StatusInternalServerError`,
and assigns `ctx.Body = H{"message": "Internal Server Error"}`. The fixed
JSON body means downstream consumers must replace this middleware to deliver
a custom error envelope; chaining will not work because the response is
mutated before the chain completes.

Known limitation: `Recovery` does not re-raise `http.ErrAbortHandler`. The
Go standard library uses that sentinel panic to abort a request without
logging; with this middleware in place the abort is converted into a normal
500 response. If you serve traffic where this distinction matters, fork
`Recovery` and add `if err == http.ErrAbortHandler { panic(err) }` before
the response-mutation block.

## Internal implementation details affecting correctness

### Trie structure

Each HTTP method has its own root `node`. The `node` type is unexported:

```go
type node struct {
    pattern  string   // non-empty only at leaf nodes representing a complete route
    part     string   // the segment this node represents
    children []*node
    isWild   bool     // true when part starts with ':' or '*'
}
```

`matchChild` returns the first child with an exact `part` match (used during
insertion). `matchChildren` returns all children that match -- exact matches
first, then wild matches. This ordering gives literal segments priority during
search.

### compose function

```go
func compose(middlewares []Middleware, final Middleware) func(ctx *Context)
```

Builds a dispatch closure. Each call to `next()` increments an index counter.
When the index exceeds `len(middlewares)`, the `final` handler executes with
an empty `next` (`func() {}`). There is no guard against calling `next()`
multiple times; the downstream chain re-executes from the next index.

### Response rendering

`ctx.respond()` is called exactly once by `router.handle` after the composed
chain returns. It checks `ctx.flushed` first (SSE/WebSocket/Static bail out).
The auto-200 logic: if `Body != nil` and `statusSet == false` and
`Status == http.StatusNotFound`, promote to `http.StatusOK`. Headers from
`ctx.headers` are flushed. Then the type switch selects the renderer.

### matchRouterPath

```go
func matchRouterPath(requestPath, routerPrefix string) bool
```

Returns true when the prefix is empty or `/`, or when requestPath starts with
routerPrefix and the next character is either end-of-string or `/`. This
prevents `/v1` from matching `/v10`.

## Typical usage

### Minimal JSON API with SSE streaming

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os/signal"
    "syscall"
    "time"

    swifty "github.com/hangtiancheng/swifty.go/swifty_http"
)

func main() {
    app := swifty.Default()

    api := app.Router("/api")
    api.Use(authMiddleware)

    api.Get("/users/:id", func(ctx *swifty.Context, next func()) {
        id := ctx.Param("id")
        user, err := findUser(id)
        if err != nil {
            ctx.Throw(http.StatusNotFound, "user not found")
            return
        }
        ctx.Status = http.StatusOK
        ctx.JSON(user)
    })

    api.Post("/chat", func(ctx *swifty.Context, next func()) {
        sse := ctx.SSE()
        defer sse.Done()
        stop := sse.Heartbeat(15 * time.Second)
        defer stop()

        upstream := generateResponse(ctx.Request.Context())
        for {
            select {
            case <-sse.Closed():
                return
            case chunk, ok := <-upstream:
                if !ok {
                    return
                }
                sse.Data(chunk)
            }
        }
    })

    go func() {
        if err := app.Listen(":8080"); err != nil && err != http.ErrServerClosed {
            log.Fatal(err)
        }
    }()

    sigCtx, stopSig := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stopSig()
    <-sigCtx.Done()

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    _ = app.Shutdown(shutdownCtx)
}
```

### WebSocket echo server

```go
package main

import (
    swifty "github.com/hangtiancheng/swifty.go/swifty_http"
)

func main() {
    app := swifty.Default()

    app.Get("/ws", func(ctx *swifty.Context, next func()) {
        ws, err := ctx.Upgrade(nil)
        if err != nil {
            return
        }
        defer ws.Close()

        for {
            msgType, data, err := ws.ReadMessage()
            if err != nil {
                return
            }
            if err := ws.WriteMessage(msgType, data); err != nil {
                return
            }
        }
    })

    app.Listen(":8080")
}
```

### WebSocket with event-driven API

```go
package main

import (
    "log"
    "time"

    swifty "github.com/hangtiancheng/swifty.go/swifty_http"
)

func main() {
    app := swifty.Default()

    app.Get("/ws", func(ctx *swifty.Context, next func()) {
        ws, err := ctx.Upgrade(&swifty.UpgradeOptions{
            CheckOrigin: func(r *http.Request) bool { return true },
        })
        if err != nil {
            return
        }
        defer ws.Close()

        stop := ws.Heartbeat(30 * time.Second)
        defer stop()

        ws.OnMessage(func(messageType int, data []byte) {
            log.Printf("received: %s", data)
            _ = ws.Send("echo: " + string(data))
        })

        ws.OnClose(func(code int, text string) {
            log.Printf("closed: %d %s", code, text)
        })

        ws.OnError(func(err error) {
            log.Printf("error: %v", err)
        })

        ws.Listen()
    })

    app.Listen(":8080")
}
```

### WebSocket with JSON messages

```go
app.Get("/ws", func(ctx *swifty.Context, next func()) {
    ws, err := ctx.Upgrade(nil)
    if err != nil {
        return
    }
    defer ws.Close()

    var req RequestMessage
    if err := ws.ReadJSON(&req); err != nil {
        return
    }
    _ = ws.WriteJSON(ResponseMessage{Status: "ok", Data: req.Data})
})
```

## Pitfalls / known limitations

1. `BindJSON` closes the request body. Calling it twice or reading the body
   afterwards yields an empty or errored read. There is no built-in body
   buffering.

2. The `next()` function inside `compose` has no single-call guard. Calling it
   more than once from the same middleware re-executes the downstream chain.
   Write middleware defensively.

3. `Recovery` swallows `http.ErrAbortHandler`. Standard library middleware that
   panics with this sentinel (e.g., `httputil.ReverseProxy` on client disconnect)
   will not abort cleanly; the request gets a 500 instead.

4. Route registration is not goroutine-safe. All routes must be registered
   before calling `Listen`. Registering routes from handlers or background
   goroutines causes data races.

5. `SSEWriter.Heartbeat` and `WSConn.Heartbeat` return a stop function that
   panics if called more than once. Always use `defer stop()` and do not
   store the function for reuse.

6. `WriteBufferSize` in `UpgradeOptions` is accepted but not used. All WebSocket
   writes go unbuffered to the underlying `net.Conn`.

7. WebSocket `maxFrameSize` is 65536 bytes. Messages larger than this cause a
   read error (`ErrWSInvalidFrame`). There is no message fragmentation support.

8. `SetFuncMap` must be called before `LoadHTMLGlob`; otherwise the template
   function map is not applied and template execution may fail.

9. `ctx.Set(...)` headers are only flushed by `respond()` and by `ctx.SSE()`.
   The `Static` handler bypasses header flushing entirely. To add custom
   headers to static file responses, write to `ctx.Writer.Header()` directly
   in a wrapping middleware.

10. `Redirect` always uses status 302 (Found). There is no parameter for 301
    (Moved Permanently) or 307 (Temporary Redirect). Set `ctx.Status` manually
    and `ctx.Set("Location", url)` if you need a different redirect status.

11. The WebSocket `Listen` method does not acquire `readMu`. Do not call
    `ReadMessage` or `ReadJSON` concurrently with `Listen`; choose one read
    pattern per connection.

12. Middleware ordering depends on router declaration order in `app.routers`,
    not nesting depth. All routers whose prefix matches the request path
    contribute their middleware, concatenated in the order they were created.

## File map

| File           | Purpose                                                                                |
| -------------- | -------------------------------------------------------------------------------------- |
| `swifty.go`    | `Application`, constructors, `Listen`, `Shutdown`, `ServeHTTP`, and `compose`          |
| `context.go`   | `Context`, `H` and `Middleware` aliases, request accessors, `Throw`, `Set`             |
| `response.go`  | Deferred response setters (`JSON`, `String`, `Data`, `HTML`, `Redirect`) and `respond` |
| `group.go`     | `Router` with prefix, `Use`, method registration, `Static`, `matchRouterPath`          |
| `router.go`    | Internal `router`: `addRoute`, `getRoute`, `handle`, synthesized 404 handler           |
| `trie.go`      | Trie `node`: `insert`, `search`, `travel`, child matching                              |
| `sse.go`       | `SSEWriter` and `Context.SSE`                                                          |
| `websocket.go` | `WSConn`, `UpgradeOptions`, `Context.Upgrade`, WebSocket frame I/O, constants, errors  |
| `logger.go`    | `Logger` middleware                                                                    |
| `recovery.go`  | `Recovery` middleware and the panic trace helper                                       |
| `main.go`      | Demo entry point; carries `//go:build ignore`, excluded from package builds            |

## Dependencies

None beyond the Go standard library. The package's `go.mod` declares
`go 1.26.0`; generated code should target that toolchain or newer.

Standard library packages used: `bufio`, `context`, `crypto/sha1`,
`encoding/base64`, `encoding/binary`, `encoding/json`, `errors`, `fmt`,
`html/template`, `io`, `log`, `mime/multipart`, `net`, `net/http`, `path`,
`runtime`, `strings`, `sync`, `time`, `unicode/utf8`.

## Cross-references

- `swifty-cache` -- in-memory caching layer; commonly used together in API
  handlers that need response caching.
- `swifty-orm` -- database ORM; handlers typically call into ORM models to
  fetch data before responding via `ctx.JSON`.
- `swifty-rpc` -- gRPC framework; for services that expose both HTTP and gRPC,
  use `swifty_http` for the HTTP surface and `swifty_rpc` for the gRPC surface.

## Behavioral contracts cheat sheet

Use this table when generating or reviewing `swifty_http` code. Every row is a
non-obvious contract enforced by the source.

| Area              | Contract                                                                                                                  |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------- |
| Status auto-200   | `Body != nil` and `Status` never explicitly set promotes status from `http.StatusNotFound` to `http.StatusOK`.            |
| Throw stickiness  | `ctx.Throw` flips an internal `statusSet`; later setters keep the throw status unless `ctx.Status` is reassigned.         |
| Redirect lock     | `ctx.Redirect(url)` always writes status 302 and overwrites any previously assigned status.                               |
| respond skip      | `respond()` returns early when `ctx.flushed == true`; `SSE()`, `Upgrade()`, and `Static` rely on this.                    |
| BindJSON closes   | `ctx.BindJSON` closes `Request.Body` via `defer`; do not read the body again afterwards.                                  |
| Header buffering  | `ctx.Set` buffers headers in `ctx.headers`; they flush in `respond()` and in `SSE()` only. `Static` does not flush them.  |
| Static 404 body   | A missing static file responds 404 with an empty body; the static handler does not populate `ctx.Body`.                   |
| Router boundary   | Prefix `/v1` matches `/v1` and `/v1/...`, never `/v10`. Enforced by `matchRouterPath`.                                    |
| Route concurrency | Route maps are unsynchronized. Register every route before `Listen`; do not register from handlers or background workers. |
| Duplicate routes  | Re-registering `method + pattern` silently overwrites the previous handler.                                               |
| Wildcard position | `*name` must be the last segment; trailing segments are silently dropped by `parsePattern`.                               |
| Literal priority  | When literal and wildcard children coexist, literal wins.                                                                 |
| next() reuse      | `compose` does not guard against multiple `next()` calls; downstream chains run repeatedly if invoked again.              |
| Heartbeat stop    | The `stop` function returned by `SSEWriter.Heartbeat` and `WSConn.Heartbeat` is not idempotent; call exactly once.        |
| SSE flusher       | `ctx.Writer` not implementing `http.Flusher` degrades silently; `Flush` becomes a no-op and writes may buffer.            |
| ID / Retry        | SSE field lines (`ID`, `Retry`) do not by themselves dispatch an event; pair them with `Event`, `Data`, or `JSON`.        |
| Stream cancel     | `SSEWriter.Stream` returns on channel close or client disconnect; it does not cancel the upstream producer for you.       |
| Recovery panic    | `Recovery` does not re-raise `http.ErrAbortHandler`; forks must add the re-raise if that distinction matters.             |
| Listen return     | `Listen` returns `http.ErrServerClosed` after a successful `Shutdown`; treat that value as a normal termination.          |
| Templates         | `SetFuncMap` must precede `LoadHTMLGlob`; otherwise FuncMap is not captured.                                              |
| WS maxFrameSize   | Frames over 65536 bytes return `ErrWSInvalidFrame`. No fragmentation support.                                             |
| WS Listen vs Read | `WSConn.Listen` and `ReadMessage` must not run concurrently; `Listen` does not acquire the read mutex.                    |
| WS hijack         | `ctx.Upgrade` requires a `ResponseWriter` that supports `Hijack` via `http.NewResponseController`.                        |
| WS close once     | `WSConn.Close` / `CloseWithMessage` use `sync.Once` on the closed channel; safe to call once only from user code.         |
| Logger flushed    | Logger logs status 101 when `ctx.flushed == true` regardless of actual `ctx.Status`.                                      |
