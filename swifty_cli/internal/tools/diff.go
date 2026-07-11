package tools

import (
	"fmt"
	"strings"
)

const (
	diffContextLines = 3
	// maxDiffLines caps the diff output to prevent excessively large diffs
	// from degrading TUI rendering performance and consuming too much context.
	maxDiffLines = 200
)

// DiffResult is the return value of BuildDiff: a line-numbered diff string
// along with counts of added and removed lines.
type DiffResult struct {
	Text      string
	Additions int
	Removals  int
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// BuildDiff compares the old and new file contents, producing a line-numbered diff.
// It leverages the observation that edits typically modify a small contiguous region:
// common prefix and suffix lines are matched from both ends, avoiding a full
// LCS/Myers diff (faster for large files and simpler to implement).
func BuildDiff(oldContent, newContent string) DiffResult {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	prefixLen := 0
	maxPrefix := min(len(oldLines), len(newLines))
	for prefixLen < maxPrefix && oldLines[prefixLen] == newLines[prefixLen] {
		prefixLen++
	}

	suffixLen := 0
	maxSuffix := maxPrefix - prefixLen
	for suffixLen < maxSuffix &&
		oldLines[len(oldLines)-1-suffixLen] == newLines[len(newLines)-1-suffixLen] {
		suffixLen++
	}

	removedLines := oldLines[prefixLen : len(oldLines)-suffixLen]
	addedLines := newLines[prefixLen : len(newLines)-suffixLen]

	contextStart := max(0, prefixLen-diffContextLines)
	contextBefore := oldLines[contextStart:prefixLen]
	contextEnd := min(len(oldLines), len(oldLines)-suffixLen+diffContextLines)
	contextAfter := oldLines[len(oldLines)-suffixLen : contextEnd]

	var out []string
	oldLineNo := contextStart + 1
	newLineNo := contextStart + 1
	truncated := false

	push := func(prefix string, lineNo int, content string) {
		if len(out) >= maxDiffLines {
			truncated = true
			return
		}
		out = append(out, fmt.Sprintf("%s %4d  %s", prefix, lineNo, content))
	}

	for _, l := range contextBefore {
		push(" ", oldLineNo, l)
		oldLineNo++
		newLineNo++
	}
	for _, l := range removedLines {
		push("-", oldLineNo, l)
		oldLineNo++
	}
	for _, l := range addedLines {
		push("+", newLineNo, l)
		newLineNo++
	}
	for _, l := range contextAfter {
		push(" ", oldLineNo, l)
		oldLineNo++
		newLineNo++
	}

	if truncated {
		out = append(out, fmt.Sprintf("  … (diff truncated at %d lines)", maxDiffLines))
	}

	return DiffResult{
		Text:      strings.Join(out, "\n"),
		Additions: len(addedLines),
		Removals:  len(removedLines),
	}
}
