package stream

import "time"

// backoff implements an exponential backoff with a minimum and a cap.
// It is not safe for concurrent use; callers must serialize access.
type backoff struct {
	min     time.Duration
	max     time.Duration
	current time.Duration
}

// newBackoff returns a backoff starting at minD and doubling up to maxD.
// If minD <= 0 it defaults to 2s; if maxD <= 0 it defaults to 120s.
func newBackoff(minD, maxD time.Duration) *backoff {
	if minD <= 0 {
		minD = 2 * time.Second
	}
	if maxD <= 0 {
		maxD = 120 * time.Second
	}
	if minD > maxD {
		minD = maxD
	}
	return &backoff{min: minD, max: maxD}
}

// Next returns the next backoff duration. The first call returns min, each
// subsequent call doubles the result up to max.
func (b *backoff) Next() time.Duration {
	if b.current <= 0 {
		b.current = b.min
		return b.current
	}
	next := b.current * 2
	if next > b.max {
		next = b.max
	}
	b.current = next
	return next
}

// Reset returns the backoff to its initial state; the next Next() call
// returns the minimum duration again.
func (b *backoff) Reset() { b.current = 0 }
