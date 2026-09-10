// Package cron provides a self-contained parser and scheduler for standard
// cron expressions. It supports 5-field expressions (minute hour day-of-month
// month day-of-week) and 6-field expressions with a leading seconds field,
// including lists (,), ranges (-), steps (/), the any-value markers (* and ?),
// named months (jan-dec) and named weekdays (sun-sat).
package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronParser parses cron expressions and computes matching execution times.
type CronParser struct{}

func NewCronParser() *CronParser {
	return &CronParser{}
}

// IsValidCronExpr reports whether the given cron expression is valid.
func (c *CronParser) IsValidCronExpr(cron string) bool {
	_, err := parseExpression(cron)
	return err == nil
}

// NextFromNow returns the closest time instant after now matching the expression.
func (c *CronParser) NextFromNow(cron string) (time.Time, error) {
	expr, err := parseExpression(cron)
	if err != nil {
		return time.Time{}, err
	}

	nextTime, ok := expr.next(time.Now())
	if !ok {
		return time.Time{}, fmt.Errorf("failed to get next time for cron: %s", cron)
	}
	return nextTime, nil
}

// NextsBefore returns all matching times between now and end (exclusive).
func (c *CronParser) NextsBefore(cron string, end time.Time) ([]time.Time, error) {
	return c.NextsBetween(cron, time.Now(), end)
}

// NextsBetween returns all matching times in [start, end). Times are returned
// in ascending order and are strictly greater than start.
func (c *CronParser) NextsBetween(cron string, start, end time.Time) ([]time.Time, error) {
	if end.Before(start) {
		return nil, fmt.Errorf("end must not be earlier than start, start: %v, end: %v", start, end)
	}

	expr, err := parseExpression(cron)
	if err != nil {
		return nil, err
	}

	var nexts []time.Time
	for next, ok := expr.next(start); ok && next.Before(end); next, ok = expr.next(next) {
		nexts = append(nexts, next)
	}
	return nexts, nil
}

// expression is a parsed cron expression. Every field is a bitset of accepted
// values. For a 5-field expression the seconds set contains only 0.
type expression struct {
	seconds   [60]bool
	minutes   [60]bool
	hours     [24]bool
	monthDays [32]bool // index 1..31
	months    [13]bool // index 1..12
	weekDays  [7]bool  // index 0..6, 0 is Sunday
	// monthDayRestricted / weekDayRestricted record whether the corresponding
	// field was restricted (not "*"). When both are restricted the day matches
	// if either field matches (standard vixie-cron semantics).
	monthDayRestricted bool
	weekDayRestricted  bool
}

var namedDescriptors = map[string]string{
	"@yearly":   "0 0 0 1 1 *",
	"@annually": "0 0 0 1 1 *",
	"@monthly":  "0 0 0 1 * *",
	"@weekly":   "0 0 0 * * 0",
	"@daily":    "0 0 0 * * *",
	"@midnight": "0 0 0 * * *",
	"@hourly":   "0 0 * * * *",
}

// parseExpression parses a 5-field or 6-field cron expression.
func parseExpression(cron string) (*expression, error) {
	expr := strings.TrimSpace(strings.ToLower(cron))
	if expr == "" {
		return nil, fmt.Errorf("empty cron expression")
	}

	if expanded, ok := namedDescriptors[expr]; ok {
		expr = expanded
	}

	fields := strings.Fields(expr)
	switch len(fields) {
	case 5: // minute hour day-of-month month day-of-week
		fields = append([]string{"0"}, fields...)
	case 6: // second minute hour day-of-month month day-of-week
	default:
		return nil, fmt.Errorf("cron expression must have 5 or 6 fields, got %d: %s", len(fields), cron)
	}

	e := &expression{}

	if err := parseField(fields[0], 0, 59, nil, func(v int) { e.seconds[v] = true }); err != nil {
		return nil, fmt.Errorf("invalid seconds field %q: %w", fields[0], err)
	}
	if err := parseField(fields[1], 0, 59, nil, func(v int) { e.minutes[v] = true }); err != nil {
		return nil, fmt.Errorf("invalid minutes field %q: %w", fields[1], err)
	}
	if err := parseField(fields[2], 0, 23, nil, func(v int) { e.hours[v] = true }); err != nil {
		return nil, fmt.Errorf("invalid hours field %q: %w", fields[2], err)
	}
	if fields[3] != "*" && fields[3] != "?" {
		e.monthDayRestricted = true
	}
	if err := parseField(fields[3], 1, 31, nil, func(v int) { e.monthDays[v] = true }); err != nil {
		return nil, fmt.Errorf("invalid day-of-month field %q: %w", fields[3], err)
	}
	if err := parseField(fields[4], 1, 12, monthNames, func(v int) { e.months[v] = true }); err != nil {
		return nil, fmt.Errorf("invalid month field %q: %w", fields[4], err)
	}
	if fields[5] != "*" && fields[5] != "?" {
		e.weekDayRestricted = true
	}
	if err := parseField(fields[5], 0, 6, weekDayNames, func(v int) { e.weekDays[v] = true }); err != nil {
		return nil, fmt.Errorf("invalid day-of-week field %q: %w", fields[5], err)
	}

	return e, nil
}

var monthNames = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

var weekDayNames = map[string]int{
	"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
}

// parseField marks every value accepted by a single cron field via set.
// min/max bound the field, names optionally maps aliases (jan, mon, ...).
func parseField(field string, min, max int, names map[string]int, set func(int)) error {
	accept := func(v int) {
		set(normalizeValue(v, names))
	}

	for _, part := range strings.Split(field, ",") {
		if part == "" {
			return fmt.Errorf("empty list element")
		}

		// Split off the step, e.g. "*/5" or "10-20/2".
		rangePart, stepPart := part, ""
		if before, after, found := strings.Cut(part, "/"); found {
			rangePart, stepPart = before, after
		}

		start, end := min, max
		switch {
		case rangePart == "*" || rangePart == "?":
			// keep full bounds
		case strings.Contains(rangePart, "-"):
			bounds := strings.SplitN(rangePart, "-", 2)
			first, err := parseValue(bounds[0], names)
			if err != nil {
				return err
			}
			last, err := parseValue(bounds[1], names)
			if err != nil {
				return err
			}
			if first < min || last > max || first > last {
				return fmt.Errorf("range %q out of bounds [%d,%d]", rangePart, min, max)
			}
			start, end = first, last
		default:
			value, err := parseValue(rangePart, names)
			if err != nil {
				return err
			}
			if value < min || value > max {
				return fmt.Errorf("value %d out of bounds [%d,%d]", value, min, max)
			}
			// A plain value with a step (e.g. "5/15") starts a range at that
			// value and runs to the field maximum, matching common cron usage.
			if stepPart != "" {
				end = max
			} else {
				end = value
			}
			start = value
		}

		if stepPart != "" {
			step, err := strconv.Atoi(stepPart)
			if err != nil || step <= 0 {
				return fmt.Errorf("invalid step %q", stepPart)
			}
			for v := start; v <= end; v += step {
				accept(v)
			}
		} else {
			for v := start; v <= end; v++ {
				accept(v)
			}
		}
	}
	return nil
}

func parseValue(value string, names map[string]int) (int, error) {
	if names != nil {
		if v, ok := names[value]; ok {
			return v, nil
		}
	}
	v, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid value %q", value)
	}
	return v, nil
}

// normalizeValue maps out-of-range aliases onto canonical values. The only case
// is day-of-week 7 which standard cron treats as Sunday (0).
func normalizeValue(v int, names map[string]int) int {
	if v == 7 && names != nil {
		if _, ok := names["sun"]; ok {
			return 0
		}
	}
	return v
}

// next returns the closest time instant strictly after from that matches the
// expression. ok is false when no match exists within five years of from.
func (e *expression) next(from time.Time) (time.Time, bool) {
	loc := from.Location()
	t := time.Date(from.Year(), from.Month(), from.Day(), from.Hour(), from.Minute(), from.Second(), 0, loc).Add(time.Second)
	yearLimit := from.Year() + 5

	for t.Year() <= yearLimit {
		if !e.months[int(t.Month())] {
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, loc)
			continue
		}
		if !e.dayMatches(t) {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, loc)
			continue
		}
		if !e.hours[t.Hour()] {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, loc)
			continue
		}
		if !e.minutes[t.Minute()] {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute()+1, 0, 0, loc)
			continue
		}
		if !e.seconds[t.Second()] {
			t = t.Add(time.Second)
			continue
		}
		return t, true
	}
	return time.Time{}, false
}

func (e *expression) dayMatches(t time.Time) bool {
	monthDayOK := e.monthDays[t.Day()]
	weekDayOK := e.weekDays[int(t.Weekday())]
	switch {
	case e.monthDayRestricted && e.weekDayRestricted:
		return monthDayOK || weekDayOK
	case e.monthDayRestricted:
		return monthDayOK
	case e.weekDayRestricted:
		return weekDayOK
	default:
		return true
	}
}
