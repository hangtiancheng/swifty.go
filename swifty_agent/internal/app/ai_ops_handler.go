package app

import (
	"net/http"

	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/agent/plan_execute_replan"
	"github.com/hangtiancheng/swifty.go/swifty_http"
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

// handleAIOps processes AI operations analysis requests.
// It runs the plan-execute-replan agent with a predefined alert analysis prompt
// and returns both the final report and detailed execution steps.
func (a *App) handleAIOps(ctx *swifty_http.Context, next func()) {
	resp, detail, err := plan_execute_replan.BuildPlanAgent(ctx.Request.Context(), a.cfg, aiOpsQuery)
	if err != nil {
		ctx.Throw(http.StatusInternalServerError, err.Error())
		return
	}
	if resp == "" {
		ctx.Throw(http.StatusInternalServerError, "internal error: empty response")
		return
	}

	ctx.Status = http.StatusOK
	ctx.JSON(swifty_http.H{
		"message": "OK",
		"data": swifty_http.H{
			"result": resp,
			"detail": detail,
		},
	})
}
