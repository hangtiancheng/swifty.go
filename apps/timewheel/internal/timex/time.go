// Package timex provides small time helpers used by the timewheel.
package timex

import "time"

// minuteLayout is the layout used for minute-level time strings, e.g. "2024-01-02-15:04".
const minuteLayout = "2006-01-02-15:04"

// GetTimeMinuteStr formats t as a minute-level string, e.g. "2024-01-02-15:04".
// It is used to derive the minute-level shard keys of the redis time wheel.
func GetTimeMinuteStr(t time.Time) string {
	return t.Format(minuteLayout)
}

// TruncateToSecond returns t truncated (zeroed) to the second, keeping the
// local time zone.
func TruncateToSecond(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local)
}
