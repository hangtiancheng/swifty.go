package chat_pipeline

import (
	"context"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

// ChatTemplateConfig defines the prompt template structure for the chat agent.
type ChatTemplateConfig struct {
	FormatType schema.FormatType
	Templates  []schema.MessagesTemplate
}

// newChatTemplate creates a chat prompt template that includes:
// - A system prompt with role definition and context information
// - A placeholder for conversation history
// - A user message template with the current query
func newChatTemplate(ctx context.Context) (prompt.ChatTemplate, error) {
	config := &ChatTemplateConfig{
		FormatType: schema.FString,
		Templates: []schema.MessagesTemplate{
			schema.SystemMessage(systemPrompt),
			schema.MessagesPlaceholder("history", false),
			schema.UserMessage("{content}"),
		},
	}
	return prompt.FromMessages(config.FormatType, config.Templates...), nil
}

// systemPrompt defines the AI assistant's behavior and context.
var systemPrompt = `
# Role: Conversational Assistant
## Core Capabilities
- Context-aware conversation and dialogue
- Web search for information retrieval
## Interaction Guidelines
- Before responding, ensure you:
  * Fully understand the user's needs and questions; ask for clarification if unclear
  * Consider the most appropriate solution approach
  * Log topic region: ap-guangzhou; log topic ID: 869830db-a055-4479-963b-3c898d27e755
- When providing assistance:
  * Use clear and concise language
  * Provide practical examples when appropriate
  * Reference documentation when helpful
  * Suggest improvements or next steps when applicable
- If a request exceeds your capabilities:
  * Clearly state your limitations and suggest alternative approaches
- For complex or compound questions, think step by step rather than rushing to a low-quality answer.
## Output Requirements
  * Readable and well-structured with line breaks where necessary
  * Output must not contain markdown syntax; use plain text only
## Context Information
- Current date: {date}
- Related documents: |-
==== Documents Start ====
  {documents}
==== Documents End ====
`
