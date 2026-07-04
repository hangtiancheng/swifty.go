package tools

import (
	"context"
	"encoding/json"
	"log"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/ai/retriever"
	"github.com/hangtiancheng/swifty.go/swifty_agent/internal/config"
)

// QueryInternalDocsInput defines the input for the internal docs search tool.
type QueryInternalDocsInput struct {
	Query string `json:"query" jsonschema:"description=The query string to search in internal documentation for relevant information and processing steps"`
}

// NewQueryInternalDocsTool creates a tool that searches the internal knowledge base
// using RAG (Retrieval-Augmented Generation) to find relevant documents and
// extract processing steps from the company's documentation.
func NewQueryInternalDocsTool(cfg *config.Config) tool.InvokableTool {
	t, err := utils.InferOptionableTool(
		"query_internal_docs",
		"Search internal documentation and knowledge base for relevant information. Performs RAG to find similar documents and extract processing steps. Useful for understanding internal procedures, best practices, or step-by-step guides.",
		func(ctx context.Context, input *QueryInternalDocsInput, opts ...tool.Option) (string, error) {
			rr, err := retriever.NewMilvusRetriever(ctx, cfg)
			if err != nil {
				log.Fatal(err)
			}
			resp, err := rr.Retrieve(ctx, input.Query)
			if err != nil {
				log.Fatal(err)
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	return t
}
