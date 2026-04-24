package schwab

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Nightsuki/goschwab/internal/schwabtime"
)

// formatCSV joins non-empty values with a comma. An empty input returns "".
// This mirrors Schwabdev's _format_list helper (client.py:90-110), used for
// symbol-list query parameters.
func formatCSV(vals []string) string {
	if len(vals) == 0 {
		return ""
	}
	keep := make([]string, 0, len(vals))
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" {
			keep = append(keep, v)
		}
	}
	return strings.Join(keep, ",")
}

// timeISOMillis formats t as ISO-8601 with millisecond precision + trailing
// "Z". A zero time returns "" so it is skipped by query builders.
func timeISOMillis(t time.Time) string { return schwabtime.ISOMillis(t) }

// timeYMD formats t as YYYY-MM-DD. A zero time returns "".
func timeYMD(t time.Time) string { return schwabtime.YMD(t) }

// timeEpochMS returns t as epoch-milliseconds; zero time returns 0.
func timeEpochMS(t time.Time) int64 { return schwabtime.EpochMS(t) }

// paramBuilder accumulates query-string values while omitting zero values.
// Internal helper used by endpoint methods to mirror Schwabdev's _parse_params.
type paramBuilder struct {
	v url.Values
}

// newParamBuilder returns an empty builder.
func newParamBuilder() *paramBuilder {
	return &paramBuilder{v: url.Values{}}
}

// addString adds a string parameter if val is non-empty.
func (p *paramBuilder) addString(key, val string) {
	if val == "" {
		return
	}
	p.v.Set(key, val)
}

// addStringList adds a CSV-joined list parameter if any element is non-empty.
func (p *paramBuilder) addStringList(key string, vals []string) {
	if s := formatCSV(vals); s != "" {
		p.v.Set(key, s)
	}
}

// addInt adds an integer parameter when val != 0.
func (p *paramBuilder) addInt(key string, val int) {
	if val == 0 {
		return
	}
	p.v.Set(key, strconv.Itoa(val))
}

// addIntPtr adds an integer parameter when val is non-nil.
func (p *paramBuilder) addIntPtr(key string, val *int) {
	if val == nil {
		return
	}
	p.v.Set(key, strconv.Itoa(*val))
}

// addInt64 adds an int64 parameter when val != 0.
func (p *paramBuilder) addInt64(key string, val int64) {
	if val == 0 {
		return
	}
	p.v.Set(key, strconv.FormatInt(val, 10))
}

// addFloatPtr adds a float parameter when val is non-nil.
func (p *paramBuilder) addFloatPtr(key string, val *float64) {
	if val == nil {
		return
	}
	p.v.Set(key, strconv.FormatFloat(*val, 'f', -1, 64))
}

// addBoolPtr adds a boolean parameter when val is non-nil.
func (p *paramBuilder) addBoolPtr(key string, val *bool) {
	if val == nil {
		return
	}
	p.v.Set(key, strconv.FormatBool(*val))
}

// addTimeISO adds an ISO-8601 ms timestamp if t is non-zero.
func (p *paramBuilder) addTimeISO(key string, t time.Time) {
	if s := timeISOMillis(t); s != "" {
		p.v.Set(key, s)
	}
}

// addTimeYMD adds a YYYY-MM-DD date if t is non-zero.
func (p *paramBuilder) addTimeYMD(key string, t time.Time) {
	if s := timeYMD(t); s != "" {
		p.v.Set(key, s)
	}
}

// addTimeEpochMS adds an epoch-ms timestamp if t is non-zero.
func (p *paramBuilder) addTimeEpochMS(key string, t time.Time) {
	if n := timeEpochMS(t); n != 0 {
		p.v.Set(key, strconv.FormatInt(n, 10))
	}
}

// values returns the accumulated url.Values. Returns nil when empty so that
// callers building a URL can omit the "?" entirely.
func (p *paramBuilder) values() url.Values {
	if len(p.v) == 0 {
		return nil
	}
	return p.v
}
