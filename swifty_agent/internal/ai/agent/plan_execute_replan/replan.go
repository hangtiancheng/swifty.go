package plan_execute_replan

import (
	"context"

	"github.com/cloudwego/eino/adk"
	plan_execute "github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/models"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
)

// NewRePlanAgent creates the replanning agent that adjusts the execution plan
// based on the results of completed steps. It uses the think model for
// reasoning about whether the plan needs modification.
func NewRePlanAgent(ctx context.Context, cfg *config.Config) (adk.Agent, error) {
	model, err := models.NewThinkChatModel(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return plan_execute.NewReplanner(ctx, &plan_execute.ReplannerConfig{
		ChatModel: model,
	})
}
