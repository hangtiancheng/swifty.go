## lark_http

`lark_http` 是一个用 Go 语言编写的轻量级 HTTP 框架，API 风格对齐 Node.js 的 Koa：洋葱模型中间件、延迟响应（ctx.Status / ctx.Body / ctx.Type）、链式路由分组、Server-Sent Events 流式支持，仅依赖 Go 标准库。

适合用来快速搭建 API 服务、AI Agent 后端、SSE 实时推送层，也适合用作学习 Koa 思想在 Go 中落地的参考实现。

### 特性一览

- Koa 风格中间件签名 `func(ctx *Context, next func())`，洋葱模型保留完整
- 延迟响应模型：处理器只设置 `ctx.Status` / `ctx.Body` / `ctx.Type`，链路末尾统一落盘，下游中间件可在 `next()` 之后修改响应
- 基于 Trie 的路由匹配，支持 `:param` 命名参数与 `*wildcard` 通配，命中精确路由优先
- 路由分组：`app.Router(prefix)` 派生子 Router，独立持有中间件栈，支持任意层级嵌套
- 内置 Logger 与 Recovery 中间件，`lark_http.Default()` 一行启用
- 完整 Server-Sent Events 支持：事件、JSON、心跳、Stream、客户端断连感知，并发写线程安全
- 静态文件托管、HTML 模板渲染、自定义 FuncMap
- 优雅关闭：`Shutdown(ctx)` 走标准 `http.Server.Shutdown`
- 零第三方依赖

### 安装

```bash
go get github.com/hangtiancheng/lark-go/lark_http
```

要求 Go 1.21 及以上。

### 快速开始

```go
package main

import (
    "net/http"

    lark "github.com/hangtiancheng/lark-go/lark_http"
)

func main() {
    app := lark.Default()

    app.Get("/", func(ctx *lark.Context, next func()) {
        ctx.Status = http.StatusOK
        ctx.String("Hello, lark_http")
    })

    app.Get("/users/:id", func(ctx *lark.Context, next func()) {
        ctx.JSON(lark.H{"id": ctx.Param("id")})
    })

    app.Listen(":9999")
}
```

启动后访问 `http://localhost:9999/` 或 `http://localhost:9999/users/42` 即可。

### 核心概念

#### Application

`Application` 是顶层应用对象，同时实现 `http.Handler`，可以直接挂在标准库的 `http.Server` 上。

```go
app := lark.New()       // 干净的 Application
app := lark.Default()   // 自动启用 Logger + Recovery
```

生命周期：

```go
go func() {
    if err := app.Listen(":9999"); err != nil && err != http.ErrServerClosed {
        log.Fatal(err)
    }
}()

// ... 收到退出信号
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
app.Shutdown(ctx)
```

#### Context

`Context` 是单次请求的上下文容器，同时承载请求访问与延迟响应。

请求侧：

```go
ctx.Path                 // 请求路径
ctx.Method               // HTTP 方法
ctx.Query("page")        // URL Query
ctx.Param("id")          // 路由参数（:id 或 *filepath）
ctx.PostForm("name")     // 表单字段
ctx.Get("Authorization") // 请求头
ctx.BindJSON(&payload)   // 解码 JSON 请求体
```

响应侧（延迟生效，链路末尾统一写出）：

```go
ctx.Status = http.StatusOK
ctx.JSON(lark.H{"ok": true})            // 自动设 Content-Type: application/json
ctx.String("hello %s", name)             // 自动设 Content-Type: text/plain
ctx.Data([]byte{0x89, 0x50, 0x4e, 0x47}) // 原始字节
ctx.HTML("index.html", lark.H{"Name": "lark"})
ctx.Redirect("/login")                   // 302
ctx.Set("X-Request-Id", reqId)           // 自定义响应头
ctx.Throw(http.StatusForbidden, "forbidden") // 设状态 + JSON 错误体
```

跨中间件共享数据：

```go
ctx.State["user"] = currentUser
```

设了 `ctx.Body` 但没有显式设置状态码时，框架会自动把默认的 404 修正为 200，与 Koa 行为一致。

#### 中间件

中间件签名：

```go
type Middleware func(ctx *Context, next func())
```

洋葱模型：在 `next()` 前的代码会在下游执行前运行，在 `next()` 后的代码会在下游返回后运行。

```go
app.Use(func(ctx *lark.Context, next func()) {
    start := time.Now()
    next()
    log.Printf("%s %s -> %d (%v)", ctx.Method, ctx.Path, ctx.Status, time.Since(start))
})
```

不调用 `next()` 即可短路后续中间件与处理器。

由于响应是延迟的，下游中间件可以在 `next()` 之后修改 `ctx.Status` / `ctx.Body` / `ctx.Set(...)`，例如统一包裹返回值或统一脱敏。

#### 路由

支持的方法：`Get` / `Post` / `Put` / `Delete` / `Patch` / `Head` / `Options` / `All`。

路径语法：

- `:name` 匹配单个路径段：`/users/:id` 匹配 `/users/42`，`ctx.Param("id") == "42"`
- `*name` 通配剩余路径：`/files/*filepath` 匹配 `/files/css/app.css`，`ctx.Param("filepath") == "css/app.css"`
- 精确路由优先于通配：`/files/exact` 与 `/files/*filepath` 同时存在时，前者优先命中

#### 路由分组

```go
api := app.Router("/api")
api.Use(authMiddleware)

v1 := api.Router("/v1")
v1.Get("/users/:id", getUser)
v1.Post("/users", createUser)

v2 := api.Router("/v2")
v2.Use(rateLimit)
v2.Get("/users/:id", getUserV2)
```

每个 Router 拥有独立的中间件栈，父分组的中间件会按声明顺序在子分组之前执行。路径边界精确判断，`/v1` 分组上的中间件不会误命中 `/v10`。

#### 静态文件

```go
app.Static("/assets", "./public")
```

底层使用 `http.FileServer` + `StripPrefix`，自动处理 ETag、Range、目录索引。也可以挂在分组下：

```go
admin := app.Router("/admin")
admin.Static("/static", "./admin/dist")
```

#### HTML 模板

```go
app.SetFuncMap(template.FuncMap{
    "upper": strings.ToUpper,
})
app.LoadHTMLGlob("templates/*.tmpl")

app.Get("/", func(ctx *lark.Context, next func()) {
    ctx.HTML("index.tmpl", lark.H{"Name": "lark"})
})
```

`SetFuncMap` 必须在 `LoadHTMLGlob` 之前调用，FuncMap 才能被模板感知。

### Server-Sent Events

`ctx.SSE()` 返回 `*SSEWriter`，自动设置好 `text/event-stream` 等响应头，并把 `ctx.flushed` 置位以跳过延迟响应。所有写入方法都被互斥锁保护，可在多 goroutine 中安全使用。

最小示例：

```go
app.Get("/events", func(ctx *lark.Context, next func()) {
    sse := ctx.SSE()
    sse.Event("greeting", "hello")
    sse.Data("world")
    sse.Done()
})
```

LLM 流式转发样例：

```go
app.Post("/chat", func(ctx *lark.Context, next func()) {
    sse := ctx.SSE()
    stop := sse.Heartbeat(15 * time.Second)
    defer stop()

    upstream := openAIStream(ctx.Request.Context(), payload)
    for {
        select {
        case <-sse.Closed():
            return
        case chunk, ok := <-upstream:
            if !ok {
                sse.Done()
                return
            }
            sse.JSON("delta", chunk)
        }
    }
})
```

可用方法：

- `Event(event, data string)` 发送命名事件
- `Data(data string)` 发送匿名数据，多行内容会自动按 `\n` 拆成多个 `data:` 行
- `JSON(event string, obj interface{})` JSON 序列化后发送，`event` 为空则只发 data
- `ID(id string)` 设置事件 ID
- `Retry(ms int)` 通知客户端重连间隔
- `Comment(text string)` 发送注释行（常用于保活）
- `Heartbeat(interval time.Duration) func()` 启动后台 keepalive，返回 stop 函数
- `Stream(ch <-chan string)` 消费 channel 直至关闭或客户端断连
- `Closed() <-chan struct{}` 客户端断连信号
- `Flush()` 手动刷盘
- `Done()` 发送 `[DONE]` 终止标记，常用于 OpenAI 兼容协议

### 内置中间件

#### Logger

```go
app.Use(lark.Logger())
```

在请求处理结束后输出 `[status] URI in duration`。`lark.Default()` 默认已启用。

#### Recovery

```go
app.Use(lark.Recovery())
```

捕获处理器中的 panic，打印调用栈，并返回 500 加 `{"message":"Internal Server Error"}`。`lark.Default()` 默认已启用。

### 完整示例

一个典型的 AI 后端片段，组合了分组、鉴权中间件、JSON 接口与 SSE 流式：

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

    app.Get("/healthz", func(ctx *lark.Context, next func()) {
        ctx.JSON(lark.H{"status": "ok"})
    })

    api := app.Router("/api")
    api.Use(requireToken)

    api.Get("/users/:id", func(ctx *lark.Context, next func()) {
        ctx.JSON(lark.H{"id": ctx.Param("id"), "name": "lark"})
    })

    api.Post("/chat", func(ctx *lark.Context, next func()) {
        var req struct {
            Prompt string `json:"prompt"`
        }
        if err := ctx.BindJSON(&req); err != nil {
            ctx.Throw(http.StatusBadRequest, "invalid payload")
            return
        }
        sse := ctx.SSE()
        defer sse.Done()
        stop := sse.Heartbeat(15 * time.Second)
        defer stop()
        for _, token := range fakeStream(req.Prompt) {
            select {
            case <-sse.Closed():
                return
            default:
                sse.Data(token)
            }
        }
    })

    go func() {
        if err := app.Listen(":9999"); err != nil && err != http.ErrServerClosed {
            log.Fatal(err)
        }
    }()

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()
    <-ctx.Done()

    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    _ = app.Shutdown(shutdownCtx)
}

func requireToken(ctx *lark.Context, next func()) {
    if ctx.Get("Authorization") == "" {
        ctx.Throw(http.StatusUnauthorized, "missing token")
        return
    }
    next()
}

func fakeStream(prompt string) []string {
    return []string{"hi, ", "this is ", prompt}
}
```

### 目录结构

```
lark_http/
├── lark.go         Application 与中间件组合器
├── context.go      Context 与请求访问器
├── response.go     延迟响应模型与渲染分支
├── group.go        Router 分组、Static、路径边界
├── router.go       内部 router 与调度
├── trie.go         路由 Trie 节点
├── sse.go          SSEWriter
├── logger.go       Logger 中间件
├── recovery.go     Recovery 中间件
└── main.go         示例入口（带 //go:build ignore，不参与编译）
```

### 设计要点

延迟响应：处理器与中间件不直接写 ResponseWriter，所有响应数据存在 `ctx.Body`、`ctx.Status`、`ctx.Type` 三个字段中，由 `respond` 在链路末尾统一渲染。这使得位于洋葱外层的中间件可以在 `next()` 返回后看到下游真实写入的响应，并按需修改，例如统一包裹返回结构、记录响应体、注入 trace header。

中间件按 Router 收集：`ServeHTTP` 入口处遍历所有 Router，凡是 `matchRouterPath` 命中的，按声明顺序把其中间件追加进候选栈，再交给 `compose` 串成链。这保证了「父分组中间件先于子分组中间件」的执行约定。

精确边界判断：`matchRouterPath` 不是用简单的 `strings.HasPrefix`，而是要求请求路径在前缀长度处要么结束，要么是 `/`，确保 `/v1` 分组上的中间件不会被 `/v10` 之类路径误命中。

SSE 自动接管响应：调用 `ctx.SSE()` 时会把 `ctx.flushed` 置为 true，告知 `respond` 跳过常规渲染，由 `SSEWriter` 完全接管字节流，避免重复写头与响应体。

### 测试

```bash
cd lark_http
go test ./...
```

仓库自带覆盖路由、Context、响应、SSE、模板、静态文件、中间件顺序、Recovery 等多组测试用例，可作为使用示例参考。

### 鸣谢

灵感来源于 Node.js 的 [Koa](https://koajs.com/) 与 Go 社区中众多优秀的轻量框架。`lark_http` 不追求功能最全，只追求在 Koa 思想与 Go 习惯之间找到一种舒适的折中。

### 许可证

详见仓库根目录 `LICENSE`。
