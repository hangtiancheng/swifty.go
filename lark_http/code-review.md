## lark_http 代码评审

本文档对 `lark_http` 这一 Go 语言 HTTP 框架进行全面的代码评审。评审范围覆盖 `lark_http/` 目录下的全部源码文件，包含模块边界、API 设计、与 Koa 的对齐情况、并发安全、错误处理、可测试性以及潜在缺陷。

### 一、整体定位与设计目标

`lark_http` 的目标是提供一个轻量级、贴近 Node.js Koa 风格的 Go HTTP 框架。从源码可以看到，框架在以下几个方面体现了 Koa 化的设计取向：

1. 中间件签名采用 `func(ctx *Context, next func())`，与 Koa 的 `async (ctx, next) => {}` 形式对齐，洋葱模型保留完整。
2. `Context` 上提供 `Status`、`Body`、`Type` 三个延迟字段，处理器只设置而不立即写出，统一在链路末尾由 `respond` 真正落盘。这与 Koa 的 `ctx.status`、`ctx.body`、`ctx.type` 几乎一一对应。
3. 顶层应用对象命名为 `Application`，构造函数为 `New` 与 `Default`，方法名 `Use`、`Listen`、`Router` 等都直接呼应 Koa / koa-router 的习惯。
4. 路由分组通过 `app.Router(prefix)` 派生，独立持有自己的中间件栈，模型上类似 koa-router 的 `Router({ prefix })` 加 `router.use()`。

整体设计目标清晰，文件粒度划分合理：一个文件一个职责，未出现明显的上帝对象。框架对外仅依赖 Go 标准库（`go.mod` 中无第三方依赖），保持了轻量。

### 二、模块与文件职责梳理

| 文件          | 主要职责                                                                                                           |
| ------------- | ------------------------------------------------------------------------------------------------------------------ |
| `lark.go`     | `Application` 的定义、`New`/`Default` 构造、`Listen`/`Shutdown` 生命周期、`ServeHTTP` 入口、`compose` 中间件组合器 |
| `context.go`  | `Context` 结构体、请求侧访问器（`Query`/`Param`/`PostForm`/`Get`/`BindJSON`）、`Throw`、`Set` 头部                 |
| `response.go` | 延迟响应模型：`JSON`/`String`/`Data`/`HTML`/`Redirect`，以及实际写出的 `respond` 与 `respond*` 系列                |
| `group.go`    | `Router` 分组：前缀拼接、`Use`、各方法注册器、`Static` 静态文件、`matchRouterPath` 边界判断                        |
| `router.go`   | 内部 `router` 调度器：注册、按方法与路径查找、`handle` 入口、`compose` 调用                                        |
| `trie.go`     | 路由 Trie 节点的 `insert`/`search`/`travel` 实现                                                                   |
| `sse.go`      | `SSEWriter`：事件、数据、JSON、注释、心跳、Stream、客户端断连感知                                                  |
| `logger.go`   | 访问日志中间件                                                                                                     |
| `recovery.go` | panic 捕获中间件与堆栈追踪                                                                                         |
| `main.go`     | 演示入口（`//go:build ignore`，不参与编译）                                                                        |

### 三、核心机制评审

#### 3.1 中间件组合（compose）

`lark.go` 中的 `compose` 通过闭包递归生成调用链：

```go
func compose(middlewares []Middleware, final Middleware) func(ctx *Context) {
    return func(ctx *Context) {
        var dispatch func(i int)
        dispatch = func(i int) {
            if i >= len(middlewares) {
                if final != nil { final(ctx, func() {}) }
                return
            }
            middlewares[i](ctx, func() { dispatch(i + 1) })
        }
        dispatch(0)
    }
}
```

优点：

- 逻辑直观，洋葱模型完全等价于 Koa 的 `koa-compose`。
- `next` 闭包通过递归构造，开发者可以在 `next()` 前后写代码，先后顺序的语义与 Koa 一致，`TestMiddlewareOrder` 也明确验证了 `root-before → api-before → handler → api-after → root-after`。

潜在改进：

- Koa 的 `compose` 对同一个中间件中多次调用 `next()` 会抛错（防止重复进入下游），当前实现不会报错也不会限制，多次调用将导致下游执行多次。建议在 `dispatch` 内引入一个布尔标记，命中后直接 `panic("next() called multiple times")`，与 Koa 行为对齐。
- 当 `final` 为 `nil` 时（`router.handle` 不会出现这种调用，但 `compose` 是导出的内部能力），链尾静默结束，未来如果被复用，建议保留显式提示。

#### 3.2 请求生命周期（ServeHTTP）

`Application.ServeHTTP` 的流程：

1. 遍历 `app.routers`，凡是 `matchRouterPath` 命中的，把这些 `Router` 的中间件按声明顺序追加到候选切片中。
2. 构造 `Context`，并把当前 `app` 注入到 `ctx.app`，便于响应阶段访问模板。
3. 调用 `router.handle(ctx, middlewares)`，由内部路由器完成路由匹配与 `compose`。
4. 链路结束后由 `router.handle` 内部统一调用 `ctx.respond()`。

值得肯定的点：

- 中间件收集采用按 Router 声明顺序拼接，使得「父分组中间件优先于子分组中间件」这一约定能稳定成立。
- `matchRouterPath` 通过显式判断 `requestPath[len(routerPrefix)] == '/'` 来避免 `/v1` 误命中 `/v10`，这是常见框架的边界陷阱，作者已专门写了 `TestRouterMiddlewareUsesPathBoundary` 进行回归保护。

潜在问题与建议：

1. 每次请求都会对 `app.routers` 做一次线性扫描。在路由分组较多（数百级）的场景下，会出现 O(N) 的开销。可以在 `Router` 注册时构造一棵 prefix Trie，请求时按路径前缀逐段下沉，得到匹配栈。
2. `ctx.app = app` 是在 `newContext` 之后再赋值的，这种「先建再补」的写法依赖调用顺序。如果未来有人直接在测试中 `newContext` 后再用 `respond` 渲染 HTML，就需要手工补 `ctx.app`。建议把 `newContext` 调整为 `newContext(app, w, req)`，构造时一次性完成依赖注入。
3. 没有对 `app.router` 注册期与处理期之间的并发做约束。Go 的 map 在并发写读时会触发 fatal error，目前 `addRoute` 与 `getRoute` 共享 `roots`/`handlers`，如果有人在服务启动后调用 `app.Get(...)` 动态加路由，就会与请求处理产生竞态。建议在 README 或文档里明确「路由注册必须在 `Listen` 之前完成」，或为 router 加一把 `sync.RWMutex`。

#### 3.3 延迟响应模型（respond）

`response.go` 中的 `respond` 是这套框架最贴近 Koa 的部分：

```go
if ctx.Body != nil && !ctx.statusSet && ctx.Status == http.StatusNotFound {
    ctx.Status = http.StatusOK
}
```

这段是「设置了 Body 就自动转 200」的自动状态码，等价于 Koa 中「`ctx.body = ...` 隐式把 404 提升为 200」。设计上很贴心，`TestAutoStatusOKWhenBodySet` 与 `TestAutoStatusPreservesExplicitStatus` 双向覆盖。

调度依据 `ctx.Body` 的运行时类型选择渲染分支：`htmlPayload → []byte → string → io.Reader → JSON`。这种 switch 是 Koa `application.respond` 的直译，整体易懂。

需要改进的点：

1. `respondJSON` 在 `json.Marshal` 失败后只是 `return`，但此时 `WriteHeader(ctx.Status)` 已经写过头部，客户端会收到一个空响应体，而后端只是静默丢错。建议至少 `log.Printf` 出来，并在序列化前先做 `Marshal`，成功后再 `WriteHeader`，避免「半截响应」。
2. `respondBytes` 与 `respondReader` 在 `ctx.Type` 为空时不会主动设置 `Content-Type`，客户端只能依赖浏览器或代理推断。可在 `respondBytes` 默认填 `application/octet-stream`，在 `respondReader` 同样如此，避免出现少量边界场景下的 MIME 嗅探。
3. `respondHTML` 在模板未加载时返回 `500` 加 JSON 文案 `{"message":"HTML templates are not loaded"}`，但响应头里的 `Content-Type` 没有被设置（成功分支才会 `Set("Content-Type","text/html")`），客户端收到的是无明确类型的 JSON 文本，体验不一致。建议在兜底分支显式 `Set("Content-Type","application/json")`。同时 `ExecuteTemplate` 的错误被静默吞了，建议落日志。
4. `Set` 与 `headers` map：`ctx.Set("X-Foo", "a")` 多次调用同一 key 会覆盖，无法实现 Cookie 之类需要 `Add` 的语义。可以再补一个 `Append(header, value)` 或者把内部存储改成 `http.Header`（即 `map[string][]string`），借用 `Header.Set/Add` 双语义。
5. `Redirect` 强制 `302`，而 Koa 的 `ctx.redirect(url)` 默认也是 302，但允许通过 `ctx.status = 301; ctx.redirect(url)` 临时切到 301。当前实现是先设 `Status=302` 再设 `statusSet=true`，调用者无法覆盖。建议在 `Redirect` 内判断「`statusSet` 已经为 true 就保留原状态」，与 Koa 对齐。

#### 3.4 路由 Trie 与匹配

`trie.go` 是较为经典的「按段切分 + 通配优先级在后」的实现。`matchChildren` 先返回精确匹配，再追加 wild 子节点，使得 `/files/exact` 与 `/files/*filepath` 同时存在时优先命中精确路由，`TestStaticRouteTakesPriorityOverWildcard` 已覆盖。

`parsePattern` 的细节值得点出：

- 遇到以 `*` 开头的段会立即 `break`，这意味着 `*name` 之后的任何段都会被忽略。`TestParsePattern` 中 `/p/*name/*` 的结果是 `["p", "*name"]`，与设计相符。
- 多斜杠会被自动合并：`//p//:name//` 解析为 `["p", ":name"]`。

潜在问题：

1. `parsePattern` 不会对 `*` 在中间段的「错误使用」给出报错。例如 `/a/*b/c`，`/c` 段会被静默丢弃，使用者察觉不到拼写错误。建议在解析时如果 `*name` 不是最后一段，直接 `panic` 或者打印警告。
2. `node.insert` 在递归中通过 `part[0]` 取首字节判断通配，没有先校验 `part` 是否为空。`parsePattern` 已经把空段过滤掉，所以理论上不会爆 index out of range，但 `insert` 被 `addRoute` 直接调用而非走 `parsePattern`，未来如果有人在框架内部传入空 parts，就会发生越界 panic。可以加一个防御性 `if part == ""`。
3. 路由冲突没有检测。重复注册同一 `method + pattern` 会让后注册的处理器覆盖前者（map 直接 `r.handlers[key] = handler`），且 Trie 节点的 `pattern` 也被无声覆盖。建议在 `addRoute` 中检查 `r.handlers[key]` 是否已存在，若已存在则 `panic("duplicate route registration")`，与主流框架（gin、echo）一致。
4. `getRoute` 的 `params` 通过 `make(map[string]string)` 每次请求都新建一个 map；对于无参路由也会触发分配。可以将其延后到「确定有 `:` 或 `*` 段」再创建，减少 GC 压力。

#### 3.5 静态文件服务

`Router.Static` 的实现采用 `http.FileServer + StripPrefix`：

```go
func (r *Router) createStaticHandler(relativePath string, fs http.FileSystem) Middleware {
    absolutePath := path.Join(r.prefix, relativePath)
    fileServer := http.StripPrefix(absolutePath, http.FileServer(fs))
    return func(ctx *Context, next func()) {
        file := ctx.Param("filepath")
        if _, err := fs.Open(file); err != nil {
            ctx.Status = http.StatusNotFound
            return
        }
        ctx.flushed = true
        fileServer.ServeHTTP(ctx.Writer, ctx.Request)
    }
}
```

优点：

- 借助标准库 `http.FileServer`，自动处理 ETag、Range、目录索引。
- 通过 `ctx.flushed = true` 标记响应已绕过 `respond`，避免重复写头。

需要改进：

1. 仅捕获 `fs.Open` 失败这一种情况，并将其转换为 `404 NOT FOUND`。但当 `404` 命中后，函数立即 `return`，此时 `ctx.Body` 仍为 `nil`，`respond` 阶段会单独写 `WriteHeader(404)`，但响应体为空。需要补一句 `ctx.Body = "404 NOT FOUND"`，与 `router.handle` 的 notFound 行为对齐。
2. `fs.Open(file)` 仅做了「能否打开」的预检，浪费一次系统调用，且没有处理目录请求（目录会被 `http.FileServer` 返回索引页面，可能不是期望行为）。可以删除预检，直接交给 `FileServer`，并通过自定义 `http.FileSystem` 屏蔽目录。
3. 在 `ctx.flushed = true` 之前没有把 `ctx.headers` 真正写进 `ctx.Writer.Header()`，开发者在 `Static` 之前调用 `ctx.Set(...)` 设置的头部不会生效，与 `SSE` 相比缺乏一致性。建议在 `flushed = true` 之前补一次 `for k, v := range ctx.headers { ctx.Writer.Header().Set(k, v) }`。

#### 3.6 SSE 支持

`sse.go` 的实现是这个框架中最有价值的差异化能力，覆盖了大多数 LLM 流式场景需要的能力：

- 写入 API：`Event`、`Data`、`JSON`、`ID`、`Retry`、`Comment`、`Done`。
- 协议层：`writeData` 自动按 `\n` 切分为多个 `data:` 行，符合 SSE 规范；`Comment` 同样按行处理。
- 周边能力：`Heartbeat` 启动后台 keepalive、`Stream` 消费 channel、`Closed` 暴露请求 Context 的 Done 通道、`Flush` 手动刷盘。
- 并发安全：所有写入都被 `sync.Mutex` 包裹，`TestSSEHeartbeatConcurrency` 覆盖了心跳和业务数据并发写的场景。

可改进项：

1. `SSE()` 调用了 `ctx.Writer.(http.Flusher)` 但忽略了类型断言失败的情况：当底层 Writer 不实现 `Flusher` 时，`w.flusher` 为 `nil`，后续 `flush()` 会成为 no-op，但实际上数据可能滞留在缓冲区。这种降级是有意为之的兼容路径，不过没有任何告警，调试时容易困惑。建议至少在 SSE 初始化时打一条 `log.Printf("writer does not implement http.Flusher; SSE updates may be buffered")`。
2. `Event(event, data)` 写 `event:` 之前没有判断 `event` 是否为空，理论上 `event: \n` 是合法的（SSE 规范允许，但语义无用）。`JSON` 已经有 `if event != ""` 的判断，建议在 `Event` 上做同样的对齐，避免出现空事件名。
3. `Retry` 在格式化中追加 `\n\n`（结束帧），而 `ID` 仅追加 `\n`，未追加空行。SSE 规范中 `id`、`event`、`retry` 这些是字段，必须配合 `data:` 字段后的空行才能被客户端 dispatch。`ID` 单独 flush 一次空行后再来 `Data`，客户端虽然能处理，但语义不严谨。建议把 `ID` 仅写入字段不主动 flush，由后续 `Data`/`Event` 触发完整事件块；或者在框架文档里明确「`ID` 必须紧跟一个 `Data`/`Event` 调用」。当前 `TestSSEWriter` 测的是包含关系，没有揭露这个细节。
4. `Heartbeat` 返回的 `stop` 函数没有做幂等保护，重复 `stop()` 会因 `close(stop)` 触发 `panic: close of closed channel`。建议加一个 `sync.Once`：

   ```go
   var once sync.Once
   return func() { once.Do(func() { close(stop); <-done }) }
   ```

5. `Stream(ch <-chan string)` 在 `ctx.Request.Context().Done()` 触发后只是退出循环，并没有把 `ch` 排空或通知上游生产者，依赖调用方自行处理。建议在文档里强调「客户端断连后请检查 `sse.Closed()` 自行 cancel 生产者」。

#### 3.7 Context 与请求访问器

`context.go` 整体简洁，几点值得注意：

1. `Param` 中的 `value, _ := ctx.Params[key]` 写法多余，直接 `value := ctx.Params[key]` 即可，逗号 ok 形式只在判断存在性时才需要。这是 `go vet` 不会报错但 lint 会建议简化的小问题。
2. `BindJSON` 直接 `defer ctx.Request.Body.Close()`，处理完后 body 不再可用。如果有中间件链中后续也想读 body（例如审计、签名），就会拿到空。可以提供一个 `RawBody()` 缓存 `[]byte`，或者文档中明确「BindJSON 只能调用一次」。
3. `Throw` 把 Body 设置为 `H{"message": msg}`，强制 JSON 结构。如果业务希望返回纯文本错误（例如 plain text 的 `Forbidden`），就需要绕过 `Throw` 自行设置 `Status` 与 `Body`。这是一个轻度的语义锁定，可以接受，但可以补一个 `ThrowText` 或者让 `Throw` 接受 `interface{}` 类型的 body。
4. `Context` 字段全部导出，外部可以直接修改 `ctx.Status` 等。这与 Koa 的「ctx 上字段是数据」的风格一致；但 `headers`、`statusSet`、`flushed`、`app` 等内部字段是小写未导出的，封装边界清晰。
5. `State map[string]interface{}` 用于中间件间共享数据，并发安全完全交给调用方。若同一请求内存在并发 goroutine 修改 `State`（例如 SSE 中起 goroutine），调用方需要自带锁，文档应说明这一点。

#### 3.8 Logger 与 Recovery

- `Logger` 简洁直接，`%s` 打印 `RequestURI` 与耗时。但缺少请求方法、远端 IP 等字段，生产环境一般还会要求结构化日志。可以引入接口 `Logger interface{ Printf(...) }` 把底层 logger 注入，方便对接 zap/logrus。
- `Recovery` 自带 `trace`，会输出最多 32 层调用栈。`runtime.Callers(3, pcs[:])` 的偏移在 `defer` + `recover` 的标准用法下是合适的。但响应内容固定为 `{"message":"Internal Server Error"}`，并直接覆盖了 `ctx.Body`；如果上游有自定义错误响应中间件，可能会丢失上下文。可在 panic 时把 `err` 也塞进 `ctx.State["panic_error"]`，方便上层日志或埋点继续使用。
- `Recovery` 没有处理 `http.ErrAbortHandler`，这是 Go 标准库约定的「希望直接终止 handler 且不打日志」的特殊 panic。建议判断 `if err == http.ErrAbortHandler { panic(err) }`，与 net/http 保持一致。

#### 3.9 Application 生命周期

- `Listen` 直接 `ListenAndServe`，阻塞当前 goroutine；`Shutdown` 走标准 `http.Server.Shutdown`。基本能力齐全。
- 未提供 `ListenTLS`、`Serve(l net.Listener)`、`ServeWith(srv *http.Server)` 等扩展点，复杂部署场景（HTTP/2、unix socket、preforked listener）需要绕过框架自行启动。建议至少补一个 `ServeListener(l net.Listener)`。
- `Listen` 阻塞返回错误，但当 `Shutdown` 触发时 `ListenAndServe` 会返回 `http.ErrServerClosed`。文档需要说明：调用方应判断 `if err != nil && err != http.ErrServerClosed`。

### 四、可测试性与测试覆盖

测试文件分布与质量：

- `lark_test.go`：覆盖嵌套 Router、中间件顺序、Router 路径边界、Recovery、所有 HTTP 方法、静态文件、模板、`Listen` 错误、`matchRouterPath` 边界。
- `context_test.go`：覆盖请求访问器、`BindJSON`、`State`、`Throw`。
- `response_test.go`：覆盖 JSON、String、Data、空 Body、`flushed`、自定义头、自动状态码、显式状态码、链尾修改响应。
- `router_test.go`：覆盖 `parsePattern`、`getRoute`、`getRoutes`、精确路由优先级、404、`node.String`。
- `sse_test.go`：覆盖事件、多行 data、JSON、Comment、Stream、自定义头、心跳并发、`Closed`。

整体测试覆盖很完整，主要功能路径都有覆盖。`go test ./...` 跑通无失败。建议补强的方向：

1. 没有 `Redirect` 的测试。
2. 没有 HTML 模板未加载时的兜底测试。
3. 没有 `respondReader` 的测试（`io.Reader` 分支）。
4. 没有 `BindJSON` 失败的负样例测试。
5. 没有「中间件多次调用 `next()`」的行为定义测试，建议在确定下游策略后补上。
6. SSE 的 `Heartbeat` 没有断言「能在 ticker 触发后写出 keepalive 帧」的最小样例（仅做了并发压测）。

### 五、对齐 Koa 的差距与建议

参照 Koa 的核心 API，对照如下：

- `app.use(fn)`：已对齐。
- `app.listen(port)`：已对齐。
- `ctx.body = ...`、`ctx.status = ...`、`ctx.type = ...`：通过字段赋值实现，已对齐。
- `ctx.throw(status, msg)`：已对齐，但 body 固定 JSON 形态。
- `ctx.redirect(url, alt)`：仅支持 `Redirect(url)`，未支持 `alt` 备用文案，且无法切换 301。
- `ctx.cookies.set/get`：完全缺失。如果定位是「最小可用」可以暂不补；如果想真正对齐 Koa，需要新增 `Cookies` 子对象。
- `ctx.request.is(type)`、`ctx.accepts(types...)`：缺失。内容协商相关 API 都未提供。
- `ctx.fresh`、`ctx.assert`：缺失。
- 错误事件 `app.on('error', fn)`：缺失。`Recovery` 完成了「捕获并响应」，但没有暴露一个让用户自定义错误回调的钩子。

补齐建议（优先级降序）：

1. `ctx.Cookies()` 子对象（高频，对齐认证场景）。
2. `app.OnError(handler func(err interface{}, ctx *Context))` 钩子，让 `Recovery` 把错误回调出来。
3. `ctx.Redirect(url, alt string)` 允许在不支持重定向的客户端上回退到文本（HTML 链接）。
4. `ctx.Accepts(mimes ...string) string`，配合 `respondJSON/respondHTML` 做 content negotiation。

### 六、代码风格与可维护性

- 命名整体清晰，函数粒度细，单文件最长不到 200 行，可读性良好。
- 注释主要用作分节标签（如 `// Koa-style fields`），缺少包级 doc.go 与导出 API 上的 GoDoc 注释。`golint`、`staticcheck` 会就这些导出符号没有注释报告 warning。建议为所有导出类型与方法补 doc。
- `import` 顺序符合 `goimports` 习惯，未发现脏分组。
- `package lark_http` 使用了下划线命名，不符合 Go 官方推荐（推荐 `larkhttp`）。但既然这是仓库统一风格，从一致性出发可以保留。
- `main.go` 用 `//go:build ignore` 排除编译，避免污染对外引入者的二进制，这是好的实践，可在 README 中说明「`main.go` 仅为示例」。

### 七、潜在缺陷清单（按影响优先级）

| 优先级 | 位置                             | 描述                                     | 建议                                            |
| ------ | -------------------------------- | ---------------------------------------- | ----------------------------------------------- |
| 高     | `sse.go - Heartbeat`             | 返回的 `stop` 函数重复调用会 panic       | 用 `sync.Once` 包裹                             |
| 高     | `router.go - addRoute`           | 重复注册同一路由会静默覆盖               | 注册前检查并 panic                              |
| 高     | `group.go - createStaticHandler` | 404 分支未设置 `Body`，最终响应体为空    | 补 `ctx.Body = "404 NOT FOUND"` 或显式 `String` |
| 中     | `response.go - respondJSON`      | `Marshal` 失败时已写头，客户端拿到空响应 | 先 `Marshal` 再 `WriteHeader`，失败时 500       |
| 中     | `lark.go - compose`              | 多次 `next()` 不报错，与 Koa 行为不一致  | 用 flag 控制只能调用一次                        |
| 中     | `recovery.go - Recovery`         | 未放行 `http.ErrAbortHandler`            | 显式判断并重新抛出                              |
| 中     | `group.go - Static`              | 设置的自定义头部未生效                   | 在 `flushed=true` 前刷出 `ctx.headers`          |
| 中     | `lark.go - ServeHTTP`            | 路由分组过多时遍历 O(N)                  | 构造前缀 Trie 加速匹配                          |
| 低     | `context.go - Param`             | `value, _ := m[key]` 多余写法            | 简化为 `value := m[key]`                        |
| 低     | `response.go - Redirect`         | 不允许调用方覆盖为 301                   | 已设置 `statusSet` 时保留                       |
| 低     | `sse.go - Event`                 | 未防御空 `event` 名                      | 与 `JSON` 一样判断                              |
| 低     | `trie.go - parsePattern`         | `*name` 不在最后一段时静默丢段           | 解析期报错                                      |

### 八、结论

`lark_http` 在不到 800 行核心代码内，提供了路由分组、洋葱中间件、延迟响应、SSE 流式、静态文件、HTML 模板、panic 兜底、访问日志等完整能力，是一个定位清晰、风格统一、可读性强的轻量级框架。

针对 Koa 对齐目标，框架已经达成 70 分以上，关键差距在 Cookies、内容协商、错误事件钩子三块；针对生产可用度，主要风险点集中在 SSE 心跳重复关闭、路由重复注册无告警、静态文件 404 体为空这三处具体缺陷。

按本评审清单完成高优先级修复并补齐 Koa 的几项 API 后，`lark_http` 完全可以承担起 `lark_demo` 中后端 HTTP 与 SSE 转发的能力底座。
