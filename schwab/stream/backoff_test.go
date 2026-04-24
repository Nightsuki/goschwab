package stream

import (
	"testing"
	"time"
)

func TestBackoffDoublingAndCap(t *testing.T) {
	b := newBackoff(2*time.Second, 120*time.Second)
	want := []time.Duration{
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		32 * time.Second,
		64 * time.Second,
		120 * time.Second,
		120 * time.Second,
		120 * time.Second,
	}
	for i, w := range want {
		got := b.Next()
		if got != w {
			t.Errorf("Next()[%d] = %s; want %s", i, got, w)
		}
	}
}

func TestBackoffReset(t *testing.T) {
	b := newBackoff(2*time.Second, 120*time.Second)
	b.Next()
	b.Next()
	b.Next()
	b.Reset()
	if got := b.Next(); got != 2*time.Second {
		t.Errorf("after Reset: Next() = %s; want 2s", got)
	}
}

func TestBackoffDefaults(t *testing.T) {
	b := newBackoff(0, 0)
	if got := b.Next(); got != 2*time.Second {
		t.Errorf("default min Next() = %s; want 2s", got)
	}
	// Double a few times to confirm the 120s cap kicks in.
	for i := 0; i < 10; i++ {
		b.Next()
	}
	if got := b.Next(); got != 120*time.Second {
		t.Errorf("default cap Next() = %s; want 120s", got)
	}
}
