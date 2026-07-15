# Swifty Agent 后端迁移 Review 报告(Next.js → Go)

**审查范围**: `/swifty-cli/apps/swifty-agent`(Next.js 后端 `lib/` + `app/api/`)→ `/swifty.go/swifty_agent`(Go `internal/` + `cmd/`),排除两端前端。
**参考报告**: claude / Sisyphus / kimi / swifty(按优先级)。
**结论**: 迁移结构完整,但存在 **6 个 P0 致命 bug** + 多处数据层不兼容 + 功能未对齐。已有 4 份报告**集体遗漏了若干关键 bug**(下文标注 🆕)。

---

## 一、P0 致命 Bug(服务崩溃 / 功能完全失效)

### P0-1. `chatRequest` JSON 字段大小写不匹配 — chat 全线不可用 ✅已证实

- **Go** `internal/app/chat_handler.go:20-22`: `json:"Id"` / `json:"Question"`
- **Next.js** `app/api/chat/route.ts:6-8`: 前端发送小写 `id`/`question`
- `ctx.BindJSON` 严格匹配 tag → 两字段反序列化为空 → `/api/chat`、`/api/chat_stream` 全部失效。
- 4 份报告均命中。修复:改 `json:"id"` / `json:"question"`。

### P0-2. 助手回复被存为 `SystemMessage` 而非 `AssistantMessage` 🆕(仅 Sisyphus 命中,其余 3 份遗漏)

- **Go** `internal/app/chat_handler.go:53` 与 `internal/app/chat_handler.go:103`:
  ```go
  mem.Get(req.ID).Append(schema.SystemMessage(out.Content))  // 应为 AssistantMessage
  ```
- 同样 bug 还出现在 CLI `cmd/chat/main.go:65`。
- **Next.js** `lib/ai/pipelines/chat.ts:78-79`: `{ role: "assistant", content: answer }`。
- 影响:多轮对话时助手回复被 LLM 当作系统指令,上下文逐步污染、回答劣化。`prompt.go:24` 的 `SystemMessage(systemPrompt)` 是正确的,只有回写记忆处错了。

### P0-3. System Prompt 输出格式要求完全矛盾 ✅已证实

- **Go** `internal/ai/agent/chat_pipeline/prompt.go:53`: `Output must not contain markdown syntax; use plain text only`
- **Next.js** `lib/ai/pipelines/chat.ts:39`: `Output markdown only`
- 完全相反。前端 markdown 渲染器接到 Go 后端会显示纯文本。

### P0-4. 工具函数中 `log.Fatal` 终止整个 HTTP 进程 ✅已证实

9 处(已 grep 确认):

- `mysql_crud.go:34,47,53,62`
- `get_current_time.go:54`
- `query_internal_docs.go:29,33,40`
- `query_metrics_alerts.go:158`

`log.Fatal` = `os.Exit(1)`,工具在 agent 运行时回调 → 单次坏的工具调用 = 全站宕机。Next.js `lib/ai/tools/operations.ts` 用 try-catch 返回错误。修复:全部改 `return "", fmt.Errorf(...)`。

### P0-5. `mysql_crud` stdin 阻塞 + SQL 双重执行 🆕(双重执行仅 Kimi 命中)

- **Go** `internal/ai/tools/mysql_crud.go:38-44`: `bufio.NewScanner(os.Stdin)` 在 HTTP 模式永久阻塞。
- **双重执行 bug** `internal/ai/tools/mysql_crud.go:46-54`: 先 `db.Exec(input.SQL)`(对所有类型),再对 `query` 类型 `db.Raw(input.SQL).Scan(...)` → SELECT 被执行两次;且 INSERT/UPDATE/DELETE 返回空字符串 `""` 而非成功消息。
- **Next.js** `lib/ai/tools/operations.ts:135-153`: 无交互确认;按 `operateType` 分支只执行一次,非查询返回 `{success:true, message:...}`。

### P0-6. `query_prometheus_alerts` 死代码 — 永远返回空 ✅已证实

- **Go** `internal/ai/tools/query_metrics_alerts.go:54-56`: 第 56 行 `return PrometheusAlertsResult{}, nil`,其后全部不可达。
- ai_ops 流程第 1 步要求 "call query_prometheus_alerts to get all active alerts" → 永远 0 告警 → 整个 AI Ops 报告无意义。
- **Next.js** `lib/ai/tools/operations.ts:56-104` 实际请求 `${prometheusBaseUrl}/api/v1/alerts`。

### P0-7. MCP 工具无缓存 + 无降级 → MCP 成为所有 chat 的硬依赖 🆕(Sisyphus 提及但严重度被低估)

- **Go** `internal/ai/tools/mcp_tool.go:22-44`: `GetLogMcpTool` 每次新建 SSE 客户端,且**返回 error 直接上抛**。
- 该函数被 `internal/ai/agent/chat_pipeline/tools_node.go:28`(chat pipeline)**和** `internal/ai/agent/plan_execute_replan/executor.go:18`(plan executor)在**构建阶段**调用 → MCP server 宕机 = 每个 `/api/chat`、`/api/chat_stream`、`/api/ai_ops` 请求在 BuildChatAgent/BuildPlanAgent 阶段就失败。
- **Next.js** `lib/ai/tools/query-log.ts:9-10,47-55`: `cachedClient`/`cachedTools` 缓存;catch 后 `console.warn` 并返回 `{}`(空工具集),chat 照常工作。
- 这应升级为 P0:MCP 是日志工具,不应拖垮主聊天链路。

---

## 二、数据层不兼容(两项目共用 Redis 时无法互通)

### D-1. Redis Key Prefix 不一致 ✅已证实

| 项目                             | prefix | 示例 key     |
| -------------------------------- | ------ | ------------ |
| Next.js `lib/config.ts:48`       | `biz:` | `biz:{uuid}` |
| Go `internal/consts/consts.go:6` | `doc:` | `doc:{uuid}` |

### D-2. Vector Field Name 不一致 ✅已证实

| 项目                              | vector field     |
| --------------------------------- | ---------------- |
| Next.js `lib/redis/indexer.ts:30` | `vector`         |
| Go `internal/consts/consts.go:16` | `vector_content` |

Go retriever `internal/ai/retriever/retriever.go:32` 搜 `vector_content`,Next.js 写 `vector` → 互相检索不到。

### D-3. `_source` 字段值不一致(basename vs 完整 URI)🆕已通过 eino-ext 源码证实

- **Next.js** `lib/ai/loader.ts:13`: `source = path.basename(filePath)` → `alert-guide.md`
- **Go**: 使用 Eino `file.NewFileLoader`,其源码 `file_loader.go:37,96` 中 `MetaKeySource = "_source"` 且 `meta[MetaKeySource] = src.URI` → 存完整路径 `./docs/alert-guide.md`。
- `internal/ai/agent/knowledge_index_pipeline/index_file.go:47` 据此做 `@_source:{<escaped full path>}` 去重。
- 影响:Go 内部自洽(同路径同 `_source`),但**无法去重 Next.js 已索引的记录**,反之亦然——跨项目迁移会产生重复数据。Sisyphus 曾怀疑,现确认。

### D-4. Index Schema 字段不一致 ✅已证实

- **Next.js** `lib/redis/client.ts:72-86`: `vector`(VECTOR)+ `content`(TEXT)+ `_source`(TAG),且 hash 里额外存 `metadata`(JSON string)、`created_at`。
- **Go** `internal/utility/redis/redis.go:46-56`: `content`+`_source`+`vector_content`,**无 `metadata` schema 字段**。Go retriever `internal/ai/retriever/retriever.go:33` `ReturnFields` 只取 `content`,丢失 metadata/score。

### D-5. 无 Index 维度漂移检测 🆕(仅 Swifty 命中)

- **Next.js** `lib/redis/client.ts:39-69`: `FT.INFO` 读取 vector 属性 DIM,与 `EMBEDDING_DIM` 不一致则 `dropIndex` 重建。
- **Go** `internal/utility/redis/redis.go:37-39`: 只判断 `FT.INFO` 是否报错,不检查维度。切换 embedding model(如 2048d→768d)后静默搜索失败。

---

## 三、功能未对齐(P1)

### F-1. Plan-Execute-Replan Executor 缺 `mysql_crud` 工具 ✅已证实

- **Go** `internal/ai/agent/plan_execute_replan/executor.go:23-26`: 注册 MCP + PrometheusAlerts + QueryInternalDocs + GetCurrentTime,**缺 MysqlCrud**。
- **Next.js** `lib/ai/pipelines/plan-execute-replan/index.ts:43-46`: `{...mcp, ...builtinTools}`(含 mysql_crud)。
- chat pipeline `internal/ai/agent/chat_pipeline/tools_node.go:35-38` 反而**包含** MysqlCrud——两条链路工具集不一致。

### F-2. Executor `MaxIterations = 999999` ✅已证实

- **Go** `internal/ai/agent/plan_execute_replan/executor.go:40`: `999999`(单步工具调用上限)。
- **Next.js** `lib/ai/pipelines/plan-execute-replan/executor.ts:18`: `isStepCount(10)`。
- 注:外层 `MaxIterations:20`(`internal/ai/agent/plan_execute_replan/plan_execute_replan.go:38`)与 Next.js `MAX_ITERATIONS=20` 对齐。失控风险在单步。

### F-3. Memory 无 LRU 淘汰 — 内存泄漏 ✅已证实

- **Go** `internal/utility/mem/mem.go:12-33`: 纯 map,无上限。
- **Next.js** `lib/memory.ts:10-29`: `MAX_SESSIONS=100`,Map 插入序实现 LRU。
- 附带:`internal/utility/mem/mem.go:63-66` `All()` 直接返回内部 slice,调用方(eino pipeline)可能修改内容,应返回拷贝(Sisyphus P3-7)。

### F-4. `deleteBySource` 无分布式锁 + 未分批 ✅已证实

- **Go** `internal/ai/agent/knowledge_index_pipeline/index_file.go:49-65`: 单次 `FT.SEARCH ... LIMIT 0 10000` + `DEL`,无锁。
- **Next.js** `lib/redis/indexer.ts:46-77`: SETNX 锁(30s TTL)+ 每批 1000 循环删。
- 并发上传同文件有竞态;>10000 chunk 的大文件删不全。

### F-5. System Prompt 硬编码 Log Topic ✅已证实

- **Go** `internal/ai/agent/chat_pipeline/prompt.go:42`: 硬编码 `ap-guangzhou` / `869830db-...`。
- **Next.js** `lib/ai/pipelines/chat.ts:12-17`: 读 `LOG_TOPIC_REGION`/`LOG_TOPIC_ID`,未设置则省略该行。
- Go `internal/config/config.go` 也无对应配置字段。

### F-6. AI Ops Prompt 文本差异 + 字面引号包裹 🆕(引号问题仅 Kimi 简提)

- **Go** `internal/app/ai_ops_handler.go:12-26` 用反引号 raw string,但每行写成 `"1. You are..."` → **字面双引号字符**被送进 LLM。
- 章节标题也与 Next.js 不同:
  - Go: `# Alert Processing Details` / `## Root Cause Analysis N` / `## Processing Plan N`
  - Next.js `lib/ai/pipelines/plan-execute-replan/index.ts:18-31`: `# Alert Handling Details` / `## Alert Root Cause Analysis N` / `## Handling Procedure Execution N`
- 🆕 该 `aiOpsQuery` 常量在 `internal/app/ai_ops_handler.go` 和 `cmd/ai_ops/main.go:18-32` **重复定义**(两份逐字相同),应抽到公共包。

### F-7. Date 格式差异 ✅已证实

- **Go** `internal/ai/agent/chat_pipeline/lambda_func.go:21`: `2006-01-02 15:04:05` → `2026-07-15 15:12:38`
- **Next.js** `lib/ai/pipelines/chat.ts:49`: `toLocaleString("en-US")` → `7/15/2026, 3:12:38 PM`

### F-8. `get_current_time` 精度差异 🆕

- **Go** `internal/ai/tools/get_current_time.go:34`: `2006-01-02 15:04:05.000000`(微秒,6 位)
- **Next.js** `lib/ai/tools/operations.ts:22-27`: 毫秒,3 位
- 工具描述 Go 写 "microseconds",Next.js 实际给毫秒。轻微但属未对齐。

### F-9. `query_prometheus_alerts` 错误时同时返回 JSON 和 error 🆕(4 份报告均遗漏)

- **Go** `internal/ai/tools/query_metrics_alerts.go:122`: `return string(b), err` —— 同时返回序列化结果和非 nil error。Eino 工具框架可能据此判定工具失败而非返回结果给 LLM。
- **Next.js** `lib/ai/tools/operations.ts:95-104`: catch 后返回 `{success:false, error:...}` 字符串,不抛错。
- 另外 Go 工具描述 `internal/ai/tools/query_metrics_alerts.go:110` 说返回 "labels, annotations, state, and values",实际返回 SimplifiedAlert(去重后),描述与实现不符。

---

## 四、配置层未对齐(P1-P2)

| 项                       | Next.js                            | Go                                                             | 状态                 |
| ------------------------ | ---------------------------------- | -------------------------------------------------------------- | -------------------- |
| `prometheusBaseUrl`      | `lib/config.ts:55` 有              | ❌ 无,硬编码且不可达                                           | 需加字段             |
| `fileDir` 默认           | `./data/docs`                      | `./docs`(`internal/config/config.go:105`)                      | 不一致               |
| `model` 默认             | `openai-v3-2-251201`               | `deepseek-v3-2-251201`(`config.json:7,12`)                     | 不一致               |
| `max_tokens`             | Anthropic 默认 8192                | config.json 未设→默认 4096(`internal/config/config.go:98-102`) | 不一致               |
| `embeddingProvider`      | dashscope/ollama 可选              | ❌ 仅 dashscope(`internal/ai/embedder/embedder.go:17`)         | 缺 Ollama            |
| `anthropic.thinking`     | 可配(`lib/config.ts:30`)           | ❌ 无                                                          | 缺 extended thinking |
| Anthropic signature 修补 | `lib/ai/models.ts:18-61` 有        | ❌ 依赖 Eino claude 内部                                       | 非官方网关可能报错   |
| `redis.indexName`        | `idx:biz`                          | `idx:biz`(`internal/consts/consts.go:9`)                       | ✅ 对齐              |
| Redis 重连               | 指数退避(`lib/redis/client.ts:29`) | ❌ 无                                                          | 韧性不足             |
| `.env.example`           | 有                                 | ❌ 无                                                          | 缺文档               |

---

## 五、API 行为未对齐(P2)

### A-1. 请求参数校验缺失 ✅已证实

- **Go** `internal/app/chat_handler.go:28`: `BindJSON` 只校验 JSON 语法,`{Id:"", Question:""}` 可通过并发给 LLM。
- **Next.js** `app/api/chat/route.ts:6-9`: Zod `.min(1)`,空串返回 400。

### A-2. `/api/chat` 错误响应丢失结构化诊断 ✅已证实

- **Go** `internal/app/chat_handler.go:48`: `ctx.Throw(500, err.Error())` 只给裸字符串。
- **Next.js** `app/api/chat/route.ts:47-59`: 序列化 `name/message/statusCode/url/responseBody/responseHeaders`。

### A-3. 错误响应缺 `data: null` 字段 🆕(仅 Kimi 命中,已通过源码证实)

- **Go** `swifty_http/context.go:51-55`: `Throw` 设 `Body = H{"message": msg}`,**无 `data`**。
- 成功响应(chat_handler.go:56-59)却有 `data`。Next.js 统一 `{message, data}`。前端 `lib/schemas.ts` 期望 `data` 字段可选,勉强能过,但不一致。

### A-4. SSE 帧差异 ✅已证实

- **`id` 语义**:Go `internal/app/chat_handler.go:74` `sse.ID(req.ID)` 仅在 connected 帧发一次(session id);Next.js `app/api/chat_stream/route.ts:35` 每个 event 都发 `id: ${Date.now()}`。
- **多余 `[DONE]` 帧**:Go `internal/app/chat_handler.go:110-111` 先 `sse.Event("done","Stream completed")` 再 `sse.Done()`,后者 `swifty_http/sse.go:133-135` 额外发 `data: [DONE]`。Next.js 只发 `event: done`。前端能容忍但行为不一致。
- **connected payload**:Go 用字符串拼接 `+"`+req.ID+`"`,若 `req.ID` 含引号会破坏 JSON;Next.js 用 `JSON.stringify`。

### A-5. CORS 更宽松 ✅已证实

- **Go** `internal/app/app.go:42-43`: `GET,POST,PUT,DELETE,PATCH,OPTIONS` + `Authorization`。
- **Next.js**: 仅 `POST, OPTIONS` + `Content-Type`。
- 注:经核实 `swifty_http/sse.go:24-26`,CORS 中间件设置的 header 会被 `SSE()` 拷进响应头,所以 **SSE 的 CORS 实际是生效的**(Sisyphus P2-4 的担忧不成立)。

---

## 六、死代码 / 代码质量问题(P2-P3)

### Q-1. 🆕 `newEmbedding` 在两个 pipeline 都是死代码

- `internal/ai/agent/chat_pipeline/embedding.go` 和 `internal/ai/agent/knowledge_index_pipeline/embedding.go` 定义 `newEmbedding`,但 retriever/indexer 各自内部 `embedder.New`,**从未调用**(已 grep 确认无调用点)。

### Q-2. `newSearchTool` 死代码 ✅已证实

- `internal/ai/agent/chat_pipeline/search_tool.go` 定义但无 pipeline 调用。引入 `duckduckgo` 依赖却不用。

### Q-3. 变量名 `config` 遮蔽包名 ✅已证实

- `internal/ai/agent/chat_pipeline/tools_node.go:16` 与 `internal/ai/agent/chat_pipeline/prompt.go:21` 局部变量 `config` 遮蔽 `config` 包。

### Q-4. Content 无长度截断 ✅已证实

- **Go**: Eino redis indexer 原样存 content。
- **Next.js** `lib/redis/indexer.ts:17,31`: `MAX_CONTENT_LENGTH=8192` 截断。

### Q-5. Retriever TopK 硬编码 / 无 score 🆕

- **Go** `internal/ai/retriever/retriever.go:34`: `TopK:1` 硬编码;`ReturnFields` 仅 `content`,不返回 score/metadata。
- **Next.js** `lib/redis/retriever.ts:53`: `retrieve(query, topK=1)` 可参;返回 `score = (2-distance)/2`。

### Q-6. `query_internal_docs` 返回原始 Eino Document 🆕

- **Go** `internal/ai/tools/query_internal_docs.go:35-36`: `json.Marshal(resp)` 序列化 `[]*schema.Document`(含 ID/Content/MetaData)。
- **Next.js** `lib/ai/tools/index.ts:34`: `retrieveDocs` 返回 `RetrievedDoc[]`(id/content/metadata/score)。结构不同。

---

## 七、缺失基础设施(P3)

| 项                   | Next.js                        | Go                                            |
| -------------------- | ------------------------------ | --------------------------------------------- |
| `docker-compose.yml` | redis-stack+prometheus+grafana | ❌                                            |
| `prometheus.yml`     | 有                             | ❌                                            |
| `.env.example`       | 有                             | ❌                                            |
| `README.md`          | 有                             | ❌                                            |
| `Makefile`           | npm scripts                    | ❌                                            |
| `Dockerfile`         | 有                             | ❌                                            |
| 测试文件             | ESLint                         | ❌ 零 `*_test.go`                             |
| 结构化日志           | console 统一                   | `fmt.Printf`/`log.Printf`/`log_callback` 混用 |
| 对话记忆持久化       | 内存(两者均无持久化)           | 内存                                          |

---

## 八、`bug.md` 遗漏项 🆕

Go 项目自带 `bug.md` 记录了 9 项,但**遗漏以下关键问题**:

- ❌ `chatRequest` JSON tag 大小写(P0-1)
- ❌ `SystemMessage` 误用(P0-2)
- ❌ System prompt markdown 矛盾(P0-3)
- ❌ Redis key prefix / vector field / `_source` 值不一致(D-1/D-2/D-3)
- ❌ Executor 缺 mysql_crud(F-1)
- ❌ Executor MaxIterations=999999(F-2)
- ❌ MCP 无降级拖垮主链路(P0-7)
- ❌ Memory 无 LRU(F-3)
- ❌ deleteBySource 无锁(F-4)
- ❌ Log topic 硬编码(F-5)
- ❌ model name / fileDir / max_tokens 默认值不一致
- ❌ mysql_crud 双重执行 + 非查询返回空串(P0-5)
- ❌ aiOpsQuery 字面引号 + 重复定义(F-6)

---

## 九、修复优先级建议

**立即修复(上线前阻断)**:

1. P0-1: JSON tag 改小写 `id`/`question`(1 行)
2. P0-2: `schema.SystemMessage` → `schema.AssistantMessage`(3 处)
3. P0-3: prompt 改 `Output markdown only`
4. P0-4: 9 处 `log.Fatal` → `return "", err`
5. P0-5: 删 mysql_crud stdin + 修双重执行 + 非查询返回成功消息
6. P0-6: 删 `queryPrometheusAlerts` early return + 加 `prometheus_url` 配置
7. P0-7: MCP 加缓存 + 降级返回空工具集

**数据层统一(决定迁移方向后)**: 8. D-1/D-2/D-3: 统一 key prefix(`biz:`)、vector field(`vector`)、`_source`(basename)——或写迁移脚本 9. D-5: 加 index 维度漂移检测

**功能对齐**: 10. F-1: executor 补 `NewMysqlCrudTool()` 11. F-2: `MaxIterations` 999999 → 10 12. F-3: memory 加 LRU + `All()` 返回拷贝 13. F-4: deleteBySource 加 SETNX 锁 + 分批 14. F-5/F-6: log topic 外部化 + 修 aiOpsQuery 引号 + 抽公共常量

**其余**: 对齐 config 默认值、请求校验、错误响应 `data:null`、SSE 行为、补基础设施文件。

---

## 十、统计

- P0 致命: 7 项
- 数据层不兼容: 5 项
- 功能未对齐: 9 项
- 配置未对齐: 10 项
- API 行为: 5 项
- 死代码/质量: 6 项
- 基础设施: 8 项

其中 🆕 标注为 4 份已有报告遗漏或低估、由本次独立证实的新发现(共 11 项,含 1 个 P0)。

---

## 十一、文件映射参考

| Next.js                               | Go                                            | 状态                  |
| ------------------------------------- | --------------------------------------------- | --------------------- |
| app/api/chat/route.ts                 | internal/app/chat_handler.go:handleChat       | 对齐但有 bug          |
| app/api/chat_stream/route.ts          | internal/app/chat_handler.go:handleChatStream | 对齐但有 bug          |
| app/api/upload/route.ts               | internal/app/file_handler.go:handleFileUpload | 对齐                  |
| app/api/ai_ops/route.ts               | internal/app/ai_ops_handler.go:handleAIOps    | 对齐但 prompt 差异    |
| lib/config.ts                         | internal/config/config.go + config.json       | 部分对齐              |
| lib/memory.ts                         | internal/utility/mem/mem.go                   | 缺 LRU                |
| lib/ai/models.ts                      | internal/ai/models/models.go                  | 缺 thinking/signature |
| lib/ai/embedder.ts                    | internal/ai/embedder/embedder.go              | 缺 Ollama             |
| lib/ai/loader.ts                      | internal/ai/loader/loader.go                  | \_source 值不同       |
| lib/ai/callbacks.ts                   | internal/utility/log_callback/log_callback.go | 对齐                  |
| lib/ai/tools/schemas.ts               | (内嵌于 Go jsonschema tag)                    | 隐式                  |
| lib/ai/tools/operations.ts            | internal/ai/tools/\*.go                       | 有 bug                |
| lib/ai/tools/index.ts                 | internal/ai/agent/chat_pipeline/tools_node.go | 对齐                  |
| lib/ai/tools/query-log.ts             | internal/ai/tools/mcp_tool.go                 | 缺缓存+降级           |
| lib/ai/pipelines/chat.ts              | internal/ai/agent/chat_pipeline/              | prompt 不一致         |
| lib/ai/pipelines/knowledge-index.ts   | internal/ai/agent/knowledge_index_pipeline/   | 对齐                  |
| lib/ai/pipelines/plan-execute-replan/ | internal/ai/agent/plan_execute_replan/        | executor 迭代数不一致 |
| lib/redis/client.ts                   | internal/utility/redis/redis.go               | prefix 不一致         |
| lib/redis/indexer.ts                  | internal/ai/indexer/indexer.go                | 缺锁+截断             |
| lib/redis/retriever.ts                | internal/ai/retriever/retriever.go            | vector field 不一致   |
| lib/schemas.ts                        | (服务端校验不同)                              | N/A                   |
| .env.example                          | (无)                                          | 缺失                  |
| docker-compose.yml                    | (无)                                          | 缺失                  |
| prometheus.yml                        | (无)                                          | 缺失                  |
