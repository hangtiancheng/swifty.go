package knowledge_index_pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/loader"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/consts"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/utility/log_callback"
	swifty_milvus "github.com/hangtiancheng/swifty.go/swifty_agent/internal/utility/milvus"
)

// IndexFile indexes a single document file into the Milvus knowledge base.
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

	// Connect to Milvus and remove existing documents with the same source.
	cli, err := swifty_milvus.NewClient(ctx, cfg)
	if err != nil {
		return err
	}

	source := docs[0].MetaData["_source"]
	expr := fmt.Sprintf(`metadata["_source"] == "%s"`, source)
	queryResult, err := cli.Query(ctx, consts.MilvusCollectionName, []string{}, expr, []string{"id"})
	if err == nil && len(queryResult) > 0 {
		var idsToDelete []string
		for _, column := range queryResult {
			if column.Name() == "id" {
				for i := 0; i < column.Len(); i++ {
					id, err := column.GetAsString(i)
					if err == nil {
						idsToDelete = append(idsToDelete, id)
					}
				}
			}
		}
		if len(idsToDelete) > 0 {
			deleteExpr := fmt.Sprintf(`id in ["%s"]`, strings.Join(idsToDelete, `","`))
			if err := cli.Delete(ctx, consts.MilvusCollectionName, "", deleteExpr); err != nil {
				fmt.Printf("[warn] delete existing data failed: %v\n", err)
			} else {
				fmt.Printf("[info] deleted %d existing records with _source: %s\n", len(idsToDelete), source)
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
