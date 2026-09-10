// Package cron parses cron expressions and computes matching execution times.
// It is a thin wrapper around github.com/robfig/cron/v3 configured for 5-field
// expressions (minute hour day-of-month month day-of-week) and 6-field
// expressions with a leading seconds field, including lists (,), ranges (-),
// steps (/), the any-value markers (* and ?), named months (jan-dec), named
// weekdays (sun-sat) and the @-descriptors (@yearly, @daily, ...).
package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// CronParser parses cron expressions and computes matching execution times.
type CronParser struct {
	parser cron.Parser
}

// NewCronParser returns a parser accepting optional seconds and descriptors.
func NewCronParser() *CronParser {
	return &CronParser{
		parser: cron.NewParser(
			cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
		),
	}
}

// IsValidCronExpr reports whether the given cron expression is valid.
func (c *CronParser) IsValidCronExpr(cron string) bool {
	_, err := c.parse(cron)
	return err == nil
}

// NextFromNow returns the closest time instant after now matching the expression.
func (c *CronParser) NextFromNow(cron string) (time.Time, error) {
	schedule, err := c.parse(cron)
	if err != nil {
		return time.Time{}, err
	}

	nextTime := schedule.Next(time.Now())
	if nextTime.IsZero() {
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

	schedule, err := c.parse(cron)
	if err != nil {
		return nil, err
	}

	var nexts []time.Time
	for next := schedule.Next(start); !next.IsZero() && next.Before(end); next = schedule.Next(next) {
		nexts = append(nexts, next)
	}
	return nexts, nil
}

// parse normalizes and parses a cron expression into a schedule.
func (c *CronParser) parse(cron string) (cron.Schedule, error) {
	return c.parser.Parse(normalize(cron))
}

// normalize lowercases the expression and keeps the vixie-cron extension of
// day-of-week 7 as an alias for Sunday, which robfig/cron rejects.
func normalize(expr string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(expr)))
	if len(fields) == 0 {
		return ""
	}
	fields[len(fields)-1] = normalizeDow(fields[len(fields)-1])
	return strings.Join(fields, " ")
}

// normalizeDow rewrites day-of-week 7 (Sunday) to the canonical 0: a
// standalone "7" becomes "0" and a range ending in 7, e.g. "5-7", becomes
// "5-6,0".
func normalizeDow(field string) string {
	parts := strings.Split(field, ",")
	for i, part := range parts {
		switch {
		case part == "7":
			parts[i] = "0"
		case strings.HasSuffix(part, "-7"):
			start := strings.TrimSuffix(part, "-7")
			if start == "7" {
				parts[i] = "0"
				continue
			}
			if v, err := strconv.Atoi(start); err == nil && v >= 0 && v <= 6 {
				parts[i] = start + "-6,0"
			}
		}
	}
	return strings.Join(parts, ",")
}
