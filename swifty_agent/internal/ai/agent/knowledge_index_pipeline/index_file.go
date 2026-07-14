package knowledge_index_pipeline

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/loader"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/consts"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/utility/log_callback"
	swifty_redis "github.com/hangtiancheng/swifty.go/swifty_agent/internal/utility/redis"
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
	query := fmt.Sprintf("@%s:{%s}", consts.RedisSourceField, escapeTag(source))
	res, err := client.Do(ctx, "FT.SEARCH", consts.RedisIndexName, query,
		"NOCONTENT", "LIMIT", "0", "10000").Slice()
	if err == nil && len(res) > 1 {
		keys := make([]string, 0, len(res)-1)
		for _, v := range res[1:] { // res[0] is the total count.
			if k, ok := v.(string); ok {
				keys = append(keys, k)
			}
		}
		if len(keys) > 0 {
			if n, err := client.Del(ctx, keys...).Result(); err != nil {
				fmt.Printf("[warn] delete existing data failed: %v\n", err)
			} else {
				fmt.Printf("[info] deleted %d existing records with _source: %s\n", n, source)
			}
		}
	}

	// Index the new document through the pipeline.
	ids, err := runner.Invoke(ctx, document.Source{URI: path}, compose.WithCallbacks(log_callback.NewHandler(nil)))
	if err != nil {
		return fmt.Errorf("invoke index graph: %w", err)
	}
	fmt.Printf("[done] indexing file: %s, len of parts: %d\n", path, len(ids))
	return nil
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
