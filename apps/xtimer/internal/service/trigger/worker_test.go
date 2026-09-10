package trigger

import (
	"testing"
)

func TestGetStartMinute(t *testing.T) {
	tm, err := getStartMinute("2026-09-01 10:00_3")
	if err != nil {
		t.Fatalf("getStartMinute: %v", err)
	}
	if got := tm.Format("2006-01-02 15:04:05"); got != "2026-09-01 10:00:00" {
		t.Fatalf("start minute = %s, want 2026-09-01 10:00:00", got)
	}

	if _, err := getStartMinute("invalid"); err == nil {
		t.Fatal("expected error for a malformed msg key")
	}
	if _, err := getStartMinute("not-a-time_1"); err == nil {
		t.Fatal("expected error for an invalid time part")
	}
}

func TestGetBucket(t *testing.T) {
	bucket, err := getBucket("2026-09-01 10:00_7")
	if err != nil {
		t.Fatalf("getBucket: %v", err)
	}
	if bucket != 7 {
		t.Fatalf("bucket = %d, want 7", bucket)
	}

	if _, err := getBucket("invalid"); err == nil {
		t.Fatal("expected error for a malformed msg key")
	}
	if _, err := getBucket("2026-09-01 10:00_notanumber"); err == nil {
		t.Fatal("expected error for a non-numeric bucket")
	}
}
