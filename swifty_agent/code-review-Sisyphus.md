# Swifty Agent Migration Code Review: Next.js -> Go

Reviewer: Sisyphus
Date: 2026-07-15

Scope: Next.js backend (`/apps/swifty-agent/`) vs Go project (`/swifty_agent/`), excluding frontend.

---

## Executive Summary

Go project migrates 4 API routes, 3 AI pipelines, 5 tools, and supporting infrastructure from Next.js. The migration is structurally complete but has 6 P0 critical bugs that will crash the server or produce incorrect results, 13 P1 behavioral mismatches that break parity with the Next.js version, and a complete lack of deployment infrastructure. The bug.md file in the Go project partially catalogues these issues but misses several critical items found in this review.

Total issues found: 31
P0 Critical: 7
P1 High: 10
P2 Medium: 7
P3 Low: 7

---

## Legend

[BUG] Code defect causing crash, hang, or incorrect behavior
[MISMATCH] Behavioral difference from the Next.js source
[MISSING] Feature/config/infrastructure not yet migrated
[DEAD] Unused code with no runtime effect

---

## P0 -- Critical Bugs (Server Crash, Data Corruption, Complete Feature Failure)

### P0-1. chatRequest JSON field casing mismatch with frontend

[BUG] Go `internal/app/chat_handler.go:20-22`

```go
type chatRequest struct {
    ID       string `json:"Id"`
    Question string `json:"Question"`
}
```

Next.js `app/api/chat/route.ts:6-8`:

```ts
const chatRequestSchema = z.object({
  id: z.string().min(1),
  question: z.string().min(1),
});
```

The frontend sends `{ "id": "...", "question": "..." }` with lowercase keys. Go expects `"Id"` / `"Question"` with uppercase. If the frontend is shared between both backends (or if the Go project replaces the Next.js one), every chat and chat_stream request will fail with `invalid request body`.

Impact: All chat functionality non-functional when called from the Next.js frontend.

### P0-2. System prompt contradicts Next.js on markdown output requirement

[MISMATCH] Go `internal/ai/agent/chat_pipeline/prompt.go:53`

```
Output must not contain markdown syntax; use plain text only
```

Next.js `lib/ai/pipelines/chat.ts:39`:

```
Output markdown only
```

These are exact opposites. The Go version forces plain-text output, the Next.js version requires markdown. LLM behavior will be fundamentally different. Any frontend that renders markdown will show raw text when connected to the Go backend.

Impact: Different response format, breaks frontend markdown rendering assumption.

### P0-3. All tool functions use log.Fatal -> kills HTTP server

[BUG] 6 files in `internal/ai/tools/`

Every tool creation function calls `log.Fatal(err)` on `InferOptionableTool` failure. Every tool callback function calls `log.Fatal(err)` on internal errors. `log.Fatal` calls `os.Exit(1)`, which terminates the entire HTTP server process.

Affected locations:

| File                    | Line | Trigger                     |
| ----------------------- | ---- | --------------------------- |
| mysql_crud.go           | 34   | gorm.Open failure           |
| mysql_crud.go           | 47   | db.Exec failure             |
| mysql_crud.go           | 53   | db.Raw.Scan failure         |
| mysql_crud.go           | 62   | InferOptionableTool failure |
| get_current_time.go     | 54   | InferOptionableTool failure |
| query_internal_docs.go  | 29   | NewRedisRetriever failure   |
| query_internal_docs.go  | 33   | rr.Retrieve failure         |
| query_internal_docs.go  | 40   | InferOptionableTool failure |
| query_metrics_alerts.go | 158  | InferOptionableTool failure |

Impact: Any tool error crashes the server. Single bad LLM tool call = full downtime.

### P0-4. mysql_crud tool blocks on stdin -> hangs all requests in HTTP mode

[BUG] `internal/ai/tools/mysql_crud.go:38-44`

```go
scanner := bufio.NewScanner(os.Stdin)
fmt.Print("\nConfirm SQL execution (y/n): ", input.SQL)
scanner.Scan()
```

In HTTP server mode, `os.Stdin` is not connected to a terminal. `scanner.Scan()` blocks indefinitely. When the LLM agent calls this tool, the goroutine hangs permanently. Over time with repeated tool calls, goroutine accumulation will exhaust memory.

Next.js `lib/ai/tools/operations.ts:135-152` removes the interactive prompt entirely and executes SQL directly.

Impact: Every mysql_crud tool invocation permanently leaks a goroutine.

### P0-5. queryPrometheusAlerts() returns empty -- tool is completely non-functional

[BUG] `internal/ai/tools/query_metrics_alerts.go:54-56`

```go
func queryPrometheusAlerts() (PrometheusAlertsResult, error) {
    return PrometheusAlertsResult{}, nil
    // FIXME: unreachable
    baseURL := "http://127.0.0.1:9090"
    ...
}
```

The function returns empty before reaching the HTTP call. All code below line 56 is dead code. The AI_OPS_QUERY prompt instructs the agent to "First call the tool query_prometheus_alerts to get all active alerts" -- this will always return zero alerts. The entire AI Ops pipeline produces empty/meaningless reports.

Next.js `lib/ai/tools/operations.ts:62-64` makes an actual HTTP call to `${config.prometheusBaseUrl}/api/v1/alerts`.

Impact: /api/ai_ops endpoint always produces empty/garbage reports.

### P0-6. Go stores assistant response as SystemMessage instead of AssistantMessage

[BUG] `internal/app/chat_handler.go:52-53`

```go
mem.Get(req.ID).Append(schema.UserMessage(req.Question))
mem.Get(req.ID).Append(schema.SystemMessage(out.Content))  // BUG: should be AssistantMessage
```

Next.js `lib/ai/pipelines/chat.ts:78-79`:

```ts
mem.setMessages({ role: "user", content: question });
mem.setMessages({ role: "assistant", content: answer });
```

Assistant responses are stored with role `system` instead of `assistant`. When these messages are fed back into the chat pipeline as conversation history, the LLM misinterprets assistant responses as system instructions, corrupting multi-turn context.

Impact: Multi-turn conversations produce increasingly degraded/incorrect responses.

### P0-7. Redis key prefix mismatch: "biz:" (Next.js) vs "doc:" (Go) -- incompatible data

[MISMATCH] Go `internal/consts/consts.go:6` vs Next.js `lib/config.ts:48`

| Constant     | Next.js  | Go               |
| ------------ | -------- | ---------------- |
| Key prefix   | `biz:`   | `doc:`           |
| Vector field | `vector` | `vector_content` |

Data written by Next.js cannot be read by Go and vice versa. If both projects point at the same Redis instance, they see each other as empty. The indexer uses `doc:` prefix, but the retriever searches the same index -- the data is self-consistent within Go, but completely isolated from the Next.js data.

Impact: Zero interoperability between the two backends when sharing Redis.

---

## P1 -- High-Severity Mismatches (Functional Impact)

### P1-1. System prompt hardcoded log topic values in Go

[MISMATCH] Go `internal/ai/agent/chat_pipeline/prompt.go:43`

```go
* Log topic region: ap-guangzhou; log topic ID: 869830db-a055-4479-963b-3c898d27e755
```

This is hardcoded in the system prompt source code.

Next.js `lib/ai/pipelines/chat.ts:12-17`:

```ts
const LOG_TOPIC_REGION = process.env.LOG_TOPIC_REGION ?? "";
const LOG_TOPIC_ID = process.env.LOG_TOPIC_ID ?? "";
const logTopicLine =
  LOG_TOPIC_REGION && LOG_TOPIC_ID
    ? `  - Log topic region: ${LOG_TOPIC_REGION}; log topic id: ${LOG_TOPIC_ID}`
    : "";
```

Next.js reads these from environment variables and conditionally includes the line only when both values are set. Go always includes the line with hardcoded Guangzhou values.

Impact: Go always prompts the LLM with Guangzhou-specific log context, even when the deployment serves a different region.

### P1-2. AI_OPS_QUERY prompt text differs between projects

[MISMATCH] Go `internal/app/ai_ops_handler.go:12-26` vs Next.js `lib/ai/pipelines/plan-execute-replan/index.ts:18-31`

Key differences:

| Aspect         | Next.js                                            | Go                         |
| -------------- | -------------------------------------------------- | -------------------------- |
| Title prefix   | "AI Ops"                                           | "AI Ops -- "               |
| Step 1 wording | "analyze the information retrieved for each alert" | (omitted)                  |
| Report heading | "Alert Handling Details"                           | "Alert Processing Details" |
| Section 3      | "Handling Procedure Execution N"                   | "Root Cause Analysis N"    |
| Section 4      | (omitted)                                          | "Processing Plan N"        |

The Go version produces a report with different structure (includes "Root Cause Analysis" and "Processing Plan" as separate sections, Next.js has only "Handling Procedure Execution"). Reports from the two backends are structurally different.

### P1-3. Executor MaxIterations: 999999 (Go) vs 10 (Next.js)

[MISMATCH] Go `internal/ai/agent/plan_execute_replan/executor.go:40`

```go
return plan_execute.NewExecutor(ctx, &plan_execute.ExecutorConfig{
    ...
    MaxIterations: 999999,
})
```

Next.js `lib/ai/pipelines/plan-execute-replan/executor.ts:18`:

```ts
stopWhen: isStepCount(10),
```

The Go executor allows up to 999999 iterations per plan step. Next.js limits to 10. A single poorly-specified plan step could trigger a near-infinite loop of LLM calls, consuming API credits and potentially hanging the request.

Impact: Runaway API cost, request timeout, potential resource exhaustion.

### P1-4. Memory: no LRU eviction in Go -- unbounded session growth

[MISMATCH] Go `internal/utility/mem/mem.go:13-14`

```go
memMap = make(map[string]*ConversationMemory)
mu     sync.Mutex
```

No maximum session count. New sessions accumulate indefinitely.

Next.js `lib/memory.ts:10-28` implements LRU eviction capped at `MAX_SESSIONS = 100`, using Map insertion-order re-insertion on access.

Impact: Long-running Go server with many unique session IDs will grow memory unbounded until OOM.

### P1-5. Go has no distributed lock for deleteBySource in knowledge index

[MISMATCH] Go `internal/ai/agent/knowledge_index_pipeline/index_file.go:48-65`

Go does a single FT.SEARCH + DEL in one batch:

```go
res, err := client.Do(ctx, "FT.SEARCH", consts.RedisIndexName, query,
    "NOCONTENT", "LIMIT", "0", "10000").Slice()
```

Next.js `lib/redis/indexer.ts:46-76` has:

- SETNX distributed lock with 30s TTL to prevent concurrent deletion races
- Batched deletion in groups of 1000 to handle large document sets
- Proper error handling (warn log, not crash)

Go's approach is not just simpler but actively dangerous:

1. Two simultaneous uploads of the same file will race on delete-then-insert
2. Documents with more than 10000 chunks will have orphaned chunks left behind
3. No lock prevents concurrent operations from interleaving

Impact: Race conditions on re-upload, orphaned data for large files.

### P1-6. No content length cap in Go indexer

[MISSING] Go indexer does not truncate document content.

Next.js `lib/redis/indexer.ts:17` caps at `MAX_CONTENT_LENGTH = 8192`:

```ts
content: chunks[i].content.slice(0, MAX_CONTENT_LENGTH),
```

Go Eino Redis indexer stores content as-is with no truncation. Very large chunks (e.g., a long markdown section without any `#` header break) will be stored in full. This may cause embedding dimension mismatches or exceed Redis value size limits.

### P1-7. SSE streaming: no `id` field, different event payload shapes

[MISMATCH] Go `internal/app/chat_handler.go:74-75,120-121`

```go
sse.Event("connected", `{"status":"connected","client_id":"`+req.ID+`"}`)
sse.Event("message", chunk.Content)
sse.Event("done", "Stream completed")
```

Next.js `app/api/chat_stream/route.ts:35-43`:

```ts
const send = (event: string, data: string) => {
  controller.enqueue(
    encoder.encode(`id: ${Date.now()}\nevent: ${event}\ndata: ${data}\n\n`),
  );
};
send("connected", JSON.stringify({ status: "connected", client_id: id }));
send("done", "Stream completed");
```

Differences:

1. Go lacks `id:` field in SSE frames (Next.js emits `id: ${Date.now()}`)
2. Connected event payload format is the same (JSON), but Go uses string concatenation while Next.js uses `JSON.stringify`. If `req.ID` contains quotes or special chars, Go's output breaks.
3. If the frontend SSE parser expects `id` fields for reconnection/resume, Go's stream won't work correctly.

Impact: SSE may work for simple clients but could fail with stricter SSE parsers.

### P1-8. Error response for /api/chat: Next.js returns detailed diagnostics, Go returns bare message

[MISMATCH] Go `internal/app/chat_handler.go:46-49`

```go
ctx.Throw(http.StatusInternalServerError, err.Error())
```

Next.js `app/api/chat/route.ts:33-61` returns structured error diagnostics:

```ts
message: JSON.stringify({
    name: err?.name,
    message: err?.message ?? String(e),
    statusCode: err?.statusCode,
    url: err?.url,
    responseBody: err?.responseBody,
    responseHeaders: err?.responseHeaders,
}),
```

When an upstream LLM API returns an error (e.g., rate limit, invalid API key, model not found), the Next.js version gives the client rich diagnostic information. The Go version strips all of this to a bare error string. Frontend error handling that parses the message field will break.

Impact: Debugging LLM errors through the frontend becomes much harder.

### P1-9. File loader source metadata: basename (Next.js) vs full URI (Go)

[MISMATCH] Next.js `lib/ai/loader.ts:13`:

```ts
const source = path.basename(filePath);
```

Go uses Eino `file.NewFileLoader` which sets document metadata `URI` to the full path. When used for `_source` deduplication in `index_file.go:47`:

```go
source, _ := docs[0].MetaData[consts.RedisSourceField].(string)
```

If Eino sets `_source` to the full path (e.g., `/data/docs/alert-guide.md`) instead of just the basename (`alert-guide.md`), then the deduplication query `@_source:{/data/docs/alert-guide.md}` will not match records stored by Next.js where `_source` = `alert-guide.md`.

Impact: Re-indexing a file previously indexed by Next.js will not delete old records, creating duplicates.

### P1-10. Missing Prometheus URL in Go configuration

[MISSING] Go `internal/config/config.go` -- Config struct has no `PrometheusBaseUrl` field. The Prometheus URL `http://127.0.0.1:9090` is hardcoded in the (currently dead) code at `query_metrics_alerts.go:58`.

Next.js `lib/config.ts:55`: `prometheusBaseUrl: process.env.PROMETHEUS_BASE_URL ?? "http://127.0.0.1:9090"`.

When the Prometheus tool is re-enabled, the URL must be configurable. Currently it cannot be changed without recompiling.

---

## P2 -- Medium-Severity Issues

### P2-1. No Ollama embedding support in Go

[MISSING] Go `internal/ai/embedder/embedder.go` only supports DashScope:

```go
return dashscope.NewEmbedder(ctx, &dashscope.EmbeddingConfig{...})
```

Next.js `lib/ai/embedder.ts` switches between DashScope (2048d) and Ollama (768d) via `EMBEDDING_PROVIDER` env var. Go cannot use local Ollama embeddings.

### P2-2. No .env.example / config template documentation

[MISSING] Go project has no equivalent of Next.js `.env.example`. The `config.json` contains placeholder `"your-api-key"` values but no documented schema of all available options, defaults, or usage instructions.

### P2-3. MCP tool client: no caching, no graceful degradation

[MISMATCH] Go `internal/ai/tools/mcp_tool.go` creates a new SSE MCP client on every call to `GetLogMcpTool`. Next.js `lib/ai/tools/query-log.ts:9-10` caches both the client and the tools:

```ts
let cachedClient: Client | null = null;
let cachedTools: Record<string, Tool> | null = null;
```

Go also does not gracefully handle MCP server unavailability. If the MCP server is down, `GetLogMcpTool` returns an error that propagates up and fails the entire agent build. Next.js catches the error, logs a warning, and returns an empty tool set.

Impact: MCP server downtime takes down all chat functionality in Go. Next.js continues working without log tools.

### P2-4. Go does not set CORS headers on SSE streaming responses

[MISMATCH] Go `internal/app/chat_handler.go` handleChatStream uses `ctx.SSE()` which sets `Content-Type: text/event-stream` but does not explicitly add CORS headers. The CORS middleware in `app.go:41-43` adds headers via `ctx.Set()` which writes to response headers, so this may work if the middleware runs before SSE starts -- but the SSE writer may have already flushed headers.

Next.js `app/api/chat_stream/route.ts:52-58` explicitly spreads CORS_HEADERS into the SSE Response headers object.

Impact: CORS may or may not work for SSE depending on timing of middleware vs SSE header flush.

### P2-5. No request body validation in Go handlers

[MISMATCH] Go uses `ctx.BindJSON(&req)` which only checks JSON parsing. It does not validate that `id` and `question` are non-empty strings.

Next.js uses Zod schemas with `.min(1)` validation:

```ts
const chatRequestSchema = z.object({
  id: z.string().min(1),
  question: z.string().min(1),
});
```

Go accepts `{ "Id": "", "Question": "" }` and proceeds to invoke the LLM with an empty query.

### P2-6. Variable name `config` shadows imported package name

[DEAD] `internal/ai/agent/chat_pipeline/tools_node.go:16`

```go
func newReactAgentLambda(ctx context.Context, cfg *config.Config) (*compose.Lambda, error) {
    config := &react.AgentConfig{  // shadows imported config package
```

The local variable `config` shadows the `config` package import. Not a runtime bug but a maintenance hazard.

### P2-7. DuckDuckGo search tool is dead code

[DEAD] `internal/ai/agent/chat_pipeline/search_tool.go`

`newSearchTool()` is defined but never called by any pipeline. The Next.js system prompt mentions "Search the web for information" as a core capability, and the Go prompt says "Web search for information retrieval" -- but neither project actually includes a web search tool in the runtime pipeline. The Go version imports `duckduckgo` as a dependency for nothing.

---

## P3 -- Low-Severity / Missing Infrastructure

### P3-1. No docker-compose.yml

[MISSING] Next.js provides `docker-compose.yml` with 3 services: redis-stack, prometheus, grafana. Go project has nothing. A developer cloning the Go project cannot start the required infrastructure.

### P3-2. No Makefile

[MISSING] No standard build/run/test entry points. Next.js uses npm scripts (next dev, next build, etc.).

### P3-3. No Dockerfile

[MISSING] No container build definition for the Go binary. Next.js uses the Next.js Docker integration.

### P3-4. No README.md

[MISSING] No project documentation, setup guide, or API reference.

### P3-5. No test files

[MISSING] Zero `*_test.go` files in the entire project. All testing relies on 5 CLI commands in `cmd/`.

### P3-6. Unstructured logging across 3 different systems

[MISSING] Logs use a mix of `fmt.Printf`, `log.Printf`, and `log_callback` with no unified format, level system, or output target. Next.js uses `console.error`/`console.warn`/`console.log` consistently.

### P3-7. Memory returns direct slice reference -- external mutation risk

[BUG] `internal/utility/mem/mem.go:63-66`

```go
func (m *ConversationMemory) All() []*schema.Message {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.Messages
}
```

Returns the internal slice directly. The caller (`chat_handler.go:37`) passes this to the Eino pipeline which may modify the slice contents. Should return a copy: `return append([]*schema.Message(nil), m.Messages...)`.

---

## File Mapping Reference

| Next.js                               | Go                                                 | Status                    |
| ------------------------------------- | -------------------------------------------------- | ------------------------- |
| app/api/chat/route.ts                 | internal/app/chat_handler.go:handleChat            | Aligned with bugs         |
| app/api/chat_stream/route.ts          | internal/app/chat_handler.go:handleChatStream      | Aligned with bugs         |
| app/api/upload/route.ts               | internal/app/file_handler.go:handleFileUpload      | Aligned                   |
| app/api/ai_ops/route.ts               | internal/app/ai_ops_handler.go:handleAIOps         | Aligned                   |
| lib/config.ts                         | internal/config/config.go + config.json            | Partially aligned         |
| lib/memory.ts                         | internal/utility/mem/mem.go                        | Missing LRU               |
| lib/ai/models.ts                      | internal/ai/models/models.go                       | Aligned                   |
| lib/ai/embedder.ts                    | internal/ai/embedder/embedder.go                   | Missing Ollama            |
| lib/ai/loader.ts                      | internal/ai/loader/loader.go                       | Aligned                   |
| lib/ai/callbacks.ts                   | internal/utility/log_callback/log_callback.go      | Aligned                   |
| lib/ai/tools/schemas.ts               | (embedded in Go jsonschema tags)                   | Implicit                  |
| lib/ai/tools/operations.ts            | internal/ai/tools/\*.go                            | Buggy                     |
| lib/ai/tools/index.ts                 | internal/ai/agent/chat_pipeline/tools_node.go      | Aligned                   |
| lib/ai/tools/query-log.ts             | internal/ai/tools/mcp_tool.go                      | Missing cache+degradation |
| lib/ai/pipelines/chat.ts              | internal/ai/agent/chat_pipeline/                   | Prompt mismatch           |
| lib/ai/pipelines/knowledge-index.ts   | internal/ai/agent/knowledge_index_pipeline/        | Aligned                   |
| lib/ai/pipelines/plan-execute-replan/ | internal/ai/agent/plan_execute_replan/             | Executor iter mismatch    |
| lib/redis/client.ts                   | internal/utility/redis/redis.go                    | Prefix mismatch           |
| lib/redis/indexer.ts                  | internal/ai/indexer/indexer.go                     | Missing lock + cap        |
| lib/redis/retriever.ts                | internal/ai/retriever/retriever.go                 | Vector field mismatch     |
| lib/schemas.ts                        | (not applicable -- server-side validation differs) | N/A                       |
| .env.example                          | (not present)                                      | Missing                   |
| docker-compose.yml                    | (not present)                                      | Missing                   |
| prometheus.yml                        | (not present)                                      | Missing                   |

---

## Recommended Fix Priority

1. P0-1: Fix JSON field tags to lowercase `id`/`question` (1 line change, fixes all chat)
2. P0-3: Replace all `log.Fatal` with `return "", err` (9 locations, prevents server crashes)
3. P0-4: Remove stdin confirmation from mysql_crud (delete lines 38-44)
4. P0-5: Remove early return in queryPrometheusAlerts, add config for URL (restores tool)
5. P0-6: Change `schema.SystemMessage` to `schema.AssistantMessage` in chat_handler.go (1 line)
6. P0-2: Fix system prompt output requirement to match Next.js (1 line)
7. P0-7: Align Redis key prefix and vector field with Next.js (2 consts)
8. P1-1: Move hardcoded log topic values to config.json
9. P1-3: Change MaxIterations from 999999 to 10 (1 line)
10. P1-4: Add LRU eviction to memory (20 lines)
11. P3-1: Create docker-compose.yml by porting from Next.js
12. P3-4: Create README.md
