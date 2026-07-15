# Next.js → Go Backend Migration Review Report

## Executive Summary

This report presents a detailed one-to-one comparison of all four API routes migrated from a Next.js backend to a Go backend. **12 issues** were identified: **1 Critical**, **3 High**, **5 Medium**, and **3 Low** severity.

The most severe issue is a **JSON field name casing mismatch** in the `chatRequest` struct that will cause all chat and chat_stream requests from the existing frontend to silently fail (fields parsed as empty strings).

---

## 1. Route Mapping Overview

| Next.js Route | Go Route | Method | Status |
|---|---|---|---|
| `/api/chat` | `/api/chat` | POST | ✅ Mapped |
| `/api/chat_stream` | `/api/chat_stream` | POST | ✅ Mapped |
| `/api/ai_ops` | `/api/ai_ops` | POST | ✅ Mapped |
| `/api/upload` | `/api/upload` | POST | ✅ Mapped |
| OPTIONS (all routes) | CORS middleware | OPTIONS | ⚠️ Handled differently |

**Verdict**: All four routes are mapped. No missing routes.

---

## 2. Detailed Findings

---

### 🔴 CRITICAL-01: JSON Field Name Casing Mismatch — `chatRequest` struct

**Severity**: Critical  
**Category**: Request Body Parsing  
**Affects**: `/api/chat`, `/api/chat_stream`

**Next.js** expects lowercase JSON keys:
```typescript
// route.ts (chat & chat_stream)
const chatRequestSchema = z.object({
  id: z.string().min(1),       // lowercase "id"
  question: z.string().min(1), // lowercase "question"
});
```

**Go** struct uses PascalCase JSON tags:
```go
// chat_handler.go, line ~22
type chatRequest struct {
    ID       string `json:"Id"`       // ❌ PascalCase "Id"
    Question string `json:"Question"` // ❌ PascalCase "Question"
}
```

**Impact**: When the frontend sends `{"id":"abc","question":"hello"}`, Go's `json.Decoder` will NOT match `"Id"` / `"Question"` and both fields will be empty strings. **All chat and streaming chat requests will fail silently** — the pipeline will run with empty ID and empty query.

**Fix**:
```go
type chatRequest struct {
    ID       string `json:"id"`
    Question string `json:"question"`
}
```

---

### 🟠 HIGH-01: Error Response Format Missing `data: null` Field

**Severity**: High  
**Category**: Response Format  
**Affects**: All routes, all error paths

**Next.js** always returns a two-field envelope `{ message, data }`:
```typescript
// Every error path across all routes:
return Response.json(
  { message: "missing id or question", data: null },  // ✅ data: null
  { status: 400, headers: CORS_HEADERS },
);
```

**Go** `ctx.Throw()` only returns `{ message }`:
```go
// swifty_http/context.go, Throw()
func (ctx *Context) Throw(status int, msg string) {
    ctx.Status = status
    ctx.statusSet = true
    ctx.Body = H{"message": msg}  // ❌ No "data" key
}
```

**Impact**: Every error response from the Go backend is missing the `data` field. Frontend code that destructures `{ message, data }` will receive `data` as `undefined` instead of `null`. Depending on the client implementation, this may cause runtime errors or incorrect error display.

**Affected error responses** (non-exhaustive):
| Route | Next.js Error | Go Error |
|---|---|---|
| `/api/chat` 400 | `{message:"missing id or question", data:null}` | `{message:"invalid request body"}` |
| `/api/chat` 500 | `{message:<detailed JSON>, data:null}` | `{message:<err.Error()>}` |
| `/api/chat_stream` 400 | `{message:"missing id or question", data:null}` | `{message:"invalid request body"}` |
| `/api/upload` 400 | `{message:"no file uploaded", data:null}` | `{message:"please upload a file"}` |
| `/api/ai_ops` 500 | `{message:<error>, data:null}` | `{message:<error>}` |

**Fix**: Either modify `Throw()` to include `data: null`, or use a separate error response helper:
```go
func (ctx *Context) Throw(status int, msg string) {
    ctx.Status = status
    ctx.statusSet = true
    ctx.Body = H{"message": msg, "data": nil}
}
```

---

### 🟠 HIGH-02: SSE `id:` Field Framing Mismatch

**Severity**: High  
**Category**: Streaming (SSE)  
**Affects**: `/api/chat_stream`

**Next.js** sends `id: <timestamp>` as part of **every** event frame:
```typescript
// chat_stream/route.ts, line ~38
const send = (event: string, data: string) => {
  controller.enqueue(
    encoder.encode(`id: ${Date.now()}\nevent: ${event}\ndata: ${data}\n\n`)
  );
};
```

Output per event:
```
id: 1719876543210
event: message
data: Hello

```

**Go** sends a standalone `id: <clientID>` once at connection start, and individual events have no `id:` field:
```go
// chat_handler.go, line ~73-74
sse := ctx.SSE()
sse.ID(req.ID)  // Sends "id: <clientID>\n" then flushes
// ...
sse.Event("message", chunk.Content)  // Sends "event: message\ndata: ...\n\n" — no id:
```

Output:
```
id: user-123

event: connected
data: {"status":"connected","client_id":"user-123"}

event: message
data: Hello

```

**Differences**:
1. Next.js uses `Date.now()` (timestamp) as the SSE event ID; Go uses the client's session ID (a user-provided string).
2. Next.js includes `id:` in every event frame; Go sends it once as a standalone line.
3. The Go standalone `id:` line has no trailing blank line separator before the first event — depending on the SSE parser, this could cause the `id:` to be associated with the `connected` event or ignored.

**Impact**: SSE clients using `Last-Event-ID` for reconnection will behave differently. The event IDs are semantically incompatible (timestamp vs. session ID).

---

### 🟠 HIGH-03: SSE `Done()` Sends Extra `[DONE]` Data Frame

**Severity**: High  
**Category**: Streaming (SSE)  
**Affects**: `/api/chat_stream`

**Next.js** closes the stream after the `done` event:
```typescript
// chat_stream/route.ts
send("done", "Stream completed");
// controller.close() in finally block
```

Output:
```
id: 1719876543211
event: done
data: Stream completed

```
*(stream closes)*

**Go** sends `done` event AND an extra `[DONE]` data frame:
```go
// chat_handler.go, line ~97-98
sse.Event("done", "Stream completed")
sse.Done()  // Calls w.Data("[DONE]") → "data: [DONE]\n\n"
```

Output:
```
event: done
data: Stream completed

data: [DONE]

```

**Impact**: The client will receive an extra `data: [DONE]` message event after the stream is supposed to be complete. If the client's SSE listener processes all `message` events (unnamed event), it will receive `[DONE]` as an additional data chunk, potentially causing display corruption or parsing errors.

---

### 🟡 MEDIUM-01: Missing Input Validation in Go (No Min-Length Check)

**Severity**: Medium  
**Category**: Request Validation  
**Affects**: `/api/chat`, `/api/chat_stream`

**Next.js** uses Zod with `.min(1)` validation:
```typescript
const chatRequestSchema = z.object({
  id: z.string().min(1),       // Rejects empty string
  question: z.string().min(1), // Rejects empty string
});
```

**Go** only performs JSON decoding — no field validation:
```go
var req chatRequest
if err := ctx.BindJSON(&req); err != nil {
    ctx.Throw(http.StatusBadRequest, "invalid request body")
    return
}
// req.ID and req.Question could be empty strings — no check
```

**Impact**: The Go backend accepts `{"id":"","question":""}` without error, passing empty strings to the AI pipeline. This could cause unexpected behavior, wasted API calls, or panics downstream.

**Fix**: Add explicit validation after binding:
```go
if req.ID == "" || req.Question == "" {
    ctx.JSON(map[string]interface{}{
        "message": "missing id or question",
        "data":    nil,
    })
    ctx.Status = http.StatusBadRequest
    ctx.statusSet = true
    return
}
```

---

### 🟡 MEDIUM-02: Chat Error Response Missing Detailed Error Information

**Severity**: Medium  
**Category**: Error Handling  
**Affects**: `/api/chat`

**Next.js** catches errors and returns detailed diagnostic information:
```typescript
// chat/route.ts, catch block
const err = e as { name?, message?, statusCode?, url?, responseBody?, responseHeaders? };
return Response.json({
  message: JSON.stringify({
    name: err?.name,
    message: err?.message ?? String(e),
    statusCode: err?.statusCode,
    url: err?.url,
    responseBody: err?.responseBody,
    responseHeaders: err?.responseHeaders,
  }),
  data: null,
}, { status: 500 });
```

**Go** only returns the raw error string:
```go
// chat_handler.go
if err != nil {
    ctx.Throw(http.StatusInternalServerError, err.Error())  // Just err.Error()
    return
}
```

**Impact**: The frontend loses critical debugging information (upstream API URL, status code, response body, headers) when the LLM provider returns an error. This significantly reduces production debuggability.

---

### 🟡 MEDIUM-03: Error Message Text Mismatches

**Severity**: Medium  
**Category**: Error Handling  
**Affects**: `/api/chat`, `/api/chat_stream`, `/api/upload`

| Route | Scenario | Next.js Message | Go Message |
|---|---|---|---|
| `/api/chat` | Invalid body | `"missing id or question"` | `"invalid request body"` |
| `/api/chat_stream` | Invalid body | `"missing id or question"` | `"invalid request body"` |
| `/api/upload` | No file | `"no file uploaded"` | `"please upload a file"` |
| `/api/ai_ops` | Empty result | `"internal error"` | `"internal error: empty response"` |

**Impact**: If the frontend has any string-matching logic on error messages (e.g., for i18n or user-facing toasts), these mismatches will cause incorrect behavior. Even without string matching, the user experience will differ.

---

### 🟡 MEDIUM-04: CORS Configuration Differences

**Severity**: Medium  
**Category**: CORS & Middleware  
**Affects**: All routes

| Aspect | Next.js | Go |
|---|---|---|
| `Allow-Methods` | `POST, OPTIONS` | `GET,POST,PUT,DELETE,PATCH,OPTIONS` |
| `Allow-Headers` | `Content-Type` | `Authorization,Content-Type` |
| Scope | Per-route, explicitly set | Global middleware |

**Next.js** (per-route):
```typescript
const CORS_HEADERS = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Methods": "POST, OPTIONS",
  "Access-Control-Allow-Headers": "Content-Type",
};
```

**Go** (global middleware in `app.go`):
```go
func corsMiddleware(ctx *swifty_http.Context, next func()) {
    ctx.Set("Access-Control-Allow-Origin", "*")
    ctx.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH,OPTIONS")
    ctx.Set("Access-Control-Allow-Headers", "Authorization,Content-Type")
    // ...
}
```

**Impact**:
- Go allows more methods (GET, PUT, DELETE, PATCH) which may mask routing errors — a client sending a DELETE to `/api/chat` would get a CORS preflight pass but then a 404, whereas Next.js would reject at preflight.
- Go allows `Authorization` header which Next.js does not. This is likely intentional (for future use) but is a deviation.
- **Functional difference**: The Go CORS middleware handles OPTIONS globally by returning early with 204. This is functionally equivalent to the per-route OPTIONS handlers in Next.js.

---

### 🟡 MEDIUM-05: Default File Upload Directory Differs

**Severity**: Medium  
**Category**: File Upload  
**Affects**: `/api/upload`

| Backend | Default Directory |
|---|---|
| **Next.js** (`lib/config.ts`) | `./data/docs` |
| **Go** (`config/config.go`) | `./docs` |

```typescript
// Next.js config.ts
fileDir: process.env.FILE_DIR ?? "./data/docs",
```
```go
// Go config.go
if cfg.FileDir == "" {
    cfg.FileDir = "./docs"
}
```

**Impact**: If deployed without explicit `FILE_DIR` configuration, files will be stored in different locations, potentially causing knowledge base inconsistencies or missing files during migration.

---

### 🔵 LOW-01: Go Memory Has No LRU Eviction (Unbounded Growth)

**Severity**: Low  
**Category**: Memory Management  
**Affects**: `/api/chat`, `/api/chat_stream`

**Next.js** has LRU eviction with a cap of 100 sessions:
```typescript
// memory.ts
const MAX_SESSIONS = 100;
// Evict oldest session if at capacity.
if (memoryMap.size >= MAX_SESSIONS) {
    const oldestKey = memoryMap.keys().next().value;
    if (oldestKey !== undefined) memoryMap.delete(oldestKey);
}
```

**Go** has no session cap — the map grows unboundedly:
```go
// mem.go
var memMap = make(map[string]*ConversationMemory)
func Get(id string) *ConversationMemory {
    // ...
    m := &ConversationMemory{...}
    memMap[id] = m  // Never evicted
    return m
}
```

**Impact**: In long-running production deployments, the Go backend will accumulate conversation memory for every unique session ID without ever releasing it, leading to a slow memory leak. This is not a correctness issue but a reliability concern for long-running services.

---

### 🔵 LOW-02: AI Ops Prompt Text Differs Between Implementations

**Severity**: Low  
**Category**: Functional Consistency  
**Affects**: `/api/ai_ops`

The hardcoded AI Ops query prompt differs between backends:

**Next.js** (`plan-execute-replan/index.ts`):
```
"1. You are an intelligent service alert analysis assistant. First, call the tool 
query_prometheus_alerts to retrieve all active alerts.
2. For each alert, call the tool query_internal_docs by alert name..."
```

**Go** (`ai_ops_handler.go`):
```
"1. You are an intelligent alert analysis assistant. First call the tool 
query_prometheus_alerts to get all active alerts."
"2. For each alert, call the tool query_internal_docs to find the corresponding 
processing guide."...
```

**Specific differences**:
- Go version has each line wrapped in separate quoted strings (with extra `"` characters that may be included in the actual prompt sent to the LLM).
- Wording differences: "service alert analysis" vs "alert analysis", "retrieve" vs "get", "handling procedure" vs "processing guide".
- Go version has a trailing newline and different formatting.

**Impact**: The LLM may produce slightly different output quality or formatting. The extra quote characters in the Go string constant could be injected into the prompt, potentially confusing the model.

---

### 🔵 LOW-03: SSE `connected` Event Data Not JSON-Encoded in Go

**Severity**: Low  
**Category**: Streaming (SSE)  
**Affects**: `/api/chat_stream`

**Next.js** properly JSON-stringifies the connected payload:
```typescript
send("connected", JSON.stringify({ status: "connected", client_id: id }));
```
Output: `data: {"status":"connected","client_id":"abc"}\n\n`

**Go** uses string concatenation without proper JSON encoding:
```go
sse.Event("connected", `{"status":"connected","client_id":"`+req.ID+`"}`)
```

**Impact**: If `req.ID` contains special characters (quotes, backslashes, newlines), the resulting JSON will be malformed. For example, if `req.ID` is `user"evil`, the output becomes `{"status":"connected","client_id":"user"evil"}` which is invalid JSON.

**Fix**:
```go
connectedPayload, _ := json.Marshal(map[string]string{
    "status":    "connected",
    "client_id": req.ID,
})
sse.Event("connected", string(connectedPayload))
```

---

## 3. SSE Wire Protocol Comparison

### Next.js SSE Output (chat_stream):
```
id: 1719876543210
event: connected
data: {"status":"connected","client_id":"user-123"}

id: 1719876543211
event: message
data: Hello

id: 1719876543212
event: message
data:  world

id: 1719876543213
event: done
data: Stream completed

```
*(stream closes)*

### Go SSE Output (chat_stream):
```
id: user-123
event: connected
data: {"status":"connected","client_id":"user-123"}

event: message
data: Hello

event: message
data:  world

event: done
data: Stream completed

data: [DONE]

```
*(stream closes)*

**Key differences**: `id:` semantics, per-event vs. one-shot ID, extra `[DONE]` frame.

---

## 4. Response Headers Comparison

| Header | Next.js | Go |
|---|---|---|
| `Content-Type` (JSON) | `application/json` (implicit) | `application/json` (set by `respondJSON`) |
| `Content-Type` (SSE) | `text/event-stream` | `text/event-stream` |
| `Cache-Control` (SSE) | `no-cache` | `no-cache` |
| `Connection` (SSE) | `keep-alive` | `keep-alive` |
| `Access-Control-Allow-Origin` | `*` | `*` |

✅ Response headers are functionally equivalent.

---

## 5. Summary Table

| ID | Severity | Category | Issue |
|---|---|---|---|
| CRITICAL-01 | 🔴 Critical | Request Parsing | JSON field names `"Id"`/`"Question"` should be `"id"`/`"question"` |
| HIGH-01 | 🟠 High | Response Format | Error responses missing `data: null` field |
| HIGH-02 | 🟠 High | SSE | `id:` field uses client ID once instead of timestamp per event |
| HIGH-03 | 🟠 High | SSE | Extra `data: [DONE]` frame not in Next.js |
| MEDIUM-01 | 🟡 Medium | Validation | No min-length validation on `id`/`question` |
| MEDIUM-02 | 🟡 Medium | Error Handling | Chat 500 errors missing detailed diagnostic fields |
| MEDIUM-03 | 🟡 Medium | Error Handling | Error message text mismatches |
| MEDIUM-04 | 🟡 Medium | CORS | Allowed methods/headers broader than Next.js |
| MEDIUM-05 | 🟡 Medium | Config | Default file upload directory differs |
| LOW-01 | 🔵 Low | Memory | No LRU eviction for session memory |
| LOW-02 | 🔵 Low | Functional | AI Ops prompt text differences |
| LOW-03 | 🔵 Low | SSE | Connected event JSON not properly encoded |

---

## 6. Recommendations (Priority Order)

1. **Fix CRITICAL-01 immediately** — Change JSON tags to `json:"id"` and `json:"question"`. Without this, the chat endpoints are completely non-functional.
2. **Fix HIGH-01** — Add `"data": nil` to all error responses to maintain API contract compatibility.
3. **Fix HIGH-03** — Remove the `sse.Done()` call or modify the SSE library's `Done()` to not emit `[DONE]` if the Next.js frontend doesn't expect it.
4. **Address HIGH-02** — Align SSE `id:` semantics with the frontend's expectations (especially if reconnection via `Last-Event-ID` is used).
5. **Address MEDIUM-01** — Add empty-string validation for request fields.
6. **Address MEDIUM-02** — Enrich error responses with diagnostic details for production debugging.
7. **Address MEDIUM-05** — Align default file directory or ensure config is always explicitly set.
8. **Fix LOW-03** — Use `json.Marshal` for SSE connected event payload to prevent injection.
9. **Plan LOW-01** — Add session cap / LRU eviction to memory module before production deployment.
