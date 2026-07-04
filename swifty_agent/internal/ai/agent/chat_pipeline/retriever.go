package chat_pipeline

import (
	"context"

	"github.com/cloudwego/eino/components/retriever"
	swifty_retriever "github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/retriever"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
)

// newRetriever creates a Milvus-backed vector retriever for the chat pipeline.
// It searches the knowledge base for documents relevant to the user's query.
func newRetriever(ctx context.Context, cfg *config.Config) (retriever.Retriever, error) {
	return swifty_retriever.NewMilvusRetriever(ctx, cfg)
}
