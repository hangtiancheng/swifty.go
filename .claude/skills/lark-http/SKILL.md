---
name: lark-http
description: >
  Lightweight, Koa-inspired HTTP framework implemented in the `lark_http` package at
  `github.com/hangtiancheng/lark-go/lark_http`. Use this skill when working on HTTP
  routing, middleware chains, the `*Context` request/response API, Server-Sent Events
  streaming, router groups with prefixes, trie-based path matching, the deferred
  response pattern (`ctx.Status` / `ctx.Body` / `ctx.Type`), or the `Application`
  lifecycle (`Listen`, `Shutdown`). Strong trigger tokens include `lark_http`,
  `lark.New()`, `lark.Default()`, `app.Use(`, `app.Router(`, `ctx.SSE()`,
  `ctx.JSON(`, `ctx.Throw(`, `ctx.BindJSON(`, `lark.H{`, `lark.Middleware`,
  `*lark.Context`, `Logger()`, `Recovery()`, and the import path
  `github.com/hangtiancheng/lark-go/lark_http`.

  Do NOT use this skill for: plain `net/http` servers with no `lark_http` import;
  other Go HTTP frameworks such as `gin`, `echo`, `fiber`, `chi`, or `gorilla/mux`;
  reverse-proxy or load-balancer configuration (nginx, envoy, traefik); Node.js
  Koa code; gRPC services (use the `lark-rpc` skill); or generic Go questions that
  do not touch the framework.
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
                                             // (set by ctx.SSE() and Router.Static)
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
- `ctx.flushed = true` tells `respond()` to skip rendering. `SSE()` and
  `Static` set this flag because they own the bytes themselves.

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

| Method          | Description                                                                       |
| --------------- | --------------------------------------------------------------------------------- |
| `Query(key)`    | URL query parameter                                                               |
| `Param(key)`    | Route path parameter (`:name` or `*name`)                                         |
| `PostForm(key)` | Form value (`application/x-www-form-urlencoded` or `multipart/form-data`)         |
| `Get(header)`   | Request header                                                                    |
| `BindJSON(out)` | Decode JSON request body via `json.NewDecoder`; closes `Request.Body` via `defer` |

`BindJSON` is destructive: it consumes and closes the request body. Call it at
most once per request and do not expect downstream middleware to re-read the
body. There is no first-class accessor for the raw body; if you need it, read
`ctx.Request.Body` yourself before any `BindJSON` call.

Response setters (deferred -- written by `respond()` at end of chain):

| Method                 | Description                                                                             |
| ---------------------- | --------------------------------------------------------------------------------------- |
| `JSON(obj)`            | Sets `Type = "application/json"` and `Body = obj`                                       |
| `String(fmt, vals...)` | Sets `Type = "text/plain"` and `Body = fmt.Sprintf(fmt, vals...)`                       |
| `Data([]byte)`         | Sets `Body = bytes`; Content-Type is whatever `Type` already holds (may be empty)       |
| `HTML(name, data)`     | Sets `Type = "text/html"` and `Body = htmlPayload{name, data}`; requires `LoadHTMLGlob` |
| `Redirect(url)`        | Sets `Status = 302` and the `Location` header; always 302, overwrites any prior status  |
| `Set(header, value)`   | Buffers a response header; flushed by `respond()` or by `ctx.SSE()`                     |

`respond()` dispatches on the runtime type of `Body`:

- `htmlPayload` -> renders the named template via `app.htmlTemplates`
- `[]byte` -> written verbatim
- `string` -> written as UTF-8 text
- `io.Reader` -> copied to the writer via `io.Copy`
- anything else -> `json.Marshal` and write as JSON

`respond()` returns early if `ctx.flushed == true`. `ctx.SSE()` and
`Router.Static` set this flag because they own the response bytes. Do not call
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

| Method                        | Description                                                                |
| ----------------------------- | -------------------------------------------------------------------------- |
| `Event(event, data string)`   | Writes `event: <event>\n` followed by one or more `data:` lines and `\n\n` |
| `Data(data string)`           | Writes the data block; multiline input becomes multiple `data:` lines      |
| `JSON(event string, obj any)` | `json.Marshal` then write; if `event == ""` the `event:` line is omitted   |
| `ID(id string)`               | Writes `id: <id>\n` (a field line, not a complete event)                   |
| `Retry(ms int)`               | Writes `retry: <ms>\n\n`                                                   |
| `Comment(text string)`        | Writes one `: ` comment line per `\n`-separated input line                 |
| `Heartbeat(interval) func()`  | Spawns a goroutine that writes `: keepalive\n\n` every `interval`          |
| `Stream(ch <-chan string)`    | Forwards channel values via `Data` until `ch` closes or client disconnects |
| `Closed() <-chan struct{}`    | Returns `ctx.Request.Context().Done()` for client-disconnect detection     |
| `Flush()`                     | Manually invokes the underlying `http.Flusher`                             |
| `Done()`                      | Writes `data: [DONE]\n\n`, the conventional OpenAI-compatible sentinel     |

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
once the chain returns. Output is plain text, not structured. If you need
structured logging, replace this middleware with one that wraps your own
logger; do not chain a second middleware on top hoping to override the
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

## Typical usage

Minimal JSON API plus an SSE endpoint. Note the canonical patterns: import
under the alias `lark`, use `http.StatusOK` constants instead of literals,
run `Listen` in a goroutine, watch `sse.Closed()` to honor client
disconnects, and call `stop()` exactly once via `defer`.

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os/signal"
    "syscall"
    "time"

    lark "github.com/hangtiancheng/lark-go/lark_http"
)

func main() {
    app := lark.Default()

    api := app.Router("/api")
    api.Use(authMiddleware)

    api.Get("/users/:id", func(ctx *lark.Context, next func()) {
        id := ctx.Param("id")
        user, err := findUser(id)
        if err != nil {
            ctx.Throw(http.StatusNotFound, "user not found")
            return
        }
        ctx.Status = http.StatusOK
        ctx.JSON(user)
    })

    api.Post("/chat", func(ctx *lark.Context, next func()) {
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

## File map

| File          | Purpose                                                                                |
| ------------- | -------------------------------------------------------------------------------------- |
| `lark.go`     | `Application`, constructors, `Listen`, `Shutdown`, `ServeHTTP`, and `compose`          |
| `context.go`  | `Context`, `H` and `Middleware` aliases, request accessors, `Throw`, `Set`             |
| `response.go` | Deferred response setters (`JSON`, `String`, `Data`, `HTML`, `Redirect`) and `respond` |
| `group.go`    | `Router` with prefix, `Use`, method registration, `Static`, `matchRouterPath`          |
| `router.go`   | Internal `router`: `addRoute`, `getRoute`, `handle`, synthesized 404 handler           |
| `trie.go`     | Trie `node`: `insert`, `search`, `travel`, child matching                              |
| `sse.go`      | `SSEWriter` and `Context.SSE`                                                          |
| `logger.go`   | `Logger` middleware                                                                    |
| `recovery.go` | `Recovery` middleware and the panic trace helper                                       |
| `main.go`     | Demo entry point; carries `//go:build ignore`, excluded from package builds            |

## Dependencies

None beyond the Go standard library. The package's `go.mod` declares
`go 1.26.0`; generated code should target that toolchain or newer.

## Behavioral contracts cheat sheet

Use this table when generating or reviewing `lark_http` code. Every row is a
non-obvious contract enforced by the source.

| Area              | Contract                                                                                                                  |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------- |
| Status auto-200   | `Body != nil` and `Status` never explicitly set promotes status from `http.StatusNotFound` to `http.StatusOK`.            |
| Throw stickiness  | `ctx.Throw` flips an internal `statusSet`; later setters keep the throw status unless `ctx.Status` is reassigned.         |
| Redirect lock     | `ctx.Redirect(url)` always writes status 302 and overwrites any previously assigned status.                               |
| respond skip      | `respond()` returns early when `ctx.flushed == true`; `SSE()` and `Static` rely on this.                                  |
| BindJSON closes   | `ctx.BindJSON` closes `Request.Body` via `defer`; do not read the body again afterwards.                                  |
| Header buffering  | `ctx.Set` buffers headers in `ctx.headers`; they flush in `respond()` and in `SSE()` only. `Static` does not flush them.  |
| Static 404 body   | A missing static file responds 404 with an empty body; the static handler does not populate `ctx.Body`.                   |
| Router boundary   | Prefix `/v1` matches `/v1` and `/v1/...`, never `/v10`. Enforced by `matchRouterPath`.                                    |
| Route concurrency | Route maps are unsynchronized. Register every route before `Listen`; do not register from handlers or background workers. |
| Duplicate routes  | Re-registering `method + pattern` silently overwrites the previous handler.                                               |
| Wildcard position | `*name` must be the last segment; trailing segments are silently dropped by `parsePattern`.                               |
| Literal priority  | When literal and wildcard children coexist, literal wins.                                                                 |
| next() reuse      | `compose` does not guard against multiple `next()` calls; downstream chains run repeatedly if invoked again.              |
| Heartbeat stop    | The `stop` function returned by `SSEWriter.Heartbeat` is not idempotent; call it exactly once (typically via `defer`).    |
| SSE flusher       | `ctx.Writer` not implementing `http.Flusher` degrades silently; `Flush` becomes a no-op and writes may buffer.            |
| ID / Retry        | SSE field lines (`ID`, `Retry`) do not by themselves dispatch an event; pair them with `Event`, `Data`, or `JSON`.        |
| Stream cancel     | `SSEWriter.Stream` returns on channel close or client disconnect; it does not cancel the upstream producer for you.       |
| Recovery panic    | `Recovery` does not re-raise `http.ErrAbortHandler`; forks must add the re-raise if that distinction matters.             |
| Listen return     | `Listen` returns `http.ErrServerClosed` after a successful `Shutdown`; treat that value as a normal termination.          |
| Templates         | `SetFuncMap` must precede `LoadHTMLGlob`; otherwise FuncMap is not captured.                                              |
