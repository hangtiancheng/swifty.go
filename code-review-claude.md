# Swifty Agent 迁移 Code Review

从 Next.js 后端 (`swifty-cli/apps/swifty-agent`) 到 Go 项目 (`swifty.go/swifty_agent`) 的迁移审查报告。

---

## 一、严重 Bug (P0)

### 1. chatRequest JSON 字段命名不匹配 — 前端请求完全失效

Go 的 `chatRequest` 结构体使用了 PascalCase 的 JSON tag:

```go
// internal/app/chat_handler.go:19-22
type chatRequest struct {
    ID       string `json:"Id"`
    Question string `json:"Question"`
}
```

而 Next.js 前端发送的是小写字段:

```json
{ "id": "session-1", "question": "hello" }
```

前端发送 `{"id": "..."}` 但 Go 期望 `{"Id": "..."}` — `ctx.BindJSON` 绑定后 `req.ID` 和 `req.Question` 均为空字符串。`/api/chat` 和 `/api/chat_stream` 两个 endpoint 全部不可用。

修复: 将 JSON tag 改为 `json:"id"` 和 `json:"question"`。

### 2. System Prompt 输出格式要求完全矛盾

Next.js (`lib/ai/pipelines/chat.ts:39`):

```
  - Output markdown only
```

Go (`internal/ai/agent/chat_pipeline/prompt.go:53`):

```
  - Output must not contain markdown syntax; use plain text only
```

一个是要求输出 markdown，一个是禁止 markdown。这会导致 AI 回复格式在两个项目之间完全不同。

### 3. 工具函数中 log.Fatal 导致进程崩溃

Go 项目自身 `bug.md` 已记录此问题。工具在 AI agent 运行时被回调，使用 `log.Fatal` 会直接终止 HTTP 服务进程。涉及文件:

- `internal/ai/tools/mysql_crud.go` — 第 34, 47, 53, 62 行
- `internal/ai/tools/get_current_time.go` — 第 54 行
- `internal/ai/tools/query_internal_docs.go` — 第 29, 33, 40 行
- `internal/ai/tools/query_metrics_alerts.go` — 第 158 行

Next.js 对应实现在 `operations.ts` 中使用 try-catch 返回错误信息，不会崩溃。

### 4. mysql_crud 工具 stdin 阻塞 — HTTP 模式下永久挂起

`internal/ai/tools/mysql_crud.go:38-44` 使用 `bufio.NewScanner(os.Stdin)` 读取用户确认。HTTP 服务模式下 stdin 不可用，AI agent 调用此工具时会永久阻塞。

Next.js 版本 (`operations.ts:135-153`) 直接执行 SQL，无交互式确认。

---

## 二、重要差异 (P1)

### 5. Redis Key Prefix 不一致 — 两个项目数据不互通

| 项目    | Key Prefix | 示例 Key     |
| ------- | ---------- | ------------ |
| Next.js | `biz:`     | `biz:{uuid}` |
| Go      | `doc:`     | `doc:{uuid}` |

`lib/config.ts:48` 中 `keyPrefix: "biz:"`，而 `internal/consts/consts.go:6` 中 `RedisKeyPrefix = "doc:"`。两个项目写入的文档 key 前缀不同，无法互相检索。

### 6. Redis Vector Field Name 不一致 — 检索无法命中

| 项目           | Vector Field Name |
| -------------- | ----------------- |
| Next.js        | `vector`          |
| Go (indexer)   | `vector_content`  |
| Go (retriever) | `vector_content`  |

Next.js indexer (`indexer.ts:30`) 使用 `vector` 作为 hash field name。Go indexer (`indexer.go:32`) 通过 Eino 默认字段名 `vector_content` 存储。Go retriever (`retriever.go:32`) 搜索 `vector_content`。

如果两个项目共用同一个 Redis 实例，Go 无法检索到 Next.js 写入的向量，反之亦然。

### 7. Redis Index Schema 不一致

Next.js 创建的 index schema (`client.ts:72-86`):

- Fields: `vector` (VECTOR), `content` (TEXT), `_source` (TAG)

Go 创建的 index schema (`redis.go:46-56`):

- Fields: `content` (TEXT), `_source` (TAG), `vector_content` (VECTOR)

两个项目各自创建 index 时，vector 字段名不同。如果 Go 先创建 index，Next.js 的 ensureIndex 会认为 index 已存在 (FT.INFO 成功)，但 Next.js 写入的 `vector` 字段在 Go 的 index 中不存在。

### 8. queryPrometheusAlerts 函数存在死代码

`internal/ai/tools/query_metrics_alerts.go:56` 有提前 `return PrometheusAlertsResult{}, nil`，其后所有代码 (第 57-81 行) 不可达。Prometheus 告警查询工具永远返回空结果。

Next.js 版本 (`operations.ts:56-104`) 正常发送 HTTP 请求获取告警数据。

### 9. Go 缺少 Prometheus URL 配置项

Go 的 `config.go` 中没有 `prometheus_url` 配置字段，`config.json` 中也没有。即使修复了死代码问题，Prometheus URL 也是硬编码的 `"http://127.0.0.1:9090"`。

Next.js 通过环境变量 `PROMETHEUS_BASE_URL` 配置 (`config.ts:55`)。

### 10. Log Topic 配置: 硬编码 vs 环境变量

Go (`prompt.go:43`):

```
Log topic region: ap-guangzhou; log topic ID: 869830db-a055-4479-963b-3c898d27e755
```

Next.js (`chat.ts:12-17`):

```typescript
const LOG_TOPIC_REGION = process.env.LOG_TOPIC_REGION ?? "";
const LOG_TOPIC_ID = process.env.LOG_TOPIC_ID ?? "";
```

Go 中硬编码了特定环境的 log topic，不通用。

### 11. 变量名遮蔽 config 包

`internal/ai/agent/chat_pipeline/tools_node.go:16` 中局部变量 `config` 遮蔽了导入的 `config` 包:

```go
func newReactAgentLambda(ctx context.Context, cfg *config.Config) (*compose.Lambda, error) {
    config := &react.AgentConfig{  // 遮蔽了 config 包
```

同一文件的 `prompt.go:21` 也有类似问题:

```go
func newChatTemplate(ctx context.Context) (prompt.ChatTemplate, error) {
    config := &ChatTemplateConfig{  // 遮蔽
```

---

## 三、功能未对齐 (P2)

### 12. Redis 删除去重: 无分布式锁 vs SETNX 锁

Next.js (`indexer.ts:46-77`): 使用 SETNX 获取 per-source 分布式锁 (30s TTL)，批量删除 (每批 1000 条)，防止并发删除竞争。

Go (`index_file.go:49-65`): 直接 FT.SEARCH + DEL，LIMIT 10000，无锁保护。如果并发上传同一文件，可能产生竞争条件。

### 13. Content 截断: Go 无限制 vs Next.js 8192 字符

Next.js (`indexer.ts:17`) 定义了 `MAX_CONTENT_LENGTH = 8192`，写入 Redis 时截断 content (`indexer.ts:31`)。

Go 通过 Eino 的 Redis indexer 存储，没有显式的 content 长度限制。

### 14. Embedding Provider: Go 仅 DashScope vs Next.js 支持 Ollama

Go (`embedder.go:17`) 只使用 `dashscope.NewEmbedder`，不支持 Ollama 本地 embedding。

Next.js (`embedder.ts:10-27`) 支持 `dashscope` 和 `ollama` 两种 embedding provider。

### 15. Anthropic Signature Workaround: Go 缺失

Next.js (`models.ts:18-61`) 实现了 `createAnthropicFetch()` 来修补非官方 Anthropic 网关返回的 thinking block 缺少 `signature` 字段的问题。没有这个修补，AI SDK 会报 "Invalid JSON response" 错误。

Go 使用 Eino 的 `claude.NewChatModel`，依赖 Eino 框架内部处理此问题。如果 Eino 没有处理，可能在非官方网关下出现相同错误。

### 16. Memory LRU Eviction: Go 缺失

Next.js (`memory.ts:7-29`): 实现了 LRU 淘汰策略，最多保持 100 个 session (`MAX_SESSIONS = 100`)，超出时淘汰最旧的 session。

Go (`mem.go`): 无淘汰策略，`memMap` 会无限增长，长时间运行后内存泄漏。

### 17. Plan-Execute-Replan Executor 工具列表不完整

Go executor (`executor.go:23-26`) 注册的工具:

- MCP tools, PrometheusAlertsQueryTool, QueryInternalDocsTool, GetCurrentTimeTool
- 缺少 `MysqlCrudTool`

Go chat pipeline (`tools_node.go:35-38`) 注册的工具:

- MCP tools, PrometheusAlertsQueryTool, MysqlCrudTool, GetCurrentTimeTool, QueryInternalDocsTool
- 包含 `MysqlCrudTool`

plan-execute-replan 的 executor 缺少 MySQL CRUD 工具。

Next.js (`plan-execute-replan/index.ts:43-46`) 使用完整的 `builtinTools` (包含 mysql_crud)。

### 18. AI Ops Query Prompt 文本差异

Next.js (`index.ts:18-31`):

```
1. You are an intelligent service alert analysis assistant...
## Alert Handling Details
## Alert Root Cause Analysis N (the Nth alert)
## Handling Procedure Execution N (the Nth alert)
```

Go (`ai_ops_handler.go:12-26`):

```
"1. You are an intelligent alert analysis assistant...
## Alert Processing Details
## Root Cause Analysis N (for the Nth alert)
## Processing Plan N (for the Nth alert)
```

差异点:

- "service alert analysis" vs "alert analysis"
- "Alert Handling Details" vs "Alert Processing Details"
- "Alert Root Cause Analysis N" vs "Root Cause Analysis N"
- "Handling Procedure Execution N" vs "Processing Plan N"
- Go 版本每行带有引号包裹 (可能导致 LLM 解析差异)

### 19. Executor MaxIterations 差异巨大

| 项目    | Executor MaxIterations |
| ------- | ---------------------- |
| Next.js | 10 (per step)          |
| Go      | 999,999 (per step)     |

Go 的 executor 允许单个 plan step 执行近百万次迭代，存在失控风险。

### 20. 默认文件目录不一致

| 项目    | 默认目录      |
| ------- | ------------- |
| Next.js | `./data/docs` |
| Go      | `./docs`      |

### 21. CORS 配置差异

Go (`app.go:42-43`):

```
Allow-Methods: GET,POST,PUT,DELETE,PATCH,OPTIONS
Allow-Headers: Authorization,Content-Type
```

Next.js (各 route.ts):

```
Allow-Methods: POST, OPTIONS
Allow-Headers: Content-Type
```

Go 开放了更多方法和 Authorization header，安全风险略高。

### 22. 错误响应格式不一致

Next.js `/api/chat` 返回详细的错误信息 (`route.ts:47-59`):

```json
{
  "message": "{\"name\":\"...\",\"message\":\"...\",\"statusCode\":...,\"url\":\"...\",\"responseBody\":\"...\"}",
  "data": null
}
```

Go 返回简单错误信息:

```json
{
  "message": "error description",
  "data": null
}
```

前端如果解析 Go 的错误响应，预期的是 Next.js 格式的详细 JSON 字符串。

### 23. Date 格式差异 (Chat System Prompt)

| 项目    | Date 格式                                  | 示例                    |
| ------- | ------------------------------------------ | ----------------------- |
| Next.js | `new Date().toLocaleString("en-US")`       | `7/15/2026, 3:12:38 PM` |
| Go      | `time.Now().Format("2006-01-02 15:04:05")` | `2026-07-15 15:12:38`   |

### 24. Go 缺少 DuckDuckGo Search Tool 的实际接入

`internal/ai/agent/chat_pipeline/search_tool.go` 定义了 `newSearchTool()` 但未在任何 pipeline 中调用。Next.js 的 system prompt 提及 "Search the web for information" 但也未实际接入搜索工具 — 此处两边一致，但 Go 存在死代码。

---

## 四、缺失基础设施 (P3)

### 25. 无测试文件

Go 项目不存在任何 `*_test.go` 文件。Next.js 项目同样缺少测试，但有 ESLint 配置。

### 26. 无 README.md

Go 项目根目录缺少 README 文件。

### 27. 无 Docker 配置

Go 项目缺少 `Dockerfile` 和 `docker-compose.yml`。Next.js 项目有完整的 `docker-compose.yml` (Redis Stack + Prometheus + Grafana)。

### 28. 无 Makefile

Go 项目缺少统一的构建入口。

### 29. 对话记忆无持久化

两个项目均使用内存存储对话历史，服务重启后丢失。Go 项目的 bug.md 已标记此问题。

### 30. 无结构化日志

Go 项目使用 `fmt.Printf`、`log.Printf`、`log_callback` 三种日志方式混用，无统一级别和格式。

---

## 五、总结

### 严重 Bug (需立即修复)

| #   | 问题                                               | 影响                                      |
| --- | -------------------------------------------------- | ----------------------------------------- |
| 1   | chatRequest JSON tag PascalCase                    | chat/chat_stream 两个 endpoint 完全不可用 |
| 2   | System prompt "markdown only" vs "plain text only" | AI 回复格式矛盾                           |
| 3   | log.Fatal 在工具回调中                             | 任何工具失败都会终止整个服务              |
| 4   | mysql_crud stdin 阻塞                              | HTTP 模式下调用永久挂起                   |

### 数据层不兼容 (需统一)

| #   | 问题                                      | 影响                            |
| --- | ----------------------------------------- | ------------------------------- |
| 5   | Redis key prefix `doc:` vs `biz:`         | 数据不互通                      |
| 6   | Vector field `vector_content` vs `vector` | 检索无法命中                    |
| 7   | Index schema vector field name            | 两个项目各自创建的 index 不兼容 |

### 功能缺失 (需要对齐)

| #   | 问题                                 |
| --- | ------------------------------------ |
| 8-9 | Prometheus 工具死代码 + 缺少配置项   |
| 10  | Log topic 硬编码                     |
| 12  | 删除去重无分布式锁                   |
| 14  | 缺少 Ollama embedding 支持           |
| 16  | Memory 无 LRU 淘汰                   |
| 17  | Plan executor 缺少 MySQL 工具        |
| 19  | Executor MaxIterations 过大 (999999) |

### 统计

- 严重 Bug: 4 个
- 重要差异: 7 个
- 功能未对齐: 13 个
- 缺失基础设施: 6 个
- 总计: 30 项
