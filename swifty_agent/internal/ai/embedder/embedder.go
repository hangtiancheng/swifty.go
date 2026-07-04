// Package embedder provides factory functions for creating text embedding models.
// The embeddings are used to vectorize documents for storage and retrieval in Milvus.
package embedder

import (
	"context"

	"github.com/cloudwego/eino-ext/components/embedding/dashscope"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
)

// New creates a text embedding model using the configured DashScope-compatible endpoint.
// The embedding dimensions are taken from the application configuration.
func New(ctx context.Context, cfg *config.Config) (embedding.Embedder, error) {
	dims := cfg.EmbeddingModel.Dimensions
	return dashscope.NewEmbedder(ctx, &dashscope.EmbeddingConfig{
		Model:      cfg.EmbeddingModel.Model,
		APIKey:     cfg.EmbeddingModel.APIKey,
		Dimensions: &dims,
	})
}
