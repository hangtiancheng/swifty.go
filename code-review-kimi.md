# Next.js → Go 后端迁移 Code Review 报告

> **Review 范围**: Next.js 后端 (`/apps/swifty-agent/lib/`, `app/api/`) → Go 项目 (`/swifty_agent/internal/`, `cmd/`)
> **排除**: 前端工程 (Next.js `app/` 页面组件, Go `fe/`)
> **Review 维度**: API & Handler 层 / AI Pipeline & Tools 层 / 基础设施层

---

## 一、总览

| 维度                   | 对齐状态    | 关键问题数        |
| ---------------------- | ----------- | ----------------- |
| API & Handler 层       | ⚠️ 部分对齐 | 5 Bug + 5 未对齐  |
| AI Pipeline & Tools 层 | ⚠️ 部分对齐 | 5 Bug + 12 未对齐 |
| 基础设施层             | ⚠️ 部分对齐 | 4 Bug + 10 未对齐 |

**总计: 14 个 Bug + 27 个未对齐项**

---

## 二、Critical Bugs (必须修复)

### BUG-C1: JSON 字段名大小写错误 — Chat 接口完全不可用

- **文件**: `internal/app/chat_handler.go:20-21`
- **问题**: struct tag 使用 `json:"Id"` / `json:"Question"` (大写开头)，但前端发送的是小写 `id` / `question`。Go `encoding/json` 在有 tag 时严格匹配大小写，导致两个字段反序列化后均为空字符串。
- **影响**: Chat 功能完全失效，agent 收到空查询。
- **修复**: 改为 `json:"id"` 和 `json:"question"`。

### BUG-C2: `log.Fatal` 在 HTTP Server 中会杀死整个进程

- **文件**: `internal/ai/tools/query_internal_docs.go:29,33,40`; `internal/ai/tools/mysql_crud.go:34,47,53`
- **问题**: Tool 执行函数中使用 `log.Fatal(err)`，Redis/DB 出错时直接终止整个 Go 进程。Next.js 版本优雅地返回错误或传播异常。
- **修复**: 全部替换为 `return "", err`。

### BUG-C3: `mysql_crud` tool 读取 stdin — HTTP 上下文中永久阻塞

- **文件**: `internal/ai/tools/mysql_crud.go:38-44`
- **问题**: 从 `os.Stdin` 读取用户确认 (`y/n`)。HTTP server 没有终端，此操作永远阻塞。此外 line 46 先执行 `db.Exec(input.SQL)`，line 52 又执行 `db.Raw(input.SQL).Scan(...)`，查询类 SQL 被执行了两次。
- **修复**: 移除 stdin 交互；重构为只执行一次 SQL；用 error return 替代 `log.Fatal`。

### BUG-C4: Plan-Execute-Replan Executor 缺少 `mysql_crud` Tool

- **文件**: `internal/ai/agent/plan_execute_replan/executor.go:23-26`
- **问题**: Next.js 版本的 `buildTools()` 包含全部 4 个 builtin tools（含 `mysql_crud`）。Go 版本只注册了 MCP tools + PrometheusAlertsQueryTool + QueryInternalDocsTool + GetCurrentTimeTool，遗漏了 `NewMysqlCrudTool()`。
- **修复**: 添加 `NewMysqlCrudTool()` 到 executor 的 tool 列表。

### BUG-C5: Plan-Execute-Replan Executor MaxIterations = 999999 — 无限循环风险

- **文件**: `internal/ai/agent/plan_execute_replan/executor.go:40`
- **问题**: Next.js 版本设置 `stopWhen: isStepCount(10)` (每步最多 10 次 tool call)。Go 版本设为 999999，如果模型持续调用 tool 将无限循环。
- **修复**: 设为合理值如 `10`。

---

## 三、Medium Bugs

### BUG-M1: SSE `id` 字段语义不同

- **Next.js**: 每个 SSE event 用 `Date.now()` 时间戳作为 `id`
- **Go** (`chat_handler.go:74`): 用客户端 session ID 作为 `id`，且只发一次
- **影响**: 当前前端跳过 `id:` 行所以不 break，但未来 SSE 重连逻辑会行为不一致。

### BUG-M2: SSE `Done()` 多发一个 `[DONE]` data frame

- **文件**: `swifty_http/sse.go:133-135` + `chat_handler.go`
- **问题**: `handleChatStream` 先发 `sse.Event("done", ...)` 再调 `sse.Done()`，后者额外发送 `data: [DONE]\n\n`。Next.js 版本不发这个额外帧。
- **影响**: 前端能容忍，但与源行为不一致。

### BUG-M3: Error Response 缺少 `data` 字段

- **文件**: `swifty_http/context.go:51-55`
- **问题**: `Throw()` 返回 `{"message": "..."}` 不含 `"data": null`。Next.js 统一返回 `{ message, data }` 结构。
- **修复**: 在 `Throw` 中添加 `"data": nil`。

### BUG-M4: System Prompt 矛盾 — Markdown vs Plain Text

- **Go** (`prompt.go:53`): `"Output must not contain markdown syntax; use plain text only"`
- **Next.js** (`chat.ts:39`): `"Output markdown only"`
- **影响**: 输出格式完全相反。

### BUG-M5: System Prompt 硬编码敏感信息

- **Go** (`prompt.go:42`): 硬编码 `Log topic region: ap-guangzhou; log topic ID: 869830db-...`
- **Next.js**: 从环境变量 `LOG_TOPIC_REGION` / `LOG_TOPIC_ID` 读取，未设置时省略该行
- **影响**: 安全风险 + 可移植性问题。

### BUG-M6: `query_prometheus_alerts` 死代码 — 永远返回空结果

- **文件**: `internal/ai/tools/query_metrics_alerts.go:54-57`
- **问题**: 函数第 56 行有 early return，后续所有代码不可达。Next.js 版本实际查询 Prometheus。
- **修复**: 移除 early return；添加 `PrometheusBaseUrl` 到 config。

### BUG-M7: AI Ops Prompt 文本差异

- **Go** (`ai_ops_handler.go:12-26`): 使用不同的章节标题 ("Alert Processing Details" vs "Alert Handling Details")，多出 `## Active Alert List` 节，且每行被双引号包裹（LLM 收到字面引号字符）。
- **Next.js** (`plan-execute-replan/index.ts:18-31`): 无引号包裹。
- **影响**: 生成的报告格式不同。

### BUG-M8: Delete-by-Source 无并发锁

- **Go** (`index_file.go:48-65`): 无锁直接 FT.SEARCH + DEL
- **Next.js** (`indexer.ts:46-77`): 使用 SETNX 分布式锁 (30s TTL)
- **影响**: 并发上传同文件时可能产生重复数据或丢失。

### BUG-M9: Conversation Memory 无 LRU 淘汰 — 内存泄漏

- **Go** (`mem/mem.go`): 纯 map，无容量上限
- **Next.js** (`memory.ts`): LRU 淘汰，MAX_SESSIONS = 100
- **影响**: 长期运行的 server 内存持续增长。

---

## 四、Low Bugs / 行为差异

### BUG-L1: Request Validation 缺失

- **Next.js**: Zod schema 验证 `id` 和 `question` 非空 (`.min(1)`)，返回 400 + 具体错误信息
- **Go**: `ctx.BindJSON()` 仅检查 JSON 语法，空字符串直接通过

### BUG-L2: Chat Error Response 丢失结构化信息

- **Next.js**: 捕获 APICallError 并提取 name/statusCode/url/responseBody/responseHeaders
- **Go**: 仅 `err.Error()`，丢失所有上下文

### BUG-L3: CORS 更宽松

- **Next.js**: 仅允许 `POST, OPTIONS` + `Content-Type` header
- **Go**: 全局中间件允许 `GET,POST,PUT,DELETE,PATCH,OPTIONS` + `Authorization` header

### BUG-L4: Delete-by-Source 未分批删除

- **Next.js**: 每批 1000 keys
- **Go**: 单次 FT.SEARCH 取 10000 keys 后一次性 DEL，大 source 可能删不全且阻塞 Redis

---

## 五、未对齐项清单

### 配置层

| #   | Next.js                                | Go                                            | 状态                       |
| --- | -------------------------------------- | --------------------------------------------- | -------------------------- |
| 1   | `redis.url` (完整 URL)                 | `redis.addr` (host:port) + password + db 分离 | 机制不同，功能等价         |
| 2   | `redis.indexName`                      | ❌ 缺失 (可能硬编码)                          | 需确认                     |
| 3   | `redis.keyPrefix = "biz:"`             | `consts.RedisKeyPrefix = "doc:"`              | ⚠️ 数据不兼容              |
| 4   | `anthropic.thinking` (boolean)         | ❌ 缺失                                       | 无法开启 extended thinking |
| 5   | `anthropic.maxOutputTokens = 8192`     | `think_chat_model.max_tokens = 4096`          | 默认值不同                 |
| 6   | `embeddingProvider` (dashscope/ollama) | ❌ 仅 dashscope                               | 缺 Ollama 支持             |
| 7   | `prometheusBaseUrl`                    | ❌ 缺失 (hardcoded + unreachable)             | 需添加到 config            |
| 8   | `fileDir = "./data/docs"`              | `FileDir = "./docs"`                          | 默认路径不同               |
| 9   | `provider` (env-based)                 | `model_provider` (JSON-based)                 | 机制不同                   |

### AI Pipeline 层

| #   | Next.js                                         | Go                                   | 状态                 |
| --- | ----------------------------------------------- | ------------------------------------ | -------------------- |
| 10  | Ollama embedding provider                       | ❌ 不支持                            | 缺功能               |
| 11  | Anthropic thinking config + fetch wrapper       | ❌ 不支持                            | 缺功能               |
| 12  | Chat pipeline 写回 memory (`mem.setMessages()`) | ❌ pipeline 不写回，需 handler 处理  | 需验证 handler       |
| 13  | Knowledge index content truncation (8192 chars) | ❌ 无截断                            | 大文档可能超限       |
| 14  | Retriever 返回 score `(2-d)/2`                  | ❌ eino retriever 不暴露 score       | 下游无法按相关性过滤 |
| 15  | Vector field name = `vector`                    | Vector field name = `vector_content` | ⚠️ 数据不兼容        |
| 16  | Metadata 存为 JSON string                       | Metadata 各 key 存为独立 hash field  | 存储格式不同         |
| 17  | Retriever TopK 可参数化                         | TopK = 1 硬编码                      | 灵活性不足           |

### 基础设施层

| #   | Next.js                                      | Go                    | 状态                            |
| --- | -------------------------------------------- | --------------------- | ------------------------------- |
| 18  | Redis 指数退避重连策略                       | ❌ 无显式重试配置     | 韧性不足                        |
| 19  | FT.INFO 检测 vector dimension 变化并重建索引 | ❌ 仅检查索引是否存在 | 切换 embedding model 后静默失败 |
| 20  | Memory LRU eviction (MAX_SESSIONS=100)       | ❌ 无上限             | 内存泄漏                        |
| 21  | Delete-by-source SETNX 分布式锁              | ❌ 无锁               | 竞态条件                        |
| 22  | Delete-by-source 分批删除 (1000/batch)       | ❌ 单次最多 10000     | 大 source 删不全                |
| 23  | Memory window size = 6                       | ✅ MaxWindowSize = 6  | 已对齐                          |
| 24  | Response schemas (chat/upload/ai_ops)        | ✅ 结构一致           | 已对齐                          |
| 25  | Loader (文件加载)                            | ✅ 已对齐             | 已对齐                          |
| 26  | Log callback                                 | ✅ Go 版本更丰富      | 已对齐                          |
| 27  | Knowledge index pipeline 主流程              | ✅ 已对齐             | 已对齐                          |

---

## 六、修复优先级建议

### P0 — 阻塞性 (上线前必须修复)

1. **BUG-C1**: 修复 `chat_handler.go` JSON tag 大小写
2. **BUG-C2**: 替换所有 tool 中的 `log.Fatal` 为 error return
3. **BUG-C3**: 移除 `mysql_crud` 的 stdin 读取 + 修复双重执行
4. **BUG-C4**: 给 plan-execute-replan executor 添加 `mysql_crud` tool
5. **BUG-C5**: 将 executor MaxIterations 从 999999 改为 10

### P1 — 高优先级

6. **BUG-M4/M5**: 对齐 system prompt (markdown 指令 + 外部化 log topic 配置)
7. **BUG-M6**: 修复 Prometheus alerts 死代码 + 添加 config 字段
8. **BUG-M7**: 对齐 AI Ops prompt 文本，移除多余引号
9. **BUG-M8**: 给 delete-by-source 添加分布式锁
10. **BUG-M9**: 给 conversation memory 添加 LRU 淘汰
11. **MISSING-3**: 统一 Redis key prefix (`biz:` vs `doc:`) 或提供迁移脚本
12. **MISSING-15**: 统一 vector field name (`vector` vs `vector_content`)

### P2 — 中优先级

13. **BUG-M1/M2**: 对齐 SSE 行为 (id 语义 + Done frame)
14. **BUG-M3**: Error response 添加 `data: nil`
15. **BUG-L1**: 添加请求参数校验
16. **MISSING-10/11**: 添加 Ollama embedding + Anthropic thinking 支持
17. **MISSING-18/19**: Redis 重连策略 + dimension 检测
18. **MISSING-13/14**: Content truncation + retriever score 暴露

### P3 — 低优先级

19. **BUG-L2/L3**: Error response 丰富化 + CORS 收紧
20. **BUG-L4**: Delete 分批
21. **MISSING-17**: Retriever TopK 可配置化
22. **Config 默认值对齐**: fileDir, maxTokens 等

---

## 七、已正确对齐的部分 ✅

以下模块迁移质量良好，无需修改：

- **Response schemas**: chat / upload / ai_ops 三个接口的响应结构完全一致
- **Loader**: 文件加载逻辑已对齐
- **Log callback**: Go 版本甚至比 Next.js 更丰富
- **Knowledge index pipeline 主流程**: orchestration 逻辑已对齐
- **Memory window size**: 两端均为 6
- **CLI dev tools** (`cmd/`): 作为开发辅助工具，不影响生产

---

_Report generated by Kimi Code CLI Agent Team (3 agents: API_HANDLERS, AI_PIPELINE_TOOLS, INFRASTRUCTURE)_
