package tui

import (
	"testing"

	"github.com/hangtiancheng/swifty.go/swifty_cli/internal/config"
)

func TestClearResetsSession(t *testing.T) {
	provider := config.ProviderConfig{
		Name:     "test",
		Protocol: "anthropic",
		Model:    "claude-sonnet-4-20250514",
		APIKey:   "test-key",
	}
	m := New([]config.ProviderConfig{provider}, nil, nil)

	// Simulate some pre-existing state.
	m.chatMessages = []chatMessage{
		{role: "user", content: "hello"},
		{role: "assistant", content: "hi"},
	}
	m.committedUpTo = 2
	m.totalInput = 1000
	m.totalOutput = 500
	oldSessionID := m.sessionID

	// Execute /clear.
	updated, _ := m.executeCommand("clear", "")
	m = updated.(Model)

	// Verify messages are cleared.
	if len(m.chatMessages) != 0 {
		t.Errorf("chatMessages should be empty after /clear, got %d", len(m.chatMessages))
	}

	// Verify committedUpTo is reset to zero.
	if m.committedUpTo != 0 {
		t.Errorf("committedUpTo should be 0 after /clear, got %d", m.committedUpTo)
	}

	// Verify session ID has changed (differs from the old value).
	if m.sessionID == oldSessionID {
		t.Errorf("sessionID should change after /clear, still %s", m.sessionID)
	}
	if m.sessionID == "" {
		t.Error("sessionID should not be empty after /clear")
	}

	// Verify token counters are reset to zero.
	if m.totalInput != 0 {
		t.Errorf("totalInput should be 0 after /clear, got %d", m.totalInput)
	}
	if m.totalOutput != 0 {
		t.Errorf("totalOutput should be 0 after /clear, got %d", m.totalOutput)
	}

	// Verify conversation has been re-created.
	if m.conversation == nil {
		t.Error("conversation should not be nil after /clear")
	}
}
