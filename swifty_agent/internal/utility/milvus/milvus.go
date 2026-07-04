// Package milvus provides a client factory for connecting to the Milvus vector database.
// It handles database and collection creation, including schema setup and indexing.
package milvus

import (
	"context"
	"fmt"

	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/consts"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// collectionFields defines the schema for the knowledge base collection.
var collectionFields = []*entity.Field{
	{
		Name:     "id",
		DataType: entity.FieldTypeVarChar,
		TypeParams: map[string]string{
			"max_length": "256",
		},
		PrimaryKey: true,
	},
	{
		Name:     "vector",
		DataType: entity.FieldTypeBinaryVector,
		TypeParams: map[string]string{
			"dim": "65536",
		},
	},
	{
		Name:     "content",
		DataType: entity.FieldTypeVarChar,
		TypeParams: map[string]string{
			"max_length": "8192",
		},
	},
	{
		Name:     "metadata",
		DataType: entity.FieldTypeJSON,
	},
}

// NewClient creates a Milvus client connected to the configured address.
// It ensures the target database and collection exist, creating them if necessary.
func NewClient(ctx context.Context, cfg *config.Config) (client.Client, error) {
	// Connect to default database first to check/create the agent database.
	defaultClient, err := client.NewClient(ctx, client.Config{
		Address: cfg.Milvus.Address,
		DBName:  "default",
	})
	if err != nil {
		return nil, fmt.Errorf("connect to default database: %w", err)
	}

	databases, err := defaultClient.ListDatabases(ctx)
	if err != nil {
		return nil, fmt.Errorf("list databases: %w", err)
	}

	dbExists := false
	for _, db := range databases {
		if db.Name == cfg.Milvus.DBName {
			dbExists = true
			break
		}
	}
	if !dbExists {
		if err := defaultClient.CreateDatabase(ctx, cfg.Milvus.DBName); err != nil {
			return nil, fmt.Errorf("create database %s: %w", cfg.Milvus.DBName, err)
		}
	}
	defaultClient.Close()

	// Connect to the agent database.
	agentClient, err := client.NewClient(ctx, client.Config{
		Address: cfg.Milvus.Address,
		DBName:  cfg.Milvus.DBName,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to database %s: %w", cfg.Milvus.DBName, err)
	}

	// Ensure the knowledge base collection exists.
	collections, err := agentClient.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}

	collectionExists := false
	for _, c := range collections {
		if c.Name == consts.MilvusCollectionName {
			collectionExists = true
			break
		}
	}

	if !collectionExists {
		schema := &entity.Schema{
			CollectionName: consts.MilvusCollectionName,
			Description:    "Business knowledge collection",
			Fields:         collectionFields,
		}
		if err := agentClient.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
			return nil, fmt.Errorf("create collection: %w", err)
		}

		// Create AUTOINDEX indexes for all fields.
		idIndex, _ := entity.NewIndexAUTOINDEX(entity.L2)
		_ = agentClient.CreateIndex(ctx, consts.MilvusCollectionName, "id", idIndex, false)

		contentIndex, _ := entity.NewIndexAUTOINDEX(entity.L2)
		_ = agentClient.CreateIndex(ctx, consts.MilvusCollectionName, "content", contentIndex, false)

		vectorIndex, _ := entity.NewIndexAUTOINDEX(entity.HAMMING)
		_ = agentClient.CreateIndex(ctx, consts.MilvusCollectionName, "vector", vectorIndex, false)
	}

	return agentClient, nil
}
