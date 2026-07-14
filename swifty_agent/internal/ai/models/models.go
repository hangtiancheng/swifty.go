// Package models provides factory functions for creating LLM chat model instances.
// It supports both OpenAI-compatible API endpoints and Anthropic Claude for
// different reasoning use cases, selected via the model_provider configuration.
package models

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
)

// NewThinkChatModel creates a chat model instance configured for deep reasoning tasks.
// This model is used by the planner and replanner in the plan-execute-replan pipeline.
func NewThinkChatModel(ctx context.Context, cfg *config.Config) (model.ToolCallingChatModel, error) {
	return newChatModel(ctx, cfg, cfg.ThinkChatModel)
}

// NewQuickChatModel creates a chat model instance configured for fast responses.
// This model is used for standard chat interactions and tool execution steps.
func NewQuickChatModel(ctx context.Context, cfg *config.Config) (model.ToolCallingChatModel, error) {
	return newChatModel(ctx, cfg, cfg.QuickChatModel)
}

// newChatModel builds a chat model from the given model settings, selecting the
// underlying implementation based on cfg.ModelProvider. It defaults to the
// OpenAI-compatible implementation unless "anthropic" is explicitly configured.
func newChatModel(ctx context.Context, cfg *config.Config, mc config.ChatModelConfig) (model.ToolCallingChatModel, error) {
	if cfg.ModelProvider == config.ModelProviderAnthropic {
		// Claude's BaseURL is optional; nil falls back to the default Anthropic endpoint.
		var baseURL *string
		if mc.BaseURL != "" {
			baseURL = &mc.BaseURL
		}
		return claude.NewChatModel(ctx, &claude.Config{
			APIKey:    mc.APIKey,
			BaseURL:   baseURL,
			Model:     mc.Model,
			MaxTokens: mc.MaxTokens,
		})
	}
	return openai.NewChatModel(ctx, &openai.ChatModelConfig{
		Model:   mc.Model,
		APIKey:  mc.APIKey,
		BaseURL: mc.BaseURL,
	})
}
