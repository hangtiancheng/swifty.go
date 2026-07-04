// Package models provides factory functions for creating LLM chat model instances.
// It supports OpenAI-compatible API endpoints for different reasoning use cases.
package models

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
)

// NewThinkChatModel creates a chat model instance configured for deep reasoning tasks.
// This model is used by the planner and replanner in the plan-execute-replan pipeline.
func NewThinkChatModel(ctx context.Context, cfg *config.Config) (model.ToolCallingChatModel, error) {
	return openai.NewChatModel(ctx, &openai.ChatModelConfig{
		Model:   cfg.ThinkChatModel.Model,
		APIKey:  cfg.ThinkChatModel.APIKey,
		BaseURL: cfg.ThinkChatModel.BaseURL,
	})
}

// NewQuickChatModel creates a chat model instance configured for fast responses.
// This model is used for standard chat interactions and tool execution steps.
func NewQuickChatModel(ctx context.Context, cfg *config.Config) (model.ToolCallingChatModel, error) {
	return openai.NewChatModel(ctx, &openai.ChatModelConfig{
		Model:   cfg.QuickChatModel.Model,
		APIKey:  cfg.QuickChatModel.APIKey,
		BaseURL: cfg.QuickChatModel.BaseURL,
	})
}
