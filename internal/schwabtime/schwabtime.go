// Package schwabtime formats and parses the three time representations used
// by the Schwab REST API: ISO-8601 with millisecond precision + trailing Z,
// YYYY-MM-DD dates, and epoch-millisecond integers.
//
// The formatters omit the zero time.Time value (the caller is expected to
// skip such fields entirely).
package schwabtime

import (
	"fmt"
	"strconv"
	"time"
)

// ISOMillis formats t as RFC-3339 with millisecond precision and a literal
// trailing "Z", e.g. "2024-01-01T00:00:00.000Z". A zero t returns "".
func ISOMillis(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05.000") + "Z"
}

// ParseISOMillis parses the ISOMillis form produced by ISOMillis.
func ParseISOMillis(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05.000Z", s); err == nil {
		return t, nil
	}
	// Fall back to full RFC-3339 (with offset or nanos).
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("schwabtime: parse iso-millis %q: %w", s, err)
	}
	return t, nil
}

// YMD formats t as YYYY-MM-DD in UTC. A zero t returns "".
func YMD(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

// ParseYMD parses the YMD form produced by YMD.
func ParseYMD(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("schwabtime: parse ymd %q: %w", s, err)
	}
	return t, nil
}

// EpochMS returns t as epoch-milliseconds. A zero t returns 0.
func EpochMS(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano() / int64(time.Millisecond)
}

// ParseEpochMS parses an integer epoch-millisecond stamp.
func ParseEpochMS(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.Unix(0, ms*int64(time.Millisecond)).UTC()
}

// ParseEpochMSString parses a decimal string representation of EpochMS.
func ParseEpochMSString(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("schwabtime: parse epoch-ms %q: %w", s, err)
	}
	return ParseEpochMS(n), nil
}
