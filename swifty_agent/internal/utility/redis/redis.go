// Package redis provides a client factory for connecting to Redis Stack (RediSearch).
// It configures the client for vector search (FT.SEARCH) and ensures the vector index exists.
package redis

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/consts"
	"github.com/redis/go-redis/v9"
)

// NewClient creates a Redis client configured for vector search and ensures the
// RediSearch vector index exists (creating it on first connection if necessary).
func NewClient(ctx context.Context, cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		Protocol: 2, // Required: FT.SEARCH needs RESP2, otherwise results are raw.
	})
	client.Options().UnstableResp3 = true // Required for vector search.

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connect to Redis: %w", err)
	}
	if err := ensureIndex(ctx, client, cfg); err != nil {
		return nil, fmt.Errorf("ensure Redis index: %w", err)
	}
	return client, nil
}

// ensureIndex creates the HNSW/COSINE vector index if it does not already exist.
func ensureIndex(ctx context.Context, client *redis.Client, cfg *config.Config) error {
	if err := client.Do(ctx, "FT.INFO", consts.RedisIndexName).Err(); err == nil {
		return nil // Index already exists.
	}

	dim := cfg.EmbeddingModel.Dimensions
	if dim == 0 {
		dim = 2048
	}

	return client.Do(ctx, "FT.CREATE", consts.RedisIndexName,
		"ON", "HASH",
		"PREFIX", "1", consts.RedisKeyPrefix,
		"SCHEMA",
		consts.RedisContentField, "TEXT",
		consts.RedisSourceField, "TAG",
		consts.RedisVectorField, "VECTOR", "HNSW", "6",
		"TYPE", "FLOAT32",
		"DIM", strconv.Itoa(dim),
		"DISTANCE_METRIC", "COSINE",
	).Err()
}
