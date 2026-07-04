// Package tools provides AI agent tool implementations for the Swifty Chatbot.
// Each tool wraps a specific capability (MCP integration, Prometheus alerts,
// MySQL queries, documentation search, time retrieval) as an Eino tool
// that can be invoked by the AI agent during reasoning.
package tools

import (
	"context"

	eino_mcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// GetLogMcpTool connects to an MCP (Model Context Protocol) server via SSE
// and returns the available tools from that server.
//
// The MCP server provides log querying capabilities. See:
//   - https://www.cloudwego.io/zh/docs/eino/ecosystem_integration/tool/tool_mcp/
//   - https://mcp-go.dev/clients
func GetLogMcpTool(ctx context.Context, mcpURL string) ([]tool.BaseTool, error) {
	cli, err := client.NewSSEMCPClient(mcpURL)
	if err != nil {
		return nil, err
	}

	if err := cli.Start(ctx); err != nil {
		return nil, err
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "swifty-agent-client",
		Version: "1.0.0",
	}

	if _, err := cli.Initialize(ctx, initReq); err != nil {
		return nil, err
	}

	return eino_mcp.GetTools(ctx, &eino_mcp.Config{Cli: cli})
}
