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

import "testing"

func TestNewReturnsEmpty(t *testing.T) {
	s := New()
	if len(s.SeenIDs) != 0 {
		t.Fatalf("SeenIDs not empty: %v", s.SeenIDs)
	}
	if len(s.Replacements) != 0 {
		t.Fatalf("Replacements not empty: %v", s.Replacements)
	}
}

func TestCloneIndependent(t *testing.T) {
	src := New()
	src.SeenIDs["a"] = struct{}{}
	src.Replacements["a"] = "preview_a"

	cloned := src.Clone()
	cloned.SeenIDs["b"] = struct{}{}
	cloned.Replacements["b"] = "preview_b"

	if _, ok := src.SeenIDs["b"]; ok {
		t.Fatal("source SeenIDs mutated by clone write")
	}
	if _, ok := src.Replacements["b"]; ok {
		t.Fatal("source Replacements mutated by clone write")
	}
	if _, ok := cloned.SeenIDs["a"]; !ok {
		t.Fatal("clone missing source-seeded id 'a'")
	}
	if cloned.Replacements["a"] != "preview_a" {
		t.Fatal("clone preview_a not preserved")
	}
}
