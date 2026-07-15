package knowledge_index_pipeline

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/loader"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/consts"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/utility/log_callback"
	swifty_redis "github.com/hangtiancheng/swifty.go/swifty_agent/internal/utility/redis"
	"github.com/redis/go-redis/v9"
)

// IndexFile indexes a single document file into the Redis knowledge base.
// Before indexing, it removes any existing documents that share the same
// "_source" metadata value, ensuring re-indexing a file does not produce
// duplicate entries.
//
// This function is shared between the HTTP file-upload handler and the
// batch knowledge-indexing CLI to avoid logic duplication.
func IndexFile(ctx context.Context, cfg *config.Config, path string) error {
	runner, err := BuildKnowledgeIndexing(ctx, cfg)
	if err != nil {
		return err
	}

	// Load the document to obtain its metadata for deduplication.
	ldr, err := loader.NewFileLoader(ctx)
	if err != nil {
		return err
	}
	docs, err := ldr.Load(ctx, document.Source{URI: path})
	if err != nil {
		return err
	}

	// Connect to Redis and remove existing documents with the same source.
	client, err := swifty_redis.NewClient(ctx, cfg)
	if err != nil {
		return err
	}

	source, _ := docs[0].MetaData[consts.RedisSourceField].(string)
	if source != "" {
		// Use basename to match the _source value written by the indexer
		// (see indexer.documentToHashes) and the Next.js loader behavior.
		source = filepath.Base(source)
	}
	if err := deleteBySource(ctx, client, source); err != nil {
		fmt.Printf("[warn] delete existing data failed: %v\n", err)
	}

	// Index the new document through the pipeline.
	ids, err := runner.Invoke(ctx, document.Source{URI: path}, compose.WithCallbacks(log_callback.NewHandler(nil)))
	if err != nil {
		return fmt.Errorf("invoke index graph: %w", err)
	}
	fmt.Printf("[done] indexing file: %s, len of parts: %d\n", path, len(ids))
	return nil
}

// deleteBySource removes all knowledge-base entries whose _source matches the
// given source string. It acquires a per-source SETNX lock (30s TTL as a
// deadlock safety net) and deletes in batches of 1000 to handle sources with
// many chunks safely. Mirrors the Next.js lib/redis/indexer.ts deleteBySource.
func deleteBySource(ctx context.Context, client *redis.Client, source string) error {
	escaped := escapeTag(source)
	lockKey := consts.RedisKeyPrefix + "lock:delete:" + escaped

	// Acquire lock (SET NX EX 30).
	acquired, err := client.Do(ctx, "SET", lockKey, "1", "NX", "EX", "30").Result()
	if err != nil {
		return fmt.Errorf("acquire delete lock: %w", err)
	}
	if acquired == nil {
		return fmt.Errorf("cannot acquire lock for source %q — another deletion is in progress", source)
	}
	defer client.Del(ctx, lockKey)

	query := fmt.Sprintf("@%s:{%s}", consts.RedisSourceField, escaped)
	const batchSize = 1000
	for {
		res, err := client.Do(ctx, "FT.SEARCH", consts.RedisIndexName, query,
			"NOCONTENT", "LIMIT", "0", batchSize).Slice()
		if err != nil {
			return fmt.Errorf("search existing docs: %w", err)
		}
		if len(res) <= 1 {
			return nil // res[0] is the total count; no keys returned.
		}
		keys := make([]string, 0, len(res)-1)
		for _, v := range res[1:] {
			if k, ok := v.(string); ok {
				keys = append(keys, k)
			}
		}
		if len(keys) == 0 {
			return nil
		}
		n, err := client.Del(ctx, keys...).Result()
		if err != nil {
			return fmt.Errorf("delete docs: %w", err)
		}
		fmt.Printf("[info] deleted %d existing records with _source: %s\n", n, source)
		if len(keys) < batchSize {
			return nil // last batch
		}
	}
}

// escapeTag escapes RediSearch TAG special characters (DIALECT 2) so that file
// paths (containing '/', '.', '-', spaces, etc.) can be matched exactly.
func escapeTag(s string) string {
	var b strings.Builder
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
