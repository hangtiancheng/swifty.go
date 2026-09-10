package utils

import (
	"testing"
	"time"
)

func TestUnionAndSplitTimerIDUnix(t *testing.T) {
	key := UnionTimerIDUnix(42, 1785000000000)
	gotID, gotUnix, err := SplitTimerIDUnix(key)
	if err != nil {
		t.Fatalf("SplitTimerIDUnix: %v", err)
	}
	if gotID != 42 || gotUnix != 1785000000000 {
		t.Fatalf("round trip mismatch: id=%d unix=%d", gotID, gotUnix)
	}

	if _, _, err := SplitTimerIDUnix("invalid"); err == nil {
		t.Fatal("expected error for malformed key")
	}
	if _, _, err := SplitTimerIDUnix("notanumber_123"); err == nil {
		t.Fatal("expected error for non-numeric timer id")
	}
}

func TestSplitTimeBucket(t *testing.T) {
	bucketTime, bucket, err := SplitTimeBucket("2026-09-01 10:00_3")
	if err != nil {
		t.Fatalf("SplitTimeBucket: %v", err)
	}
	if bucket != 3 {
		t.Fatalf("bucket = %d, want 3", bucket)
	}
	if got := bucketTime.Format("2006-01-02 15:04"); got != "2026-09-01 10:00" {
		t.Fatalf("time = %s, want 2026-09-01 10:00", got)
	}

	if _, _, err := SplitTimeBucket("invalid"); err == nil {
		t.Fatal("expected error for malformed bucket key")
	}
}

func TestGetForwardTwoMigrateStepEnd(t *testing.T) {
	cur := time.Date(2026, 9, 1, 10, 30, 15, 0, time.Local)
	end := GetForwardTwoMigrateStepEnd(cur, 2*time.Hour)
	if got := end.Format("2006-01-02 15:04:05"); got != "2026-09-01 12:00:00" {
		t.Fatalf("expected truncated to the hour of cur+2h, got %s", got)
	}
}

func TestLockAndBloomKeys(t *testing.T) {
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.Local)
	if got := GetMigratorLockKey(now); got != "migrator_lock_2026-09-01 10" {
		t.Errorf("GetMigratorLockKey = %q", got)
	}
	if got := GetTimeBucketLockKey(now, 5); got != "time_bucket_lock_2026-09-01 10:00_5" {
		t.Errorf("GetTimeBucketLockKey = %q", got)
	}
	if got := GetTaskBloomFilterKey("2026-09-01"); got != "task_bloom_2026-09-01" {
		t.Errorf("GetTaskBloomFilterKey = %q", got)
	}
	if got := GetSliceMsgKey(now, 7); got != "2026-09-01 10:00_7" {
		t.Errorf("GetSliceMsgKey = %q", got)
	}
}
