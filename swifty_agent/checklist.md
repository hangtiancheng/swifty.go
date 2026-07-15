# Swifty Agent 迁移对齐任务清单

> 配套文档: [review.md](./review.md)。本清单按优先级组织,每项标注涉及文件、修改要点、验证方式与对应 review 编号。
> 复选框约定: `[ ]` 待办 / `[~]` 进行中 / `[x]` 已完成 / `[!]]` 阻塞(需决策)。

---

## 决策前置项(开工前必须拍板)

- [x] **DEC-1 数据迁移策略 = A**: Go 完全接管,清空旧数据重建。数据层任务(D-1/D-2/D-3/D-4)只需改 Go 常量,无需迁移脚本;切换时 `FLUSHDB` + 重建索引
- [x] **DEC-2 LLM provider 默认值 = `deepseek-v3-2-251201`**: 保持 Go 现状,不改 model name(review 中该项取消)
- [x] **DEC-3 必须保留 Ollama / Anthropic thinking 能力**: C-4(Ollama embedding)、C-5(Anthropic thinking + signature 修补)升级为 **P1 高优先级**,必须实现

---

## P0 — 致命,上线前必须修复

### P0-1 修复 chatRequest JSON tag 大小写

- [x] 改 `internal/app/chat_handler.go:20-22`: `json:"Id"`→`json:"id"`, `json:"Question"`→`json:"question"`
- [x] 验证: `curl -X POST localhost:6872/api/chat -d '{"id":"s1","question":"hi"}'` 不再返回空 answer
- ref: review P0-1

### P0-2 助手回复改用 AssistantMessage

- [x] `internal/app/chat_handler.go:53`: `schema.SystemMessage(out.Content)` → `schema.AssistantMessage(out.Content)`
- [x] `internal/app/chat_handler.go:103`: `schema.SystemMessage(resp)` → `schema.AssistantMessage(resp)`
- [x] `cmd/chat/main.go:65`: 同样修改
- [x] 验证: 连续两轮对话,第二轮 LLM 不再把上轮助手回复当系统指令
- ref: review P0-2

### P0-3 对齐 System Prompt 输出格式

- [x] `internal/ai/agent/chat_pipeline/prompt.go:53`: `Output must not contain markdown syntax; use plain text only` → `Output markdown only`
- [x] 验证: 对比 `lib/ai/pipelines/chat.ts:39` 逐字一致
- ref: review P0-3

### P0-4 消除工具函数中的 log.Fatal(9 处)

- [x] `internal/ai/tools/mysql_crud.go:34,47,53,62` → `return "", fmt.Errorf(...)`
- [x] `internal/ai/tools/get_current_time.go:54` → `return nil, err`(注意此处在 `NewGetCurrentTimeTool` 构造期,改为返回 `tool.InvokableTool, error`)
- [x] `internal/ai/tools/query_internal_docs.go:29,33,40` → `return "", err` / 返回 error
- [x] `internal/ai/tools/query_metrics_alerts.go:158` → 返回 error
- [x] 验证: `go build ./...` 通过;人为触发工具错误,服务不退出
- ref: review P0-4

### P0-5 修复 mysql_crud stdin 阻塞 + 双重执行

- [x] 删 `internal/ai/tools/mysql_crud.go:38-44` 的 `bufio.NewScanner(os.Stdin)` 确认逻辑
- [x] 重构执行逻辑: 按 `operate_type` 分支,`query` 用 `db.Raw().Scan()` 只执行一次;`insert/update/delete` 用 `db.Exec()` 只执行一次
- [x] 非查询返回 `{"success":true,"message":"Executed X sql"}` 而非空串(对齐 `lib/ai/tools/operations.ts:148-149`)
- [x] 验证: 单元测试覆盖 4 种 operate_type
- ref: review P0-5

### P0-6 恢复 query_prometheus_alerts 功能

- [x] 删 `internal/ai/tools/query_metrics_alerts.go:56` 的 early return
- [x] `internal/config/config.go` 增 `PrometheusURL string` 字段;`config.json` 增 `"prometheus_url"` 默认 `http://127.0.0.1:9090`
- [x] `query_metrics_alerts.go` 改为接收 cfg 或读取全局配置的 URL(注意工具构造签名可能需调整)
- [x] 修复 `query_metrics_alerts.go:122` `return string(b), err` → 错误时只返回 JSON 字符串、`return string(b), nil`(对齐 Next.js 不抛错)
- [x] 修正工具描述 `query_metrics_alerts.go:110` 与实际返回结构一致
- [x] 验证: 启动 prometheus(`docker-compose up prometheus`),调用工具返回真实告警
- ref: review P0-6, F-9

### P0-7 MCP 工具加缓存 + 降级

- [x] `internal/ai/tools/mcp_tool.go`: 引入 `sync.Once`/包级缓存 `var (cachedCli; cachedTools; mcpOnce)`;`GetLogMcpTool` 命中缓存直接返回
- [x] 失败时 `log.Printf` 警告并 `return []tool.BaseTool{}, nil`(空集,不返回 error)
- [x] 验证: 关闭 MCP server,`/api/chat` 仍能正常响应(无日志工具)
- ref: review P0-7

---

## 数据层统一(依赖 DEC-1)

### D-1 统一 Redis Key Prefix

- [x] 若 DEC-1=A/B: `internal/consts/consts.go:6` `doc:` → `biz:`
- [x] 若 DEC-1=A: 清空 Redis `FLUSHDB` 后重建索引
- [x] 若 DEC-1=B: 写迁移脚本 `scan doc:* → rename biz:*`
- ref: review D-1

### D-2 统一 Vector Field Name

- [x] `internal/consts/consts.go:16` `vector_content` → `vector`
- [x] 同步改 `internal/ai/retriever/retriever.go:32` `VectorField`(用常量即可)
- [x] 注意:Eino redis indexer 默认用 `vector_content`,需查 eino-ext 是否支持自定义 field;若不支持需自写 indexer 或 fork
- [x] 重建索引(`FT.DROPINDEX idx:biz` + 重启服务自动建)
- ref: review D-2

### D-3 统一 \_source 字段值(basename)

- [x] `internal/ai/agent/knowledge_index_pipeline/index_file.go:47` 取 source 后做 `filepath.Base(source)` 再用于去重查询
- [x] 或: 在 loader 之后包一层 transformer 把 `_source` 改为 basename(对齐 `lib/ai/loader.ts:13`)
- [x] 验证: 索引后 `HGETALL doc:<id>` 的 `_source` 只含文件名
- ref: review D-3

### D-4 对齐 Index Schema(metadata 字段)

- [x] `internal/utility/redis/redis.go:46-56` FT.CREATE 增 `metadata TEXT`(或保留为 hash 非索引字段)
- [x] 评估:Eino redis indexer 是否存 metadata 为 JSON string;若存为独立 hash field 则 schema 需相应调整
- [x] retriever `ReturnFields` 视下游需要决定是否加 `metadata`
- ref: review D-4

### D-5 加 Index 维度漂移检测

- [x] `internal/utility/redis/redis.go:36-57` `ensureIndex`: `FT.INFO` 成功后解析 attributes,比对 vector 字段 DIM 与 `cfg.EmbeddingModel.Dimensions`,不一致则 `FT.DROPINDEX` 后重建
- [x] 参考 `lib/redis/client.ts:39-69` 实现
- [x] 验证: 切换 dimensions 2048→768,重启后索引自动重建
- ref: review D-5

---

## P1 — 功能对齐

### F-1 Executor 补 mysql_crud 工具

- [x] `internal/ai/agent/plan_execute_replan/executor.go:26` 后追加 `toolList = append(toolList, tools.NewMysqlCrudTool())`
- [x] 验证: 对比 chat pipeline `tools_node.go:35-38` 工具集一致
- ref: review F-1

### F-2 Executor MaxIterations 降到 10

- [x] `internal/ai/agent/plan_execute_replan/executor.go:40` `999999` → `10`
- [x] 验证: 单步工具调用超 10 次自动停止
- ref: review F-2

### F-3 Memory 加 LRU + All() 返回拷贝

- [x] `internal/utility/mem/mem.go`: 引入 `container/list` 维护访问顺序,`MaxSessions=100`,`Get` 命中时移到队尾,超限淘汰队首
- [x] `mem.go:63-66` `All()` 改 `return append([]*schema.Message(nil), m.Messages...)`
- [x] 参考 `lib/memory.ts:10-29`
- [x] 验证: 压测 200 个 session,内存稳定在 100 条
- ref: review F-3

### F-4 deleteBySource 加分布式锁 + 分批

- [x] `internal/ai/agent/knowledge_index_pipeline/index_file.go:48-65` 改造:
  - SETNX `doc:lock:delete:<escaped>` 30s TTL,获取失败返回 error
  - 循环 `FT.SEARCH LIMIT 0 1000` + 批量 `DEL`,直到 total=0
  - finally 释放锁
- [x] 参考 `lib/redis/indexer.ts:46-77`
- [x] 验证: 并发上传同文件无重复数据
- ref: review F-4

### F-5 Log Topic 配置外部化

- [x] `internal/config/config.go` 增 `LogTopicRegion string` / `LogTopicID string`(JSON tag `log_topic_region`/`log_topic_id`)
- [x] `config.json` 增对应字段(默认空)
- [x] `internal/ai/agent/chat_pipeline/prompt.go:42` 改为模板占位或条件拼接:两者均非空才输出该行
- [x] 验证: 未配置时 prompt 不含 log topic 行;配置后含正确值
- ref: review F-5

### F-6 修复 aiOpsQuery 引号 + 抽公共常量

- [x] 新建 `internal/app/ai_ops_query.go`(或放 `internal/ai/agent/plan_execute_replan/`),定义 `AIOnOpsQuery` 常量,**去掉每行字面双引号**
- [x] 章节标题对齐 Next.js: `# Alert Handling Details` / `## Alert Root Cause Analysis N (the Nth alert)` / `## Handling Procedure Execution N (the Nth alert)`
- [x] `internal/app/ai_ops_handler.go` 与 `cmd/ai_ops/main.go` 引用同一常量
- [x] 验证: `go build`;对比 `lib/ai/pipelines/plan-execute-replan/index.ts:18-31` 逐字一致
- ref: review F-6

### F-7 对齐 Date 格式

- [x] `internal/ai/agent/chat_pipeline/lambda_func.go:21` 评估是否改用 `time.Now().Format("1/2/2006, 3:04:05 PM")` 对齐 `toLocaleString("en-US")`
- [x] 或: 双方约定统一格式(如 ISO8601),需同步改 Next.js(若已上线则谨慎)
- ref: review F-7

### F-8 对齐 get_current_time 精度

- [x] `internal/ai/tools/get_current_time.go:34` `2006-01-02 15:04:05.000000` → `2006-01-02 15:04:05.000`(毫秒 3 位)
- [x] 工具描述 "microseconds" → "milliseconds"
- [x] 验证: 对比 `lib/ai/tools/operations.ts:22-27`
- ref: review F-8

---

## P2 — 配置与 API 行为对齐

### C-1 对齐 config 默认值

- [x] `internal/config/config.go:105` `./docs` → `./data/docs`
- [x] `config.json:7,12` `deepseek-v3-2-251201` → 按 DEC-2 决定
- [x] `config.json` think/quick 增 `"max_tokens": 8192`(Anthropic 场景)
- [x] 验证: 对比 `lib/config.ts` 全字段
- ref: review 四、配置层

### C-2 新增 .env.example

- [x] 参照 `swifty-cli/.env.example` 生成 Go 版(说明 config.json 对应字段)
- ref: review 四、配置层

### C-3 补 Prometheus URL 配置(与 P0-6 合并)

- [x] 见 P0-6
- ref: review 四、配置层

### A-1 加请求参数校验

- [x] `internal/app/chat_handler.go:28-31` BindJSON 后检查 `req.ID==""||req.Question==""` → `ctx.Throw(400, "missing id or question")`
- [x] 同样用于 handleChatStream
- [x] 验证: 空 body 返回 400
- ref: review A-1

### A-2 /api/chat 错误响应结构化

- [ ] `internal/app/chat_handler.go:48` 改为返回 `{message: JSON.stringify({name,message,statusCode,...}), data: null}`(对齐 `app/api/chat/route.ts:47-59`)
- [ ] 需从 err 提取结构化字段(可能需自定义 error 类型)
- ref: review A-2

### A-3 Throw 补 data: null

- [x] `swifty_http/context.go:54` `ctx.Body = H{"message": msg}` → `H{"message": msg, "data": nil}`
- [x] 验证: 所有错误响应含 `"data":null`
- ref: review A-3

### A-4 对齐 SSE 行为

- [x] `internal/app/chat_handler.go:74` 删除 `sse.ID(req.ID)`(或改为每帧 `id: <timestamp>`,但 swifty_http SSE 需扩展)
- [x] `chat_handler.go:110-111` 删除多余 `sse.Done()`,仅保留 `sse.Event("done","Stream completed")`(对齐 Next.js)
- [x] `chat_handler.go:75` connected payload 改用 `json.Marshal` 而非字符串拼接
- [x] 验证: 前端 EventSource 接收帧与 Next.js 一致
- ref: review A-4

### A-5 收紧 CORS

- [x] `internal/app/app.go:42-43` `Allow-Methods` → `POST, OPTIONS`;`Allow-Headers` → `Content-Type`
- [x] 验证: 前端预检通过;其他方法被拒
- ref: review A-5

### C-4 补 Ollama embedding 支持(若 DEC-3 保留)

- [x] `internal/ai/embedder/embedder.go` 按 `cfg.EmbeddingProvider` 分支: dashscope / ollama
- [x] `internal/config/config.go` 增 `EmbeddingProvider string` + `Ollama` 子结构
- [x] `config.json` 增字段
- ref: review 四、配置层

### C-5 补 Anthropic thinking 支持(若 DEC-3 保留)

- [x] `internal/ai/models/models.go` Anthropic 分支增 thinking 配置
- [x] 评估 Eino claude 是否支持 extended thinking;不支持需自定义
- [x] 评估 Anthropic signature 修补:Eino claude 是否处理;不处理需加 fetch 拦截
- ref: review 四、配置层

---

## P2 — 死代码 / 代码质量

### Q-1 删除未调用的 newEmbedding

- [x] 删 `internal/ai/agent/chat_pipeline/embedding.go`
- [x] 删 `internal/ai/agent/knowledge_index_pipeline/embedding.go`
- [x] 验证: `go build ./...` 通过
- ref: review Q-1

### Q-2 处理 newSearchTool

- [x] 选项 A: 删 `internal/ai/agent/chat_pipeline/search_tool.go` + 移除 `duckduckgo` 依赖
- [x] 选项 B: 在 `tools_node.go` 注册 `newSearchTool()`(若需 web 搜索)
- ref: review Q-2

### Q-3 消除变量名遮蔽

- [x] `internal/ai/agent/chat_pipeline/tools_node.go:16` `config :=` → `agentCfg :=`;后续 `config.ToolCallingModel` 等同步改名
- [x] `internal/ai/agent/chat_pipeline/prompt.go:21` `config :=` → `tplCfg :=`
- [x] 验证: `go vet ./...` 无 shadow 警告
- ref: review Q-3

### Q-4 加 content 长度截断

- [x] `internal/consts/consts.go` 增 `MaxContentLength = 8192`
- [x] 评估:Eino redis indexer 是否支持 content pre-hook;不支持则在 transformer 阶段截断
- [x] 验证: 超长 chunk 写入后 content ≤ 8192
- ref: review Q-4

### Q-5 Retriever TopK 可参 + 返回 score

- [x] `internal/ai/retriever/retriever.go:34` `TopK:1` 改为从配置或参数传入
- [x] `ReturnFields` 增 `__vector_score`;retriever 输出转 `distance→score = (2-d)/2`
- [x] 评估 Eino redis retriever 是否暴露 score;不暴露需自写
- ref: review Q-5

### Q-6 对齐 query_internal_docs 返回结构

- [x] `internal/ai/tools/query_internal_docs.go:35-36` 把 `[]*schema.Document` 转为 `{id,content,metadata,score}` 结构再 marshal
- ref: review Q-6

---

## P3 — 基础设施补全

### I-1 补 docker-compose.yml

- [x] 参照 `swifty-cli/apps/swifty-agent/docker-compose.yml`(redis-stack + prometheus + grafana)生成 Go 版
- [x] 补 `prometheus.yml`
- ref: review 七

### I-2 补 Dockerfile

- [x] 多阶段构建 `golang:1.25-alpine` → `alpine`,EXPOSE 6872
- ref: review 七

### I-3 补 Makefile

- [x] `build` / `run` / `test` / `tidy` / `docker-up` 目标
- ref: review 七

### I-4 补 README.md

- [x] 项目简介、配置说明、启动步骤、API 列表
- ref: review 七

### I-5 补测试文件

- [ ] `internal/app/*_test.go`: chat/upload/ai_ops handler
- [ ] `internal/ai/tools/*_test.go`: mysql_crud(用 sqlmock)/get_current_time
- [ ] `internal/utility/mem/mem_test.go`: LRU 淘汰
- [ ] `internal/utility/redis/redis_test.go`: ensureIndex 维度检测
- ref: review 七

### I-6 统一日志

- [ ] 引入 `log/slog`,替换 `fmt.Printf`/`log.Printf`
- [ ] `internal/utility/log_callback/log_callback.go` 改用 slog
- ref: review 七

### I-7 对话记忆持久化(可选)

- [ ] 评估是否需 Redis 持久化(Next.js 也未做,可降级)
- ref: review 七

### I-8 Redis 重连策略

- [x] `internal/utility/redis/redis.go:18-23` `redis.Options` 增 `MaxRetries`/`MinRetryBackoff`/`MaxRetryBackoff`
- [x] 参考 `lib/redis/client.ts:29` 指数退避
- ref: review 四、配置层

---

## 验收 Checklist(全部 P0/P1 完成后)

- [ ] `go build ./...` 通过
- [ ] `go vet ./...` 无警告
- [ ] `go test ./...` 通过
- [ ] 4 个 API 端到端打通: chat / chat_stream / upload / ai_ops
- [ ] MCP server 关闭时 chat 仍可用(P0-7 降级验证)
- [ ] 工具错误不再导致服务退出(P0-4 验证)
- [ ] 多轮对话上下文正确(P0-2 验证)
- [ ] 与 Next.js 前端联调通过(若共用前端)
- [ ] 数据层与 Next.js 互通(若 DEC-1=B/C)
- [ ] `bug.md` 更新或标注已修复项
