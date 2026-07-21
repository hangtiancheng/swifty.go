// Copyright (c) 2026 hangtiancheng
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package tool_result

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hangtiancheng/swifty.go/swifty_cli/internal/conversation"
)

// Budget thresholds. Values match the ch08 spec; ContentReplacementState
// freezes decisions, but the actual numerical limits stay the same as the
// pre-state implementation so behavior on a fresh conversation is identical.
const (
	// SingleResultLimit spills a single tool result to a file when it exceeds
	// 50K characters, preventing context blow-up. The threshold must be
	// strictly greater than tools.MaxOutputChars (10000) + truncation suffix
	// to avoid a loop where truncated results repeatedly trigger spilling.
	SingleResultLimit = 50000

	// MessageAggregateLimit triggers Pass 2 aggregate spilling when the total
	// character count of all tool results in a single message exceeds 200K.
	MessageAggregateLimit = 200000

	// SpillSubdir lives under the agent's workdir.
	SpillSubdir = ".swifty/tool_results"
)

const (
	persistedTagPrefix = "[Result of "
)

func isAlreadyReplaced(s string) bool {
	return strings.HasPrefix(s, persistedTagPrefix)
}

// Apply implements Design A: it mutates conv in place, replacing tool_result
// content that exceeds the budget without creating a new Manager copy.
// state.SeenIDs / state.Replacements record decisions for this turn;
// subsequent calls replay the same preview string byte-for-byte to keep the
// Prompt Cache prefix stable.
//
// Returns newly added replacement records (for the caller to append to the
// session log) and any error (only filesystem-catastrophic errors; a single
// spill failure silently freezes that record to raw content without aborting
// the entire turn).
func Apply(
	conv *conversation.Manager,
	workDir string,
	state *ContentReplacementState,
) ([]Record, error) {
	messages := conv.GetMessages()
	if len(messages) == 0 {
		return nil, nil
	}

	spillDir := filepath.Join(workDir, SpillSubdir)
	absSpillDir, _ := filepath.Abs(spillDir)
	toolUseByID := buildToolUseIndex(messages)

	var records []Record

	for i, msg := range messages {
		if len(msg.ToolResults) == 0 {
			continue
		}

		decisions := make(map[string]string, len(msg.ToolResults))
		var fresh []conversation.ToolResultBlock

		for _, tr := range msg.ToolResults {
			if rep, ok := state.Replacements[tr.ToolUseID]; ok {
				// Previously decided replacement — replay the exact preview.
				decisions[tr.ToolUseID] = rep
				continue
			}
			if _, ok := state.SeenIDs[tr.ToolUseID]; ok {
				// Seen but not replaced — permanently frozen as raw content.
				decisions[tr.ToolUseID] = tr.Content
				continue
			}
			if isAlreadyReplaced(tr.Content) {
				// Externally pre-tagged content (e.g. preview already written
				// when restored from disk). Freeze to the tag itself so
				// subsequent turns remain stable.
				state.SeenIDs[tr.ToolUseID] = struct{}{}
				state.Replacements[tr.ToolUseID] = tr.Content
				decisions[tr.ToolUseID] = tr.Content
				records = append(records, Record{
					Kind:        "tool-result",
					ToolUseID:   tr.ToolUseID,
					Replacement: tr.Content,
				})
				continue
			}
			fresh = append(fresh, tr)
		}

		// Pass 1: persist any single result above SingleResultLimit.
		persistedByP1 := make(map[string]struct{})
		for _, tr := range fresh {
			if len(tr.Content) <= SingleResultLimit {
				continue
			}
			if isSpillReadback(toolUseByID[tr.ToolUseID], absSpillDir) {
				// Reading a previously-spilled file back — don't spill the
				// readback (it would loop: the readback's own result is
				// ~same size as the spilled file).
				state.SeenIDs[tr.ToolUseID] = struct{}{}
				decisions[tr.ToolUseID] = tr.Content
				persistedByP1[tr.ToolUseID] = struct{}{}
				continue
			}
			path, err := writeSpill(spillDir, tr.ToolUseID, tr.Content)
			if err != nil {
				// Spill failed → freeze as raw (decision still made: never
				// revisit). Subsequent turns will treat this id as frozen-
				// not-replaced and send the raw content.
				state.SeenIDs[tr.ToolUseID] = struct{}{}
				decisions[tr.ToolUseID] = tr.Content
				persistedByP1[tr.ToolUseID] = struct{}{}
				continue
			}
			preview := buildSpillPreview(tr.Content, path)
			decisions[tr.ToolUseID] = preview
			state.SeenIDs[tr.ToolUseID] = struct{}{}
			state.Replacements[tr.ToolUseID] = preview
			records = append(records, Record{
				Kind:        "tool-result",
				ToolUseID:   tr.ToolUseID,
				Replacement: preview,
			})
			persistedByP1[tr.ToolUseID] = struct{}{}
		}

		// Pass 2: aggregate-spill the largest remaining fresh candidates
		// until total ≤ MessageAggregateLimit.
		var remaining []conversation.ToolResultBlock
		for _, tr := range fresh {
			if _, done := persistedByP1[tr.ToolUseID]; done {
				continue
			}
			remaining = append(remaining, tr)
		}

		total := 0
		for _, c := range decisions {
			total += len(c)
		}
		for _, tr := range remaining {
			total += len(tr.Content)
		}

		if total > MessageAggregateLimit && len(remaining) > 0 {
			sorted := append([]conversation.ToolResultBlock(nil), remaining...)
			sort.SliceStable(sorted, func(a, b int) bool {
				return len(sorted[a].Content) > len(sorted[b].Content)
			})
			for _, tr := range sorted {
				if total <= MessageAggregateLimit {
					break
				}
				if isSpillReadback(toolUseByID[tr.ToolUseID], absSpillDir) {
					state.SeenIDs[tr.ToolUseID] = struct{}{}
					decisions[tr.ToolUseID] = tr.Content
					continue
				}
				path, err := writeSpill(spillDir, tr.ToolUseID, tr.Content)
				if err != nil {
					state.SeenIDs[tr.ToolUseID] = struct{}{}
					decisions[tr.ToolUseID] = tr.Content
					continue
				}
				preview := buildSpillPreview(tr.Content, path)
				decisions[tr.ToolUseID] = preview
				state.SeenIDs[tr.ToolUseID] = struct{}{}
				state.Replacements[tr.ToolUseID] = preview
				records = append(records, Record{
					Kind:        "tool-result",
					ToolUseID:   tr.ToolUseID,
					Replacement: preview,
				})
				total -= len(tr.Content) - len(preview)
			}
		}

		// Freeze remaining fresh as "seen but not replaced".
		for _, tr := range fresh {
			if _, decided := decisions[tr.ToolUseID]; decided {
				continue
			}
			state.SeenIDs[tr.ToolUseID] = struct{}{}
			decisions[tr.ToolUseID] = tr.Content
		}

		// In-place replacement: write back into the conv message history in original order.
		newResults := make([]conversation.ToolResultBlock, len(msg.ToolResults))
		for k, tr := range msg.ToolResults {
			newResults[k] = conversation.ToolResultBlock{
				ToolUseID: tr.ToolUseID,
				Content:   decisions[tr.ToolUseID],
				IsError:   tr.IsError,
			}
		}
		conv.ReplaceToolResults(i, newResults)
	}

	return records, nil
}

// previewSize is the maximum character count of the spill preview: the first
// 2KB balances readability with space consumption.
const previewSize = 2000

// buildSpillPreview builds the replacement text for a spilled tool result,
// including a 2KB preview. Once written into state.Replacements it is
// replayed byte-for-byte, so format changes will invalidate the Prompt
// Cache — do not alter lightly.
func buildSpillPreview(content string, path string) string {
	sizeKB := len(content) / 1024
	preview := content
	hasMore := false
	if len(preview) > previewSize {
		preview = preview[:previewSize]
		hasMore = true
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<persisted-output>\n")
	fmt.Fprintf(&b, "Output too large (%dKB). Full content saved to:\n%s\n\n", sizeKB, path)
	fmt.Fprintf(&b, "Preview (first 2KB):\n%s", preview)
	if hasMore {
		b.WriteString("\n...")
	}
	b.WriteString("\n</persisted-output>")
	return b.String()
}

// buildToolUseIndex maps tool_use_id → ToolUseBlock so spill decisions can
// inspect the originating tool name and arguments. Tool uses live on
// assistant messages while results live on the next user message, paired
// only by ID — pre-indexing avoids an O(n²) cross-message scan.
func buildToolUseIndex(messages []conversation.Message) map[string]conversation.ToolUseBlock {
	idx := make(map[string]conversation.ToolUseBlock, len(messages))
	for _, m := range messages {
		for _, tu := range m.ToolUses {
			idx[tu.ToolUseID] = tu
		}
	}
	return idx
}

// isSpillReadback reports whether the originating tool call was a ReadFile
// of a path inside the spill directory. Spilling the readback would loop —
// the readback's own result is ~same size as the spilled file, so it'd trip
// SingleResultLimit again and produce a chain of stub-of-stub files.
func isSpillReadback(tu conversation.ToolUseBlock, absSpillDir string) bool {
	if tu.ToolName != "ReadFile" || absSpillDir == "" {
		return false
	}
	raw, _ := tu.Arguments["file_path"].(string)
	if raw == "" {
		return false
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return false
	}
	return strings.HasPrefix(abs, absSpillDir)
}

func writeSpill(dir, toolUseID, content string) (string, error) {
	if toolUseID == "" {
		return "", fmt.Errorf("empty tool_use_id")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, toolUseID)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return path, nil
		}
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		return "", err
	}
	return path, nil
}

// PersistLargeResult spills oversized tool output to disk and returns a preview
// of it. The agent calls this when a tool result enters the conversation
// history.
func PersistLargeResult(workDir, toolUseID, content string) string {
	dir := filepath.Join(workDir, SpillSubdir)
	path, err := writeSpill(dir, toolUseID, content)
	if err != nil {
		return content
	}
	return buildSpillPreview(content, path)
}
