package stream

import "testing"

func TestFieldNamesKnownServices(t *testing.T) {
	cases := []struct {
		svc      Service
		first    string
		minCount int
	}{
		{LevelOneEquities, "Symbol", 40},
		{LevelOneOptions, "Symbol", 40},
		{LevelOneFutures, "Symbol", 30},
		{LevelOneForex, "Symbol", 20},
		{ChartEquity, "key", 9},
		{ChartFutures, "key", 7},
		{AccountActivity, "Subscription Key", 4},
	}
	for _, tc := range cases {
		got := FieldNames(tc.svc)
		if len(got) < tc.minCount {
			t.Errorf("%s: got %d fields; want >= %d", tc.svc, len(got), tc.minCount)
			continue
		}
		if got[0] != tc.first {
			t.Errorf("%s: first = %q; want %q", tc.svc, got[0], tc.first)
		}
	}
}

func TestFieldNamesChartEquityOrdering(t *testing.T) {
	// Spec Appendix C#1 requires this exact order.
	want := []string{"key", "Sequence", "Open Price", "High Price", "Low Price", "Close Price", "Volume", "Chart Time", "Chart Day"}
	got := FieldNames(ChartEquity)
	if len(got) != len(want) {
		t.Fatalf("ChartEquity fields len=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ChartEquity[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestFieldNamesUnknownReturnsNil(t *testing.T) {
	got := FieldNames(Service("NOT_A_SERVICE"))
	if got != nil {
		t.Errorf("unknown service: got %v; want nil", got)
	}
}

func TestFieldNamesDefensiveCopy(t *testing.T) {
	a := FieldNames(LevelOneEquities)
	a[0] = "MUTATED"
	b := FieldNames(LevelOneEquities)
	if b[0] == "MUTATED" {
		t.Errorf("FieldNames did not return a defensive copy: b[0]=%q", b[0])
	}
}
