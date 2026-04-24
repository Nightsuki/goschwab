package stream

import (
	"context"
	"time"

	"nhooyr.io/websocket"
)

// SetStreamerURLForTest overrides the streamer URL that would otherwise be
// returned by GetUserPreferences. Test-only.
func SetStreamerURLForTest(s *Streamer, url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.streamerURLOverride = url
}

// SetDialHookForTest installs a custom WebSocket dial function. Test-only.
func SetDialHookForTest(s *Streamer, fn func(ctx context.Context, url string) (*websocket.Conn, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dialHook = fn
}

// SetNowForTest overrides the clock used for stable-connection gating. Test-only.
func SetNowForTest(s *Streamer, fn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nowFn = fn
}

// StableConnectionThresholdForTest exposes the stable-connection threshold
// constant for tests.
const StableConnectionThresholdForTest = stableConnectionThreshold

// SendQueueCapacityForTest exposes the send queue capacity for tests.
const SendQueueCapacityForTest = sendQueueCapacity
