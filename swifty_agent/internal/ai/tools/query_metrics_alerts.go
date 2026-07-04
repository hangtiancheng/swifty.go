package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// PrometheusAlert represents a single alert from the Prometheus alerts API.
type PrometheusAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	State       string            `json:"state"`
	ActiveAt    string            `json:"activeAt"`
	Value       string            `json:"value"`
}

// PrometheusAlertsResult holds the raw response from Prometheus /api/v1/alerts.
type PrometheusAlertsResult struct {
	Status string `json:"status"`
	Data   struct {
		Alerts []PrometheusAlert `json:"alerts"`
	} `json:"data"`
	Error     string `json:"error,omitempty"`
	ErrorType string `json:"errorType,omitempty"`
}

// SimplifiedAlert is a human-friendly representation of a Prometheus alert.
type SimplifiedAlert struct {
	AlertName   string `json:"alert_name" jsonschema:"description=Alert name from Prometheus labels.alertname"`
	Description string `json:"description" jsonschema:"description=Alert description from annotations.description"`
	State       string `json:"state" jsonschema:"description=Alert state, typically firing or pending"`
	ActiveAt    string `json:"active_at" jsonschema:"description=Activation timestamp in RFC3339 format"`
	Duration    string `json:"duration" jsonschema:"description=Time since activation, e.g. 2h30m15s"`
}

// PrometheusAlertsOutput is the tool's output structure.
type PrometheusAlertsOutput struct {
	Success bool              `json:"success"`
	Alerts  []SimplifiedAlert `json:"alerts,omitempty"`
	Message string            `json:"message,omitempty"`
	Error   string            `json:"error,omitempty"`
}

// queryPrometheusAlerts queries the Prometheus alerts API.
// Currently returns empty results; enable by starting a Prometheus instance.
func queryPrometheusAlerts() (PrometheusAlertsResult, error) {
	// Toggle: start a Prometheus container before using this.
	return PrometheusAlertsResult{}, nil
	// FIXME: unreachable
	baseURL := "http://127.0.0.1:9090"
	apiURL := fmt.Sprintf("%s/api/v1/alerts", baseURL)

	log.Printf("Querying Prometheus alerts: %s", apiURL)

	httpClient := &http.Client{Timeout: 10 * time.Second}
	var result PrometheusAlertsResult

	resp, err := httpClient.Get(apiURL)
	if err != nil {
		return result, fmt.Errorf("query Prometheus alerts: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("read response: %w", err)
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("parse response: %w", err)
	}
	return result, nil
}

// calculateDuration computes the elapsed time from activeAtStr to now.
func calculateDuration(activeAtStr string) string {
	activeAt, err := time.Parse(time.RFC3339Nano, activeAtStr)
	if err != nil {
		return "unknown"
	}

	d := time.Since(activeAt)
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	switch {
	case hours > 0:
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	case minutes > 0:
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

// NewPrometheusAlertsQueryTool creates a tool that queries active Prometheus alerts.
// For alerts with the same alertname, only the first occurrence is returned.
func NewPrometheusAlertsQueryTool() tool.InvokableTool {
	t, err := utils.InferOptionableTool(
		"query_prometheus_alerts",
		"Query active alerts from Prometheus alerting system. Returns all currently active/firing alerts including labels, annotations, state, and values. Use when checking what alerts are firing, investigating alert conditions, or monitoring alert status.",
		func(ctx context.Context, input *struct{}, opts ...tool.Option) (string, error) {
			log.Printf("Querying Prometheus active alerts")

			result, err := queryPrometheusAlerts()
			if err != nil {
				out := PrometheusAlertsOutput{
					Success: false,
					Error:   err.Error(),
					Message: "Failed to query Prometheus alerts",
				}
				b, _ := json.MarshalIndent(out, "", "  ")
				return string(b), err
			}

			// Deduplicate by alertname, keeping only the first occurrence.
			seen := make(map[string]bool)
			var simplified []SimplifiedAlert
			for _, alert := range result.Data.Alerts {
				name := alert.Labels["alertname"]
				if seen[name] {
					continue
				}
				seen[name] = true
				simplified = append(simplified, SimplifiedAlert{
					AlertName:   name,
					Description: alert.Annotations["description"],
					State:       alert.State,
					ActiveAt:    alert.ActiveAt,
					Duration:    calculateDuration(alert.ActiveAt),
				})
			}

			out := PrometheusAlertsOutput{
				Success: true,
				Alerts:  simplified,
				Message: fmt.Sprintf("Successfully retrieved %d active alerts", len(simplified)),
			}
			b, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				log.Printf("Error marshaling alerts result: %v", err)
				return "", err
			}
			log.Printf("Prometheus alerts query completed: %d alerts found", len(simplified))
			return string(b), nil
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	return t
}
