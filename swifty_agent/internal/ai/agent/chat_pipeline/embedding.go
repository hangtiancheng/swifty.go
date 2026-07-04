package chat_pipeline

import (
	"context"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/embedder"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
)

// newEmbedding creates the text embedding model used for vector similarity search.
func newEmbedding(ctx context.Context, cfg *config.Config) (embedding.Embedder, error) {
	return embedder.New(ctx, cfg)
}
