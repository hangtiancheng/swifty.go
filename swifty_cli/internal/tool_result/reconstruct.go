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

import "github.com/hangtiancheng/swifty.go/swifty_cli/internal/conversation"

// Reconstruct rebuilds state from a transcript: seed SeenIDs with every
// candidate tool_use_id present in messages (anything visible at this point
// has already been sent to the model, so the decision is implicitly
// frozen), then overlay Replacements from the on-disk records. Optional
// inheritedReplacements lets a fork resume gap-fill the parent's live
// state for ids that didn't make it into the records file (e.g. forks that
// share a tool_use_id whose preview was recorded only in the parent).
func Reconstruct(
	messages []conversation.Message,
	records []Record,

	inheritedReplacements map[string]string,
) *ContentReplacementState {
	state := New()
	candidateIDs := make(map[string]struct{})
	for _, m := range messages {
		for _, tr := range m.ToolResults {
			candidateIDs[tr.ToolUseID] = struct{}{}
		}
	}
	for id := range candidateIDs {
		state.SeenIDs[id] = struct{}{}

	}
	for _, r := range records {
		if r.Kind != "tool-result" {
			continue
		}
		if _, ok := candidateIDs[r.ToolUseID]; ok {
			state.Replacements[r.ToolUseID] = r.Replacement
		}
	}
	for id, rep := range inheritedReplacements {
		if _, ok := candidateIDs[id]; !ok {
			continue
		}
		if _, exists := state.Replacements[id]; exists {
			continue
		}
		state.Replacements[id] = rep
	}
	return state
}
