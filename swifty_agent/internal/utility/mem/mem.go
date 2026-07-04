// Package mem provides an in-memory conversation history manager.
// It stores per-session message lists with a sliding window to keep
// memory usage bounded during multi-turn conversations.
package mem

import (
	"sync"

	"github.com/cloudwego/eino/schema"
)

var (
	memMap = make(map[string]*ConversationMemory)
	mu     sync.Mutex
)

// Get retrieves or creates a ConversationMemory for the given session ID.
// The returned memory is safe for concurrent use.
func Get(id string) *ConversationMemory {
	mu.Lock()
	defer mu.Unlock()

	if m, ok := memMap[id]; ok {
		return m
	}
	m := &ConversationMemory{
		ID:            id,
		Messages:      []*schema.Message{},
		MaxWindowSize: 6,
	}
	memMap[id] = m
	return m
}

// ConversationMemory holds a bounded sliding window of conversation messages
// for a single chat session.
type ConversationMemory struct {
	ID            string            `json:"id"`
	Messages      []*schema.Message `json:"messages"`
	MaxWindowSize int
	mu            sync.Mutex
}

// Append adds a message to the conversation history.
// When the history exceeds MaxWindowSize, the oldest messages are
// removed in pairs to maintain the user/assistant message pairing.
func (m *ConversationMemory) Append(msg *schema.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Messages = append(m.Messages, msg)
	if len(m.Messages) > m.MaxWindowSize {
		// Ensure even number of dropped messages to preserve conversation pairing.
		excess := len(m.Messages) - m.MaxWindowSize
		if excess%2 != 0 {
			excess++
		}
		m.Messages = m.Messages[excess:]
	}
}

// All returns a snapshot of the current conversation messages.
func (m *ConversationMemory) All() []*schema.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Messages
}
