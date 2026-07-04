// Package consts defines shared constants used across the swifty_agent application.
package consts

const (
	// MilvusDBName is the name of the Milvus database used for vector storage.
	MilvusDBName = "agent"

	// MilvusCollectionName is the name of the Milvus collection that stores
	// knowledge base documents and their embeddings.
	MilvusCollectionName = "biz"
)
