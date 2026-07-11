// Package consolidation implements background memory consolidation (autoDream).
//
// Consolidation is triggered automatically when two gate conditions are met:
// more than minHours hours have elapsed since the last consolidation, and
// more than minSessions sessions have accumulated in the meantime. Once triggered,
// a sub-agent is forked in the background to scan existing memories, merge duplicates,
// remove stale entries, fix contradictions, and maintain the index.
package consolidation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hangtiancheng/swifty.go/swifty_cli/internal/agent"
	"github.com/hangtiancheng/swifty.go/swifty_cli/internal/agents"
	"github.com/hangtiancheng/swifty.go/swifty_cli/internal/conversation"
	"github.com/hangtiancheng/swifty.go/swifty_cli/internal/llm"
	"github.com/hangtiancheng/swifty.go/swifty_cli/internal/memory"
	"github.com/hangtiancheng/swifty.go/swifty_cli/internal/permissions"
	"github.com/hangtiancheng/swifty.go/swifty_cli/internal/session"
	"github.com/hangtiancheng/swifty.go/swifty_cli/internal/tools"
)

const (
	defaultMinHours    = 24
	defaultMinSessions = 5
	// scanThrottleMs prevents scanning the session directory on every turn
	// when the time gate passes but the session gate does not.
	scanThrottleMs = 10 * 60 * 1000
)

// Deps holds the external dependencies for the Consolidator.
type Deps struct {
	MemoryDir     string                // <wd>/.swifty/memory/
	UserMemoryDir string                // ~/.swifty/memory/
	ProjectRoot   string                // Absolute path to the project root.
	Client        llm.Client            // LLM client.
	ToolRegistry  *tools.Registry       // Parent agent's tool registry.
	Protocol      string                // "anthropic" / "openai"
	Conversation  *conversation.Manager // Parent agent's conversation.
	AppendSystem  func(string)          // Notify the TUI.
	DebugLogf     func(string, ...any)  // Debug logger.
}

// Consolidator manages background memory consolidation state and execution.
type Consolidator struct {
	deps        Deps
	lastScanAt  int64 // Timestamp (ms) of the last session directory scan.
	minHours    int
	minSessions int
}

// NewConsolidator creates a new consolidator instance.
func NewConsolidator(deps Deps) *Consolidator {
	return &Consolidator{
		deps:        deps,
		minHours:    defaultMinHours,
		minSessions: defaultMinSessions,
	}
}

// SetThresholds overrides the default gate thresholds. Intended for testing.
func (c *Consolidator) SetThresholds(minHours, minSessions int) {
	c.minHours = minHours
	c.minSessions = minSessions
}

// MaybeRun checks the gate conditions and triggers a background consolidation if met.
// Designed to be called after each agent loop iteration with minimal overhead (a single stat).
func (c *Consolidator) MaybeRun(ctx context.Context) {
	if c == nil {
		return
	}
	// Skip if the memory directory does not exist.
	if _, err := os.Stat(strings.TrimRight(c.deps.MemoryDir, string(filepath.Separator))); os.IsNotExist(err) {
		return
	}

	// Time gate: check whether enough time has elapsed since the last consolidation.
	lastAt, err := ReadLastConsolidatedAt(c.deps.MemoryDir)
	if err != nil {
		c.debugf("[consolidation] ReadLastConsolidatedAt failed: %v", err)
		return
	}
	hoursSince := float64(time.Now().UnixMilli()-lastAt) / 3_600_000
	if hoursSince < float64(c.minHours) {
		return
	}

	// Scan throttle: avoid scanning the session directory on every turn.
	now := time.Now().UnixMilli()
	if now-c.lastScanAt < scanThrottleMs {
		c.debugf("[consolidation] scan throttle — last scan %ds ago", (now-c.lastScanAt)/1000)
		return
	}
	c.lastScanAt = now

	// Session gate: check whether enough sessions have accumulated.
	sessionIDs := listSessionsSince(c.deps.ProjectRoot, lastAt)
	if len(sessionIDs) < c.minSessions {
		c.debugf("[consolidation] skip — %d sessions since last consolidation, need %d",
			len(sessionIDs), c.minSessions)
		return
	}

	// Acquire the consolidation lock.
	priorMtime, err := TryAcquireLock(c.deps.MemoryDir)
	if err != nil {
		c.debugf("[consolidation] lock acquire failed: %v", err)
		return
	}
	if priorMtime == -1 {
		c.debugf("[consolidation] lock held by another process")
		return
	}

	c.debugf("[consolidation] firing — %.1fh since last, %d sessions to review",
		hoursSince, len(sessionIDs))

	go c.run(ctx, sessionIDs, priorMtime)
}

func (c *Consolidator) run(ctx context.Context, sessionIDs []string, priorMtime int64) {
	defer func() {
		if r := recover(); r != nil {
			c.debugf("[consolidation] panic: %v", r)
			RollbackLock(c.deps.MemoryDir, priorMtime)
		}
	}()

	transcriptDir := filepath.Join(c.deps.ProjectRoot, ".swifty", "sessions")
	prompt := BuildConsolidationPrompt(
		c.deps.MemoryDir, c.deps.UserMemoryDir,
		transcriptDir, sessionIDs,
	)

	// Build an independent conversation: start from scratch without inheriting parent agent context.
	conv := conversation.NewManager()
	conv.AddUserMessage(prompt)

	// Tool filtering: provide the sub-agent with async-level tools.
	subRegistry := agents.FilterToolsForAgent(c.deps.ToolRegistry, nil, nil, true)

	// Path sandbox: restrict writes to the memory directories only.
	sandboxRoots := []string{c.deps.MemoryDir}
	if c.deps.UserMemoryDir != "" {
		sandboxRoots = append(sandboxRoots, c.deps.UserMemoryDir)
	}
	subSandbox := permissions.NewPathSandbox(sandboxRoots[0], sandboxRoots[1:]...)
	subChecker := permissions.NewChecker(subSandbox, &permissions.RuleEngine{}, permissions.ModeBypass)

	subAgent := agent.New(c.deps.Client, subRegistry, c.deps.Protocol)
	subAgent.MaxIterations = 15 // Consolidation may require multiple read/write rounds.
	subAgent.Checker = subChecker
	subAgent.WorkDir = c.deps.ProjectRoot

	startTime := time.Now()
	ch := subAgent.Run(ctx, conv)
	for range ch {
		// drain
	}

	writtenPaths := extractWrittenPaths(conv.GetMessages())
	c.debugf("[consolidation] finished in %s, %d files touched: %v",
		time.Since(startTime), len(writtenPaths), writtenPaths)

	// Filter out the index file; only notify for actual memory file changes.
	var memoryPaths []string
	for _, p := range writtenPaths {
		if filepath.Base(p) == memory.AutoMemEntrypointName {
			continue
		}
		memoryPaths = append(memoryPaths, p)
	}

	if len(memoryPaths) > 0 && c.deps.AppendSystem != nil {
		var names []string
		for _, p := range memoryPaths {
			names = append(names, filepath.Base(p))
		}
		c.deps.AppendSystem(fmt.Sprintf("Memory improved: %s", strings.Join(names, ", ")))
	}
}

// listSessionsSince returns session IDs modified after the given timestamp (ms).
func listSessionsSince(projectRoot string, sinceMs int64) []string {
	sessions := session.ListSessions(projectRoot)
	since := time.UnixMilli(sinceMs)
	var ids []string
	for _, s := range sessions {
		if s.ModTime.After(since) {
			ids = append(ids, s.ID)
		}
	}
	return ids
}

// extractWrittenPaths extracts all file paths written or edited by the sub-agent
// from the conversation message history.
func extractWrittenPaths(messages []conversation.Message) []string {
	var paths []string
	seen := make(map[string]struct{})
	for _, m := range messages {
		if m.Role != "assistant" {
			continue
		}
		for _, tu := range m.ToolUses {
			if tu.ToolName != "WriteFile" && tu.ToolName != "EditFile" {
				continue
			}
			fp, ok := tu.Arguments["file_path"].(string)
			if !ok || fp == "" {
				continue
			}
			if _, exists := seen[fp]; exists {
				continue
			}
			seen[fp] = struct{}{}
			paths = append(paths, fp)
		}
	}
	return paths
}

func (c *Consolidator) debugf(format string, args ...any) {
	if c.deps.DebugLogf != nil {
		c.deps.DebugLogf(format, args...)
	}
}
