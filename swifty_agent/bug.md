# Swifty Agent 缺陷清单

## P0 -- 严重缺陷

### 1. 工具函数中使用 log.Fatal 导致进程崩溃

工具函数在 AI agent 运行时被回调，使用 `log.Fatal` 会直接终止整个 HTTP 服务进程。应改为 `return "", err`。

涉及文件和行号:

- `internal/ai/tools/mysql_crud.go:34` -- `gorm.Open` 失败时 `log.Fatal(err)`
- `internal/ai/tools/mysql_crud.go:47` -- `db.Exec` 失败时 `log.Fatal(err)`
- `internal/ai/tools/mysql_crud.go:53` -- `db.Raw.Scan` 失败时 `log.Fatal(err)`
- `internal/ai/tools/mysql_crud.go:62` -- `InferOptionableTool` 失败时 `log.Fatal(err)`
- `internal/ai/tools/get_current_time.go:54` -- `InferOptionableTool` 失败时 `log.Fatal(err)`
- `internal/ai/tools/query_internal_docs.go:29` -- `NewMilvusRetriever` 失败时 `log.Fatal(err)`
- `internal/ai/tools/query_internal_docs.go:33` -- `rr.Retrieve` 失败时 `log.Fatal(err)`
- `internal/ai/tools/query_internal_docs.go:40` -- `InferOptionableTool` 失败时 `log.Fatal(err)`
- `internal/ai/tools/query_metrics_alerts.go:158` -- `InferOptionableTool` 失败时 `log.Fatal(err)`

### 2. mysql_crud 工具使用 stdin 阻塞等待确认

`internal/ai/tools/mysql_crud.go:38-44` 中使用 `bufio.NewScanner(os.Stdin)` 读取用户确认。HTTP 服务模式下 stdin 不可用，会导致工具调用永久挂起。

```go
scanner := bufio.NewScanner(os.Stdin)
fmt.Print("\nConfirm SQL execution (y/n): ", input.SQL)
scanner.Scan()
```

## P1 -- 重要缺陷

### 4. query_metrics_alerts.go 存在死代码

`internal/ai/tools/query_metrics_alerts.go:54-57` 中函数在 `return` 之后还有大量不可达代码。代码中已标注 `// FIXME: unreachable`。

```go
func queryPrometheusAlerts() (PrometheusAlertsResult, error) {
    return PrometheusAlertsResult{}, nil  // 第 56 行提前返回
    // FIXME: unreachable               // 第 57 行以下全部不可达
    baseURL := "http://127.0.0.1:9090"
    ...
}
```

### 5. tools_node.go 中变量名遮蔽 config 包

`internal/ai/agent/chat_pipeline/tools_node.go:16` 中使用 `config` 作为局部变量名，遮蔽了同名的 `config` 包导入，容易引发维护风险。

```go
func newReactAgentLambda(ctx context.Context, cfg *config.Config) (*compose.Lambda, error) {
    config := &react.AgentConfig{  // 局部变量名 config 遮蔽了包名
```

## P2 -- 一般缺陷

### 10. 无测试文件

项目中不存在任何 `*_test.go` 文件，测试完全依赖 `cmd/` 下的 5 个 CLI 命令手动验证。

### 11. 无结构化日志

日志散落在 `fmt.Printf`、`log.Printf`、`log_callback` 三种方式中，没有统一的日志级别、格式或输出目标。

### 12. 对话记忆无持久化

`internal/utility/mem/mem.go` 中的 `ConversationMemory` 仅存储在内存中，服务重启后所有对话历史丢失。

### 13. Prometheus 工具已禁用

`query_metrics_alerts.go:56` 中 `queryPrometheusAlerts()` 函数直接返回空结果，Prometheus 告警查询功能完全不可用。AI agent 调用此工具只会得到空数据。

### 14. DuckDuckGo 搜索工具未接入

`internal/ai/agent/chat_pipeline/search_tool.go` 中定义了 `newSearchTool()` 函数，但未在任何 pipeline 中调用，属于死代码。

## P3 -- 缺失的基础设施

### 15. 无 README.md

项目根目录缺少 README 文件，新开发者无法快速了解项目用途和启动方式。

### 16. 无 Makefile

缺少统一的构建入口，当前只能通过 `go run` 或 `go build` 手动操作。

### 17. 无 Dockerfile 和 docker-compose.yml

旧项目中有完整的 Milvus 集群 docker-compose.yml (etcd + minio + milvus + attu)，新项目尚未迁移。
