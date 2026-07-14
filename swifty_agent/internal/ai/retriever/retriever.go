// Package retriever provides a Redis-backed vector retriever for RAG (Retrieval-Augmented Generation).
// It queries the RediSearch index for documents similar to the input query using embedding-based KNN search.
package retriever

import (
	"context"

	eino_redis "github.com/cloudwego/eino-ext/components/retriever/redis"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/embedder"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/consts"
	swifty_redis "github.com/hangtiancheng/swifty.go/swifty_agent/internal/utility/redis"
)

// NewRedisRetriever creates a retriever that searches the Redis knowledge base using
// KNN vector similarity search. It returns the top-1 most relevant document for each query.
func NewRedisRetriever(ctx context.Context, cfg *config.Config) (retriever.Retriever, error) {
	client, err := swifty_redis.NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	eb, err := embedder.New(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return eino_redis.NewRetriever(ctx, &eino_redis.RetrieverConfig{
		Client:       client,
		Index:        consts.RedisIndexName,
		VectorField:  consts.RedisVectorField,            // "vector_content"
		ReturnFields: []string{consts.RedisContentField}, // content only, avoids missing-field errors
		TopK:         1,
		Embedding:    eb,
	})
}
