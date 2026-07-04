package chat_pipeline

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/tools"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
)

// newReactAgentLambda creates a ReAct (Reasoning + Acting) agent that can use tools
// to answer questions. The agent iteratively reasons about the problem, selects tools,
// observes results, and generates a final response.
func newReactAgentLambda(ctx context.Context, cfg *config.Config) (*compose.Lambda, error) {
	config := &react.AgentConfig{
		MaxStep:            25,
		ToolReturnDirectly: map[string]struct{}{},
	}

	chatModel, err := newChatModel(ctx, cfg)
	if err != nil {
		return nil, err
	}
	config.ToolCallingModel = chatModel

	// Register MCP tools (log querying).
	mcpTools, err := tools.GetLogMcpTool(ctx, cfg.MCP_URL)
	if err != nil {
		return nil, err
	}
	config.ToolsConfig.Tools = mcpTools

	// Register additional tools.
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, tools.NewPrometheusAlertsQueryTool())
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, tools.NewMysqlCrudTool())
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, tools.NewGetCurrentTimeTool())
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, tools.NewQueryInternalDocsTool(cfg))

	agent, err := react.NewAgent(ctx, config)
	if err != nil {
		return nil, err
	}

	return compose.AnyLambda(agent.Generate, agent.Stream, nil, nil)
}
