// Package indexer provides a Milvus-backed document indexer for storing knowledge base documents.
// Documents are split, embedded, and stored in the Milvus collection with their vector representations.
package indexer

import (
	"context"

	"github.com/cloudwego/eino-ext/components/indexer/milvus"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/embedder"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/consts"
	swifty_milvus "github.com/hangtiancheng/swifty.go/swifty_agent/internal/utility/milvus"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// indexFields defines the schema fields for the Milvus collection used by the indexer.
var indexFields = []*entity.Field{
	{
		Name:     "id",
		DataType: entity.FieldTypeVarChar,
		TypeParams: map[string]string{
			"max_length": "255",
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

// NewMilvusIndexer creates an indexer that stores document chunks with their
// vector embeddings into the Milvus knowledge base collection.
func NewMilvusIndexer(ctx context.Context, cfg *config.Config) (*milvus.Indexer, error) {
	cli, err := swifty_milvus.NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	eb, err := embedder.New(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return milvus.NewIndexer(ctx, &milvus.IndexerConfig{
		Client:     cli,
		Collection: consts.MilvusCollectionName,
		Fields:     indexFields,
		Embedding:  eb,
	})
}
