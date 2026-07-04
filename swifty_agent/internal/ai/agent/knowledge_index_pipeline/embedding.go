package knowledge_index_pipeline

import (
	"context"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/embedder"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
)

// newEmbedding creates the text embedding model for vectorize document chunks.
func newEmbedding(ctx context.Context, cfg *config.Config) (embedding.Embedder, error) {
	return embedder.New(ctx, cfg)
}
