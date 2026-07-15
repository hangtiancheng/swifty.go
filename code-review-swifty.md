# Swifty Agent Migration Review: Next.js → Go

**Review Date**: 2026-07-15
**Reviewer**: Swifty + Agent Team (api-reviewer, ai-pipeline-reviewer, infra-reviewer)
**Scope**: Backend migration from Next.js (`/apps/swifty-agent`) to Go (`/swifty_agent`)

---

## Executive Summary

The migration has **critical bugs** that will cause runtime failures, **multiple misalignments** with the Next.js source project, and **missing infrastructure**. The most severe issues are:

1. **JSON field name mismatch** causing request parsing failures
2. **`log.Fatal` usage in tools** causing HTTP service crashes
3. **stdin blocking in mysql_crud** causing tool calls to hang
4. **Prometheus tool completely disabled** with dead code
5. **Hardcoded configuration values** instead of environment-based configuration

---

## Critical Issues (P0)

### 1. JSON Field Name Mismatch in Chat Handler

**File**: `internal/app/chat_handler.go:20-22`
**Severity**: Critical
**Description**: The `chatRequest` struct uses uppercase JSON tags (`Id`, `Question`), but the Next.js project sends lowercase (`id`, `question`). This causes request parsing to fail with 400 errors.

```go
type chatRequest struct {
    ID       string `json:"Id"`       // ❌ Should be "id"
    Question string `json:"Question"` // ❌ Should be "question"
}
```

**Fix**: Change JSON tags to lowercase:

```go
type chatRequest struct {
    ID       string `json:"id"`
    Question string `json:"question"`
}
```

---

### 2. log.Fatal Usage in Tool Functions

**Files**:

- `internal/ai/tools/mysql_crud.go:34, 47, 53, 62`
- `internal/ai/tools/get_current_time.go:54`
- `internal/ai/tools/query_internal_docs.go:29, 33, 40`
- `internal/ai/tools/query_metrics_alerts.go:158`

**Severity**: Critical
**Description**: Tool functions are invoked as callbacks during AI agent runtime. Using `log.Fatal` directly terminates the entire HTTP service process. This must be changed to `return "", err`.

**Fix**: Replace all `log.Fatal(err)` with `return "", err`:

```go
// Before
if err != nil {
    log.Fatal(err)
}

// After
if err != nil {
    return "", fmt.Errorf("tool operation failed: %w", err)
}
```

---

### 3. stdin Blocking in mysql_crud Tool

**File**: `internal/ai/tools/mysql_crud.go:38-44`
**Severity**: Critical
**Description**: The tool uses `bufio.NewScanner(os.Stdin)` to read user confirmation. In HTTP service mode, stdin is unavailable, causing tool calls to hang indefinitely.

```go
scanner := bufio.NewScanner(os.Stdin)
fmt.Print("\nConfirm SQL execution (y/n): ", input.SQL)
scanner.Scan()
```

**Fix**: Remove stdin prompt, execute directly (web version removes interactive confirmation):

```go
// Remove lines 38-44, execute SQL directly
if err := db.Exec(input.SQL).Error; err != nil {
    return "", fmt.Errorf("SQL execution failed: %w", err)
}
```

---

### 4. Prometheus Tool Completely Disabled

**File**: `internal/ai/tools/query_metrics_alerts.go:54-81`
**Severity**: Critical
**Description**: The `queryPrometheusAlerts()` function returns empty results at line 56, with all subsequent code marked as unreachable (`// FIXME: unreachable`). The Prometheus alert query functionality is completely unavailable.

```go
func queryPrometheusAlerts() (PrometheusAlertsResult, error) {
    return PrometheusAlertsResult{}, nil  // ❌ Line 56: early return
    // FIXME: unreachable               // Line 57+
    baseURL := "http://127.0.0.1:9090"
    ...
}
```

**Fix**: Remove the early return or make it configurable:

```go
func queryPrometheusAlerts() (PrometheusAlertsResult, error) {
    // Remove line 56, or add config toggle
    if !config.EnablePrometheus {
        return PrometheusAlertsResult{}, nil
    }
    baseURL := "http://127.0.0.1:9090"
    ...
}
```

---

## High Severity Issues (P1)

### 5. Hardcoded Log Topic Values in System Prompt

**File**: `internal/ai/agent/chat_pipeline/prompt.go:42`
**Severity**: High
**Description**: The system prompt hardcodes `Log topic region: ap-guangzhou; log topic ID: 869830db-a055-4479-963b-3c898d27e755`, while the Next.js project reads these from environment variables (`LOG_TOPIC_REGION`, `LOG_TOPIC_ID`).

**Next.js version** (`lib/ai/pipelines/chat.ts:12-17`):

```typescript
const LOG_TOPIC_REGION = process.env.LOG_TOPIC_REGION ?? "";
const LOG_TOPIC_ID = process.env.LOG_TOPIC_ID ?? "";
const logTopicLine =
  LOG_TOPIC_REGION && LOG_TOPIC_ID
    ? `  • Log topic region: ${LOG_TOPIC_REGION}; log topic id: ${LOG_TOPIC_ID}`
    : "";
```

**Fix**: Add to `config.Config`:

```go
type Config struct {
    ...
    LogTopicRegion string `json:"log_topic_region"`
    LogTopicID     string `json:"log_topic_id"`
}
```

Then update `prompt.go` to use template substitution:

```go
var systemPromptTemplate = `
...
  * Log topic region: {{.LogTopicRegion}}; log topic ID: {{.LogTopicID}}
...
`
```

---

### 6. Output Format Mismatch in System Prompt

**File**: `internal/ai/agent/chat_pipeline/prompt.go:53`
**Severity**: High
**Description**: The Go project requires "Output must not contain markdown syntax; use plain text only", while the Next.js project requires "Output markdown only". This causes inconsistent response formatting.

**Go version**:

```
* Output must not contain markdown syntax; use plain text only
```

**Next.js version**:

```
* Output markdown only
```

**Fix**: Align with Next.js:

```go
* Output markdown only
```

---

### 7. File Directory Default Mismatch

**Files**:

- `internal/config/config.go:105`
- `config.json:20`
- `lib/config.ts:53`

**Severity**: High
**Description**: The Go project defaults to `./docs`, while the Next.js project defaults to `./data/docs`.

**Go**: `./docs`
**Next.js**: `./data/docs`

**Fix**: Update `config.go:105`:

```go
if cfg.FileDir == "" {
    cfg.FileDir = "./data/docs"  // Align with Next.js
}
```

---

### 8. Model Name Mismatch in Config

**File**: `config.json:7, 12`
**Severity**: High
**Description**: The config uses `deepseek-v3-2-251201`, while the Next.js project uses `openai-v3-2-251201`.

**Go config.json**:

```json
"model": "deepseek-v3-2-251201"
```

**Next.js lib/config.ts:9**:

```typescript
model: process.env.OPENAI_THINK_MODEL ?? "openai-v3-2-251201";
```

**Fix**: Update `config.json`:

```json
"model": "openai-v3-2-251201"
```

---

## Medium Severity Issues (P2)

### 9. Redis Vector Field Name Mismatch

**Files**:

- `internal/consts/consts.go:16`
- `lib/redis/indexer.ts:30`

**Severity**: Medium
**Description**: The Go project uses `vector_content` as the Redis vector field name, while the Next.js project uses `vector`. This causes indexing and retrieval incompatibilities.

**Go** (`consts.go:16`):

```go
RedisVectorField = "vector_content"
```

**Next.js** (`indexer.ts:30`):

```typescript
pipeline.hSet(key, {
  vector: float32ToBuffer(vectors[i]),  // Field name is "vector"
  ...
});
```

**Fix**: Align field names. Update `consts.go:16`:

```go
RedisVectorField = "vector"
```

---

### 10. Missing LRU Eviction in Memory Management

**File**: `internal/utility/mem/mem.go`
**Severity**: Medium
**Description**: The Go project's memory implementation has no session limit, while the Next.js project implements LRU eviction with a maximum of 100 sessions. This can lead to unbounded memory growth.

**Next.js version** (`lib/memory.ts:10-28`):

```typescript
const MAX_SESSIONS = 100;
const memoryMap = new Map<string, SimpleMemory>();

export function getSimpleMemory(id: string): SimpleMemory {
  const existing = memoryMap.get(id);
  if (existing) {
    // Move to end (most recently used)
    memoryMap.delete(id);
    memoryMap.set(id, existing);
    return existing;
  }
  // Evict oldest session if at capacity
  if (memoryMap.size >= MAX_SESSIONS) {
    const oldestKey = memoryMap.keys().next().value;
    if (oldestKey !== undefined) memoryMap.delete(oldestKey);
  }
  const mem = new SimpleMemory(id);
  memoryMap.set(id, mem);
  return mem;
}
```

**Fix**: Add LRU eviction to `mem.go`:

```go
const MaxSessions = 100

var (
    memMap = make(map[string]*ConversationMemory)
    order  []string  // Track access order
    mu     sync.Mutex
)

func Get(id string) *ConversationMemory {
    mu.Lock()
    defer mu.Unlock()

    if m, ok := memMap[id]; ok {
        // Move to end (most recently used)
        for i, k := range order {
            if k == id {
                order = append(order[:i], order[i+1:]...)
                break
            }
        }
        order = append(order, id)
        return m
    }

    // Evict oldest session if at capacity
    if len(memMap) >= MaxSessions {
        oldest := order[0]
        order = order[1:]
        delete(memMap, oldest)
    }

    m := &ConversationMemory{
        ID:            id,
        Messages:      []*schema.Message{},
        MaxWindowSize: 6,
    }
    memMap[id] = m
    order = append(order, id)
    return m
}
```

---

### 11. Missing Redis Reconnection Mechanism

**File**: `internal/utility/redis/redis.go`
**Severity**: Medium
**Description**: The Go project lacks Redis auto-reconnect, while the Next.js project configures `reconnectStrategy` with exponential backoff.

**Next.js version** (`lib/redis/client.ts:26-31`):

```typescript
const client = createClient({
  url: config.redis.url,
  socket: {
    reconnectStrategy: (retries: number) => Math.min(retries * 100, 5000),
  },
});
```

**Fix**: Add reconnection logic to `redis.go`:

```go
func NewClient(ctx context.Context, cfg *config.Config) (*redis.Client, error) {
    client := redis.NewClient(&redis.Options{
        Addr:            cfg.Redis.Addr,
        Password:        cfg.Redis.Password,
        DB:              cfg.Redis.DB,
        Protocol:        2,
        MaxRetries:      3,
        MinRetryBackoff: 100 * time.Millisecond,
        MaxRetryBackoff: 5 * time.Second,
    })
    ...
}
```

---

### 12. Missing Redis Index Dimension Mismatch Detection

**File**: `internal/utility/redis/redis.go:36-57`
**Severity**: Medium
**Description**: The Go project does not check if the existing index dimension matches the embedding dimension. If the embedding provider is changed (e.g., dashscope 2048d → ollama 768d) without dropping the index, searches silently fail.

**Next.js version** (`lib/redis/client.ts:39-69`):

```typescript
// P1-7 fix: verify the existing index's vector dimension matches EMBEDDING_DIM.
const info = await client.ft.info(config.redis.indexName);
const attrs = info.attributes ?? [];
const vectorAttr = attrs.find((a) => a.attribute === "vector");
if (vectorAttr) {
  const storedDim = Number(vectorAttr.DIM ?? 0);
  if (storedDim && storedDim !== EMBEDDING_DIM) {
    console.warn(
      `Index dimension mismatch: stored=${storedDim}, expected=${EMBEDDING_DIM}. Dropping and recreating index.`,
    );
    await client.ft.dropIndex(config.redis.indexName);
    indexExists = false;
  }
}
```

**Fix**: Add dimension check to `ensureIndex`:

```go
func ensureIndex(ctx context.Context, client *redis.Client, cfg *config.Config) error {
    info, err := client.Do(ctx, "FT.INFO", consts.RedisIndexName).Result()
    if err == nil {
        // Index exists, check dimension
        infoMap := info.([]interface{})
        for i := 0; i < len(infoMap); i += 2 {
            if key, ok := infoMap[i].(string); ok && key == "attributes" {
                // Parse attributes and check dimension
                // If mismatch, drop and recreate
            }
        }
        return nil
    }
    // Create new index
    ...
}
```

---

### 13. Variable Name Shadowing in tools_node.go

**File**: `internal/ai/agent/chat_pipeline/tools_node.go:16`
**Severity**: Medium
**Description**: The local variable `config` shadows the imported `config` package name, creating maintenance risk.

```go
func newReactAgentLambda(ctx context.Context, cfg *config.Config) (*compose.Lambda, error) {
    config := &react.AgentConfig{  // ❌ Shadows package name
        MaxStep:            25,
        ...
    }
    ...
}
```

**Fix**: Rename local variable:

```go
agentConfig := &react.AgentConfig{
    MaxStep:            25,
    ...
}
agentConfig.ToolCallingModel = chatModel
```

---

### 14. ai_ops Handler Not Using Event Stream Pattern

**File**: `internal/app/ai_ops_handler.go`
**Severity**: Medium
**Description**: The Next.js project uses an async generator pattern for `ai_ops`, yielding events (`plan_created`, `step_start`, `step_done`, `replan`, `done`, `error`), while the Go project's HTTP handler does not use event streaming.

**Next.js version** (`lib/ai/pipelines/plan-execute-replan/index.ts:48-105`):

```typescript
export async function* runPlanExecuteReplan(): AsyncGenerator<PlanExecuteEvent> {
  ...
  yield { type: "plan_created", steps: plan };
  yield { type: "step_start", index: i, step };
  yield { type: "step_done", index: i, output: res.text };
  yield { type: "replan", done: obj.done, remaining: obj.remaining };
  yield { type: "done", result: obj.summary, detail };
}
```

**Go version** (`ai_ops_handler.go:31-49`):

```go
func (a *App) handleAIOps(ctx *swifty_http.Context, next func()) {
    resp, detail, err := plan_execute_replan.BuildPlanAgent(...)
    // Returns only final result, no streaming
}
```

**Fix**: Implement SSE streaming for `ai_ops`:

```go
func (a *App) handleAIOps(ctx *swifty_http.Context, next func()) {
    sse := ctx.SSE()
    sse.Event("connected", `{"status":"connected"}`)

    for event := range plan_execute_replan.BuildPlanAgentStream(...) {
        switch event.Type {
        case "plan_created":
            sse.Event("plan_created", marshal(event.Steps))
        case "step_start":
            sse.Event("step_start", marshal(event))
        ...
        }
    }
    sse.Done()
}
```

---

## Low Severity Issues (P3)

### 15. No Test Files

**Severity**: Low
**Description**: The Go project has no `*_test.go` files. Testing relies entirely on 5 CLI commands in `cmd/`.

**Fix**: Add unit tests for critical paths:

- `internal/app/*_test.go`
- `internal/ai/agent/*_test.go`
- `internal/ai/tools/*_test.go`

---

### 16. Unstructured Logging

**Severity**: Low
**Description**: Logs are scattered across `fmt.Printf`, `log.Printf`, and `log_callback`, with no unified log level, format, or output target.

**Fix**: Implement structured logging using `slog` or a library like `zap`:

```go
import "log/slog"

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

logger.Info("indexing file", "path", path, "chunks", len(ids))
logger.Error("tool failed", "tool", name, "error", err)
```

---

### 17. No Conversation Memory Persistence

**File**: `internal/utility/mem/mem.go`
**Severity**: Low
**Description**: `ConversationMemory` is stored in memory only. All conversation history is lost after service restart.

**Fix**: Add Redis persistence:

```go
func (m *ConversationMemory) Append(msg *schema.Message) {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.Messages = append(m.Messages, msg)
    // Persist to Redis
    client := redis.GetClient()
    client.RPush(ctx, "mem:"+m.ID, marshal(msg))
    client.LTrim(ctx, "mem:"+m.ID, -int64(m.MaxWindowSize), -1)
}
```

---

### 18. DuckDuckGo Search Tool Not Integrated

**File**: `internal/ai/agent/chat_pipeline/search_tool.go`
**Severity**: Low
**Description**: The file defines `newSearchTool()` but it is never called in any pipeline. This is dead code.

**Fix**: Either integrate the tool or remove the file:

```go
// In tools_node.go
config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, newSearchTool())
```

---

### 19. Missing Infrastructure Files

**Severity**: Low
**Description**: The Go project is missing:

- `README.md`
- `Makefile`
- `Dockerfile`
- `docker-compose.yml` (Milvus cluster from Next.js project)

**Next.js has**:

- `docker-compose.yml` with etcd + minio + milvus + attu
- `prometheus.yml`

**Fix**: Create infrastructure files:

```makefile
# Makefile
.PHONY: build run test

build:
    go build -o bin/swifty-agent main.go

run:
    go run main.go

test:
    go test ./...
```

```dockerfile
# Dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o bin/swifty-agent main.go

FROM alpine:latest
COPY --from=builder /app/bin/swifty-agent /usr/local/bin/
EXPOSE 6872
CMD ["swifty-agent"]
```

---

## Alignment Issues Summary

| Component             | Next.js                      | Go                           | Status      |
| --------------------- | ---------------------------- | ---------------------------- | ----------- |
| JSON field names      | lowercase (`id`, `question`) | uppercase (`Id`, `Question`) | ❌ Mismatch |
| File directory        | `./data/docs`                | `./docs`                     | ❌ Mismatch |
| Model name            | `openai-v3-2-251201`         | `deepseek-v3-2-251201`       | ❌ Mismatch |
| Redis vector field    | `vector`                     | `vector_content`             | ❌ Mismatch |
| Log topic config      | env vars                     | hardcoded                    | ❌ Mismatch |
| Output format         | markdown only                | plain text only              | ❌ Mismatch |
| Memory LRU            | 100 sessions max             | unlimited                    | ❌ Mismatch |
| Redis reconnect       | exponential backoff          | none                         | ❌ Mismatch |
| Index dimension check | yes                          | no                           | ❌ Mismatch |
| ai_ops streaming      | async generator              | sync return                  | ❌ Mismatch |
| Prometheus tool       | functional                   | disabled                     | ❌ Mismatch |
| mysql_crud stdin      | removed                      | present                      | ❌ Mismatch |

---

## Recommendations

1. **Immediate**: Fix all Critical (P0) issues before deployment
2. **Short-term**: Address High (P1) issues for feature parity
3. **Medium-term**: Implement Medium (P2) improvements for stability
4. **Long-term**: Add tests, structured logging, and infrastructure files

---

## Conclusion

The migration has **significant gaps** that prevent production deployment. The most critical issues are:

- Request parsing will fail due to JSON field name mismatch
- HTTP service will crash due to `log.Fatal` usage
- Tool calls will hang due to stdin blocking
- Prometheus functionality is completely disabled

**Recommendation**: Address all P0 and P1 issues before considering the migration complete.
