package tools

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// GetCurrentTimeInput is empty as no input parameters are needed.
type GetCurrentTimeInput struct{}

// GetCurrentTimeOutput contains the current time in multiple formats.
type GetCurrentTimeOutput struct {
	Success      bool   `json:"success" jsonschema:"description=Whether the time retrieval was successful"`
	Seconds      int64  `json:"seconds" jsonschema:"description=Current Unix timestamp in seconds"`
	Milliseconds int64  `json:"milliseconds" jsonschema:"description=Current Unix timestamp in milliseconds"`
	Microseconds int64  `json:"microseconds" jsonschema:"description=Current Unix timestamp in microseconds"`
	Timestamp    string `json:"timestamp" jsonschema:"description=Human-readable timestamp in YYYY-MM-DD HH:MM:SS.microseconds format"`
	Message      string `json:"message" jsonschema:"description=Status message"`
}

// NewGetCurrentTimeTool creates a tool that returns the current system time
// in multiple formats (Unix seconds, milliseconds, microseconds, and human-readable).
func NewGetCurrentTimeTool() tool.InvokableTool {
	t, err := utils.InferOptionableTool(
		"get_current_time",
		"Get current system time in multiple formats. Returns Unix timestamp in seconds, milliseconds, and microseconds. Use when you need current time for logging, timing operations, or timestamping events.",
		func(ctx context.Context, input *GetCurrentTimeInput, opts ...tool.Option) (string, error) {
			now := time.Now()
			timestamp := now.Format("2006-01-02 15:04:05.000000")
			log.Printf("Getting current time: %s", timestamp)

			out := GetCurrentTimeOutput{
				Success:      true,
				Seconds:      now.Unix(),
				Milliseconds: now.UnixMilli(),
				Microseconds: now.UnixMicro(),
				Timestamp:    timestamp,
				Message:      "Current time retrieved successfully",
			}
			b, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				log.Printf("Error marshaling result: %v", err)
				return "", err
			}
			return string(b), nil
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	return t
}
