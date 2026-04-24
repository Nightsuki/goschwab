// Package stream provides a WebSocket client for the Charles Schwab Streamer API.
//
// # Basic Usage
//
//	s := stream.New(restClient)
//	s.Subscribe(stream.LevelOneEquities, []string{"AAPL", "AMD"})
//	s.Start(ctx)
//	defer s.Stop()
//
// # Subscriptions
//
// Available services (subscribe to any combination):
//
//	- LevelOneEquities, LevelOneOptions, LevelOneFutures, LevelOneFuturesOptions, LevelOneForex
//	- NYSEBook, NasdaqBook, OptionsBook
//	- ChartEquity, ChartFutures
//	- ScreenerEquity, ScreenerOption
//	- AccountActivity
//
// Subscribe and unsubscribe at any time while the streamer is running:
//
//	s.Subscribe(stream.LevelOneEquities, []string{"SPY"})
//	s.Unsubscribe(stream.LevelOneEquities, []string{"AAPL"})
//
// # Message Handling
//
// Register a handler for parsed messages:
//
//	s := stream.New(c, stream.WithTypedHandler(func(msg *stream.Message) {
//		log.Printf("Quote: %v", msg)
//	}))
//
// Or handle raw text frames:
//
//	s := stream.New(c, stream.WithHandler(func(data []byte) {
//		log.Printf("Raw: %s", data)
//	}))
//
// # Resilience
//
// The streamer automatically reconnects on network failure with exponential
// backoff (default 2s initial, capped at 120s). All subscriptions are replayed
// after reconnection.
//
// Customize backoff and ping intervals:
//
//	s := stream.New(c,
//		stream.WithInitialBackoff(1*time.Second),
//		stream.WithMaxBackoff(60*time.Second),
//		stream.WithPingInterval(15*time.Second),
//	)
//
// # Concurrency
//
// A Streamer is not safe for concurrent Start/Stop, but Send, Subscribe,
// Unsubscribe, Subscriptions, and Active are concurrent-safe and may be
// called from any goroutine after construction.
package stream
