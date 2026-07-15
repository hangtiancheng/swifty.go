// Package plan_execute_replan implements a Plan-Execute-Replan agent pattern.
// The planner creates an execution plan, the executor carries out each step
// using available tools, and the replanner adjusts the plan based on results.
// This pattern is well-suited for complex multi-step AI operations tasks.
package plan_execute_replan

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-examples/adk/common/prints"
	"github.com/cloudwego/eino/adk"
	plan_execute "github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/utility/logger"
)

// BuildPlanAgent creates and runs a plan-execute-replan agent pipeline.
// It returns the final response content, a list of detail messages from each step,
// and any error encountered during execution.
func BuildPlanAgent(ctx context.Context, cfg *config.Config, query string) (string, []string, error) {
	planAgent, err := NewPlanner(ctx, cfg)
	if err != nil {
		return "", nil, err
	}
	executeAgent, err := NewExecutor(ctx, cfg)
	if err != nil {
		return "", nil, err
	}
	replanAgent, err := NewRePlanAgent(ctx, cfg)
	if err != nil {
		return "", nil, err
	}

	planExecuteAgent, err := plan_execute.New(ctx, &plan_execute.Config{
		Planner:       planAgent,
		Executor:      executeAgent,
		Replanner:     replanAgent,
		MaxIterations: 20,
	})
	if err != nil {
		return "", nil, fmt.Errorf("build PlanExecuteAgent: %w", err)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: planExecuteAgent})
	iter := runner.Query(ctx, query)

	var lastMessage adk.Message
	var detail []string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		logger.L().Info("event")
		prints.Event(event)
		if event.Output != nil {
			lastMessage, _, err = adk.GetMessage(event)
			if err == nil {
				detail = append(detail, lastMessage.String())
			}
		}
	}

	if lastMessage == nil {
		return "", nil, fmt.Errorf("no response generated")
	}
	return lastMessage.Content, detail, nil
}
