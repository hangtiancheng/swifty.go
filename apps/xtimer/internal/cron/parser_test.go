package cron

import (
	"testing"
	"time"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return tm
}

func TestIsValidCronExpr(t *testing.T) {
	parser := NewCronParser()
	valid := []string{
		"* * * * *",     // every minute (5 fields)
		"*/5 * * * *",   // every 5 minutes
		"0 0 9 * * MON", // weekdays with names
		"0 30 9 * * *",  // 6 fields with seconds
		"0 0 0 1 jan *", // named month
		"0 0 0 1 * *",   // midnight on the 1st
		"15,45 */2 1-15 3-12 sat,sun",
		"@daily",
		"@hourly",
	}
	for _, expr := range valid {
		if !parser.IsValidCronExpr(expr) {
			t.Errorf("expected %q to be valid", expr)
		}
	}
	invalid := []string{
		"", "* * * *", "* * * * * * *", "60 * * * *", "* 24 * * *",
		"* * 32 * *", "* * * 13 *", "abc * * * *", "*/0 * * * *", "40-20 * * * *",
	}
	for _, expr := range invalid {
		if parser.IsValidCronExpr(expr) {
			t.Errorf("expected %q to be invalid", expr)
		}
	}
}

func TestNextsBetween(t *testing.T) {
	parser := NewCronParser()
	start := mustTime(t, "2026-09-01 10:00:00")
	end := mustTime(t, "2026-09-01 11:00:00")

	nexts, err := parser.NextsBetween("0 */15 * * * *", start, end)
	if err != nil {
		t.Fatalf("NextsBetween: %v", err)
	}
	want := []string{"2026-09-01 10:15:00", "2026-09-01 10:30:00", "2026-09-01 10:45:00"}
	if len(nexts) != len(want) {
		t.Fatalf("got %d nexts, want %d: %v", len(nexts), len(want), nexts)
	}
	for i, w := range want {
		if got := nexts[i].Format("2006-01-02 15:04:05"); got != w {
			t.Errorf("nexts[%d] = %s, want %s", i, got, w)
		}
	}
}

func TestNextsBetweenExcludesEnd(t *testing.T) {
	parser := NewCronParser()
	start := mustTime(t, "2026-09-01 10:00:00")
	end := mustTime(t, "2026-09-01 10:30:00")

	// Every 30 minutes: the only candidate, 10:30, equals the end boundary and
	// must be excluded.
	nexts, err := parser.NextsBetween("0 */30 * * * *", start, end)
	if err != nil {
		t.Fatalf("NextsBetween: %v", err)
	}
	if len(nexts) != 0 {
		t.Fatalf("times at or beyond the end must be excluded, got %v", nexts)
	}
}

func TestNextsBetweenDayOfWeekOrMonthDay(t *testing.T) {
	parser := NewCronParser()
	// Both day fields restricted: match when either matches (standard cron).
	// Start strictly before Sep 1 so midnight Sep 1 counts as a match.
	start := mustTime(t, "2026-08-31 22:00:00")
	end := mustTime(t, "2026-09-08 00:00:00")

	nexts, err := parser.NextsBetween("0 0 0 1 * mon", start, end)
	if err != nil {
		t.Fatalf("NextsBetween: %v", err)
	}
	// Sep 1 (day-of-month match, Tuesday) and Sep 7 (Monday).
	if len(nexts) != 2 {
		t.Fatalf("got %d nexts, want 2: %v", len(nexts), nexts)
	}
	if nexts[0].Day() != 1 || nexts[1].Day() != 7 {
		t.Errorf("unexpected days: %v, %v", nexts[0], nexts[1])
	}
}

func TestNextFromNowStepsAndRanges(t *testing.T) {
	parser := NewCronParser()
	if _, err := parser.NextFromNow("0 5 2 * * sat,sun"); err != nil {
		t.Fatalf("NextFromNow: %v", err)
	}
	if _, err := parser.NextFromNow("not a cron"); err == nil {
		t.Fatal("expected error for invalid expression")
	}
}

func TestNextsBetweenEndBeforeStart(t *testing.T) {
	parser := NewCronParser()
	end := mustTime(t, "2026-09-01 10:00:00")
	start := mustTime(t, "2026-09-01 11:00:00")
	if _, err := parser.NextsBetween("* * * * * *", start, end); err == nil {
		t.Fatal("expected error when end is before start")
	}
}
