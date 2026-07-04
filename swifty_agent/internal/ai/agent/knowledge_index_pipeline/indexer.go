package knowledge_index_pipeline

import (
	"context"

	"github.com/cloudwego/eino/components/indexer"
	swifty_indexer "github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/indexer"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
)

// newIndexer creates a Milvus-backed indexer that stores document chunks
// with their vector embeddings into the knowledge base collection.
func newIndexer(ctx context.Context, cfg *config.Config) (indexer.Indexer, error) {
	return swifty_indexer.NewMilvusIndexer(ctx, cfg)
}
