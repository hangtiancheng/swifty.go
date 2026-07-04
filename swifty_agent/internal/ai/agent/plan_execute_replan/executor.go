package plan_execute_replan

import (
	"context"

	"github.com/cloudwego/eino/adk"
	plan_execute "github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/compose"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/models"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/tools"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
)

// NewExecutor creates the execution agent that carries out each step of the plan
// using the available tools. It uses the quick model for fast tool execution.
func NewExecutor(ctx context.Context, cfg *config.Config) (adk.Agent, error) {
	// Register MCP tools for log querying.
	mcpTools, err := tools.GetLogMcpTool(ctx, cfg.MCP_URL)
	if err != nil {
		return nil, err
	}

	toolList := mcpTools
	toolList = append(toolList, tools.NewPrometheusAlertsQueryTool())
	toolList = append(toolList, tools.NewQueryInternalDocsTool(cfg))
	toolList = append(toolList, tools.NewGetCurrentTimeTool())

	execModel, err := models.NewQuickChatModel(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return plan_execute.NewExecutor(ctx, &plan_execute.ExecutorConfig{
		Model: execModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: toolList,
			},
		},
		MaxIterations: 999999,
	})
}
