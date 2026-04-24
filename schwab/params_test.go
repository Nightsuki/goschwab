package schwab

import (
	"testing"
	"time"
)

func TestFormatCSV(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{"nil", nil, ""},
		{"empty", []string{}, ""},
		{"single", []string{"AMD"}, "AMD"},
		{"multi", []string{"AMD", "INTC"}, "AMD,INTC"},
		{"whitespace trimmed", []string{" AMD ", "INTC"}, "AMD,INTC"},
		{"empties skipped", []string{"", "AMD", "", "INTC", ""}, "AMD,INTC"},
		{"all empty", []string{"", "", ""}, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := formatCSV(tc.in)
			if got != tc.want {
				t.Fatalf("formatCSV(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTimeFormatters(t *testing.T) {
	ref := time.Date(2024, 1, 2, 3, 4, 5, 678_000_000, time.UTC)
	if got := timeISOMillis(ref); got != "2024-01-02T03:04:05.678Z" {
		t.Fatalf("ISOMillis: got %q", got)
	}
	if got := timeYMD(ref); got != "2024-01-02" {
		t.Fatalf("YMD: got %q", got)
	}
	if got := timeEpochMS(ref); got != ref.UnixMilli() {
		t.Fatalf("EpochMS: got %d", got)
	}
}

func TestTimeFormatters_ZeroOmitted(t *testing.T) {
	var zero time.Time
	if got := timeISOMillis(zero); got != "" {
		t.Fatalf("ISOMillis zero: got %q", got)
	}
	if got := timeYMD(zero); got != "" {
		t.Fatalf("YMD zero: got %q", got)
	}
	if got := timeEpochMS(zero); got != 0 {
		t.Fatalf("EpochMS zero: got %d", got)
	}
}

func TestParamBuilder_OmitsZeros(t *testing.T) {
	p := newParamBuilder()
	p.addString("s_empty", "")
	p.addString("s_set", "hello")
	p.addStringList("syms_empty", nil)
	p.addStringList("syms", []string{"AMD", "INTC"})
	p.addInt("z", 0)
	p.addInt("n", 5)
	p.addInt64("z64", 0)
	p.addInt64("n64", 42)
	p.addIntPtr("ip_nil", nil)
	x := 7
	p.addIntPtr("ip_set", &x)
	p.addFloatPtr("fp_nil", nil)
	f := 1.5
	p.addFloatPtr("fp_set", &f)
	p.addBoolPtr("bp_nil", nil)
	b := true
	p.addBoolPtr("bp_set", &b)
	p.addTimeISO("t_zero", time.Time{})
	p.addTimeYMD("d_zero", time.Time{})
	p.addTimeEpochMS("e_zero", time.Time{})

	got := p.values()
	if got == nil {
		t.Fatal("values() should not be nil")
	}
	for _, key := range []string{"s_empty", "syms_empty", "z", "z64", "ip_nil", "fp_nil", "bp_nil", "t_zero", "d_zero", "e_zero"} {
		if _, ok := got[key]; ok {
			t.Fatalf("zero-value key %q should be omitted", key)
		}
	}
	if got.Get("s_set") != "hello" {
		t.Fatalf("s_set: %q", got.Get("s_set"))
	}
	if got.Get("syms") != "AMD,INTC" {
		t.Fatalf("syms: %q", got.Get("syms"))
	}
	if got.Get("n") != "5" {
		t.Fatalf("n: %q", got.Get("n"))
	}
	if got.Get("n64") != "42" {
		t.Fatalf("n64: %q", got.Get("n64"))
	}
	if got.Get("ip_set") != "7" {
		t.Fatalf("ip_set: %q", got.Get("ip_set"))
	}
	if got.Get("fp_set") != "1.5" {
		t.Fatalf("fp_set: %q", got.Get("fp_set"))
	}
	if got.Get("bp_set") != "true" {
		t.Fatalf("bp_set: %q", got.Get("bp_set"))
	}
}

func TestParamBuilder_EmptyReturnsNil(t *testing.T) {
	p := newParamBuilder()
	if p.values() != nil {
		t.Fatal("empty builder should return nil values")
	}
}
