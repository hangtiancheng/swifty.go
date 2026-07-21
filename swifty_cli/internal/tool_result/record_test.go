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
	"path/filepath"
	"testing"
)

func TestAppendAndLoadRecordsRoundtrip(t *testing.T) {
	dir := t.TempDir()
	first := []Record{
		{ToolUseID: "a", Replacement: "aaa"},
		{ToolUseID: "b", Replacement: "bbb"},
	}
	if err := AppendRecords(dir, first); err != nil {
		t.Fatalf("AppendRecords first: %v", err)
	}
	if err := AppendRecords(dir, []Record{{ToolUseID: "c", Replacement: "ccc"}}); err != nil {
		t.Fatalf("AppendRecords second: %v", err)
	}

	loaded, err := LoadRecords(dir)
	if err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 records, got %d", len(loaded))
	}
	wantIDs := []string{"a", "b", "c"}
	for i, r := range loaded {
		if r.ToolUseID != wantIDs[i] {
			t.Fatalf("record %d id: got %q want %q", i, r.ToolUseID, wantIDs[i])
		}
		if r.Kind != "tool-result" {
			t.Fatalf("record %d kind: got %q (Append should default-fill)", i, r.Kind)
		}
	}

	path := filepath.Join(dir, RecordsFilename)
	if path == "" {
		t.Fatal("RecordsFilename empty")
	}
}

func TestLoadRecordsMissingFile(t *testing.T) {
	loaded, err := LoadRecords(t.TempDir())
	if err != nil {
		t.Fatalf("LoadRecords on missing file: %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected nil records on missing file, got %v", loaded)
	}
}
