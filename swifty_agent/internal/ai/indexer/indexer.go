// Package indexer provides a Redis-backed document indexer for the knowledge base.
// Documents are split, embedded, and stored as Redis hashes with their vector embeddings.
package indexer

import (
	"context"

	eino_redis "github.com/cloudwego/eino-ext/components/indexer/redis"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/embedder"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/consts"
	swifty_redis "github.com/hangtiancheng/swifty.go/swifty_agent/internal/utility/redis"
)

// NewRedisIndexer creates an indexer that stores document chunks with their vector
// embeddings into Redis using the Eino Redis indexer component (default field mapping:
// content -> "content", embedding -> "vector_content", metadata passed through as-is).
func NewRedisIndexer(ctx context.Context, cfg *config.Config) (indexer.Indexer, error) {
	client, err := swifty_redis.NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	eb, err := embedder.New(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return eino_redis.NewIndexer(ctx, &eino_redis.IndexerConfig{
		Client:    client,
		KeyPrefix: consts.RedisKeyPrefix,
		Embedding: eb,
		BatchSize: 10,
	})
}
