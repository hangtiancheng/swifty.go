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

// Package toolresult implements Swifty's Design-B tool-result budget:
// every replacement decision is recorded in ContentReplacementState and
// applied to a freshly-built *conversation.Manager so the input conversation
// is never mutated. State across turns is what makes the model-visible
// prefix byte-stable for Anthropic prompt cache.
package tool_result

// ContentReplacementState is the per-conversation-thread decision log.
//
//   - SeenIDs holds every tool_use_id that has passed through Apply at least
//     once. Once seen, the decision (replaced or not) is frozen forever for
//     that id.
//   - Replacements holds the byte-exact preview string for every id that was
//     decided "replace". Subsequent turns re-apply this string verbatim — no
//     filesystem I/O, no formatting, no chance of drift.
//
// Invariant: keys(Replacements) ⊆ SeenIDs.
type ContentReplacementState struct {
	SeenIDs      map[string]struct{}
	Replacements map[string]string
}

// New returns an empty state. One instance per conversation thread; the
// main Agent holds one and threads it through every Apply call.
func New() *ContentReplacementState {
	return &ContentReplacementState{
		SeenIDs:      make(map[string]struct{}),
		Replacements: make(map[string]string),
	}
}

// Clone produces an independent copy. Used at fork time so the child agent
// inherits the parent's frozen decisions but does not write back into the
// parent's map. Both maps store value types (struct{} / string), so a key
// copy is sufficient — no deep copy needed.
func (s *ContentReplacementState) Clone() *ContentReplacementState {
	out := &ContentReplacementState{
		SeenIDs:      make(map[string]struct{}, len(s.SeenIDs)),
		Replacements: make(map[string]string, len(s.Replacements)),
	}
	for k := range s.SeenIDs {
		out.SeenIDs[k] = struct{}{}
	}
	for k, v := range s.Replacements {
		out.Replacements[k] = v
	}
	return out
}
