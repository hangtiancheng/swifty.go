package uuid

import (
	"regexp"
	"strings"
	"testing"
)

var uuidV4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewFormat(t *testing.T) {
	for i := 0; i < 100; i++ {
		id := New()
		if !uuidV4Pattern.MatchString(id) {
			t.Fatalf("New() = %q, want RFC 4122 version 4 format", id)
		}
	}
}

func TestNewUnique(t *testing.T) {
	const count = 1000
	seen := make(map[string]struct{}, count)
	for i := 0; i < count; i++ {
		id := New()
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate uuid generated: %s", id)
		}
		seen[id] = struct{}{}
	}
}

func TestNewLengthAndSeparators(t *testing.T) {
	id := New()
	if len(id) != 36 {
		t.Fatalf("len(New()) = %d, want 36", len(id))
	}
	if got := strings.Count(id, "-"); got != 4 {
		t.Fatalf("New() = %q, contains %d dashes, want 4", id, got)
	}
}
