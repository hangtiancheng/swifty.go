package chat_pipeline

import (
	"context"

	"github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2"
	"github.com/cloudwego/eino/components/tool"
)

// newSearchTool creates a DuckDuckGo web search tool for the agent.
// Currently unused but available for enabling web search capabilities.
func newSearchTool(ctx context.Context) (tool.BaseTool, error) {
	return duckduckgo.NewTextSearchTool(ctx, &duckduckgo.Config{})
}
