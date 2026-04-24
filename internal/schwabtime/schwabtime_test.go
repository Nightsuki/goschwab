package schwabtime

import (
	"testing"
	"time"
)

func TestISOMillis_RoundTrip(t *testing.T) {
	ref := time.Date(2024, 1, 2, 3, 4, 5, 678_000_000, time.UTC)
	s := ISOMillis(ref)
	if s != "2024-01-02T03:04:05.678Z" {
		t.Fatalf("format: %q", s)
	}
	back, err := ParseISOMillis(s)
	if err != nil {
		t.Fatal(err)
	}
	if !back.Equal(ref) {
		t.Fatalf("roundtrip: got %s want %s", back, ref)
	}
}

func TestISOMillis_ZeroOmitted(t *testing.T) {
	if got := ISOMillis(time.Time{}); got != "" {
		t.Fatalf("got %q", got)
	}
	got, err := ParseISOMillis("")
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsZero() {
		t.Fatalf("expected zero time")
	}
}

func TestYMD_RoundTrip(t *testing.T) {
	ref := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	s := YMD(ref)
	if s != "2024-01-02" {
		t.Fatalf("format: %q", s)
	}
	back, err := ParseYMD(s)
	if err != nil {
		t.Fatal(err)
	}
	if !back.Equal(ref) {
		t.Fatalf("roundtrip: got %s want %s", back, ref)
	}
}

func TestEpochMS_RoundTrip(t *testing.T) {
	ref := time.Date(2024, 1, 2, 3, 4, 5, 678_000_000, time.UTC)
	ms := EpochMS(ref)
	if ms != ref.UnixMilli() {
		t.Fatalf("epoch: got %d want %d", ms, ref.UnixMilli())
	}
	back := ParseEpochMS(ms)
	if !back.Equal(ref) {
		t.Fatalf("roundtrip: got %s want %s", back, ref)
	}
	// Zero stays zero.
	if EpochMS(time.Time{}) != 0 {
		t.Fatal("zero should be 0")
	}
	if !ParseEpochMS(0).IsZero() {
		t.Fatal("parse 0 should be zero")
	}
}

func TestParseEpochMSString(t *testing.T) {
	ref := time.Date(2024, 1, 2, 3, 4, 5, 678_000_000, time.UTC)
	back, err := ParseEpochMSString("1704164645678")
	if err != nil {
		t.Fatal(err)
	}
	if !back.Equal(ref) {
		t.Fatalf("got %s want %s", back, ref)
	}
	empty, err := ParseEpochMSString("")
	if err != nil {
		t.Fatal(err)
	}
	if !empty.IsZero() {
		t.Fatal("empty should be zero")
	}
	if _, err := ParseEpochMSString("not a number"); err == nil {
		t.Fatal("expected error")
	}
}
