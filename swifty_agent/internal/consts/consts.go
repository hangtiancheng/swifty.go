// Package consts defines shared constants used across the swifty_agent application.
package consts

const (
	// RedisKeyPrefix is the key prefix for document hashes in Redis.
	RedisKeyPrefix = "doc:"

	// RedisIndexName is the RediSearch index name used for vector search.
	RedisIndexName = "idx:biz"

	// RedisContentField stores the document content (indexed as TEXT).
	RedisContentField = "content"

	// RedisVectorField stores the content embedding (indexed as VECTOR).
	// It must match the eino redis default field name "vector_content".
	RedisVectorField = "vector_content"

	// RedisSourceField stores the document source path (indexed as TAG, used for dedup).
	RedisSourceField = "_source"
)
