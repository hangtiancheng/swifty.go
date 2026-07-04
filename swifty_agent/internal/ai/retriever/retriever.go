// Package retriever provides a Milvus-backed vector retriever for RAG (Retrieval-Augmented Generation).
// It queries the Milvus collection for documents similar to the input query using embedding-based search.
package retriever

import (
	"context"

	"github.com/cloudwego/eino-ext/components/retriever/milvus"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/embedder"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/consts"
	swifty_milvus "github.com/hangtiancheng/swifty.go/swifty_agent/internal/utility/milvus"
)

// NewMilvusRetriever creates a retriever that searches the Milvus knowledge base collection
// using vector similarity. It returns the top-1 most relevant document for each query.
func NewMilvusRetriever(ctx context.Context, cfg *config.Config) (retriever.Retriever, error) {
	cli, err := swifty_milvus.NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	eb, err := embedder.New(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return milvus.NewRetriever(ctx, &milvus.RetrieverConfig{
		Client:      cli,
		Collection:  consts.MilvusCollectionName,
		VectorField: "vector",
		OutputFields: []string{
			"id",
			"content",
			"metadata",
		},
		TopK:      1,
		Embedding: eb,
	})
}
