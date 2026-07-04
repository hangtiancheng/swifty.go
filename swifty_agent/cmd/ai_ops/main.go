// Command ai_ops runs the plan-execute-replan agent standalone to perform
// intelligent alert operations analysis. It executes a predefined prompt that
// queries Prometheus alerts, retrieves internal documentation, and generates
// a structured alert operations report.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/agent/plan_execute_replan"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
)

// aiOpsQuery is the predefined prompt for the AI operations analysis agent.
// It instructs the agent to query alerts, find processing guides, and generate a report.
const aiOpsQuery = `
"1. You are an intelligent alert analysis assistant. First call the tool query_prometheus_alerts to get all active alerts."
"2. For each alert, call the tool query_internal_docs to find the corresponding processing guide."
"3. Strictly follow the internal documentation for analysis; do not use any information outside the docs."
"4. For time-related parameters, first call get_current_time to get the current time, then use it in combination with tool requirements."
"5. For log queries, use the log tool with required region and log topic parameters."
"6. Summarize and analyze the information found for each alert, then generate an alert operations report in this format:
Alert Analysis Report
---
# Alert Processing Details
## Active Alert List
## Root Cause Analysis N (for the Nth alert)
## Processing Plan N (for the Nth alert)
## Conclusion
`

func main() {
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx := context.Background()
	resp, detail, err := plan_execute_replan.BuildPlanAgent(ctx, cfg, aiOpsQuery)
	if err != nil {
		log.Fatalf("build plan agent: %v", err)
	}

	fmt.Println("----- Final Response -----")
	fmt.Println(resp)
	fmt.Println("----- Execution Detail -----")
	for i, d := range detail {
		fmt.Printf("[%d] %s\n", i, d)
	}
}
