package plan_execute_replan

import (
	"context"

	"github.com/cloudwego/eino/adk"
	plan_execute "github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/models"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
)

// NewPlanner creates the planning agent that decomposes a complex query
// into a sequence of executable steps. It uses the think model for deep reasoning.
func NewPlanner(ctx context.Context, cfg *config.Config) (adk.Agent, error) {
	planModel, err := models.NewThinkChatModel(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return plan_execute.NewPlanner(ctx, &plan_execute.PlannerConfig{
		ToolCallingChatModel: planModel,
	})
}
