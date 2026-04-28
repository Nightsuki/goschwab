// Package schwab is a Go client for the Charles Schwab Trader and Market Data APIs.
//
// The main entry point is [NewClient], which creates a concurrent-safe REST client
// with automatic OAuth2 token management and optional encryption at rest.
//
// # Basic Usage
//
//	ctx := context.Background()
//	c, err := NewClient(ctx, appKey, appSecret)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer c.Close()
//
//	quotes, err := c.GetQuotes(ctx, []string{"AAPL", "AMD"})
//
// # Market Data
//
// Fetch quotes, option chains, price history, and market hours:
//
//	quote, err := c.GetQuote(ctx, "AAPL")
//	chain, err := c.GetOptionChain(ctx, OptionChainRequest{Symbol: "SPY"})
//	history, err := c.GetPriceHistory(ctx, PriceHistoryRequest{Symbol: "AAPL", PeriodType: "day"})
//
// # Trading
//
// List accounts, place orders, and manage positions:
//
//	accounts, err := c.ListLinkedAccounts(ctx)
//	orders, err := c.ListOrders(ctx, OrderListRequest{From: time.Now().AddDate(0, 0, -1), To: time.Now()})
//	orderID, err := c.PlaceOrder(ctx, accountHash, &Order{...})
//
// # WebSocket Streaming
//
// Real-time market data and account activity are available via the [stream] subpackage.
//
//	s := stream.New(c)
//	s.Subscribe(stream.LevelOneEquities, []string{"AAPL"})
//	s.Start(ctx)
//	defer s.Stop()
//
// # Authentication and Token Storage
//
// On first run, [NewClient] prompts for OAuth2 authorization in the browser.
// Tokens are persisted to ~/.schwab/tokens.json by default and automatically refreshed.
//
// Customize token storage and encryption:
//
//	c, err := NewClient(ctx, key, secret,
//		WithTokenPath("/tmp/tokens.json"),
//		WithEncryptionKey(myKey),
//		WithAuthorizer(myCustomAuthorizer),
//	)
//
// # Concurrent-safe Refresh
//
// Refresh attempts are deduplicated within a single Client via singleflight,
// so concurrent goroutines hitting an expired access token issue at most one
// HTTP refresh and share the result. Before each refresh the token store is
// re-loaded so a peer process that has already rotated the token is observed
// and its rotated value adopted (avoiding invalid_grant on a revoked
// refresh_token).
//
// # Multi-process Deployments
//
// When several processes share the same TokenStore (e.g. a shared file, a
// Redis-backed store) the optional [TokenStoreLocker] interface coordinates
// refreshes across processes. The bundled [NewFileTokenStore] satisfies it
// on Unix via POSIX flock; external stores (Redis, Consul, etcd, etc.) can
// implement it themselves to participate. Stores that do not implement
// [TokenStoreLocker] still work — refresh is then only deduplicated within
// each process.
//
// # Error Handling
//
// Use [errors.Is] and [errors.As] to inspect errors:
//
//	if errors.Is(err, ErrRateLimited) {
//		log.Println("Rate limited; retry later")
//	}
//	var apiErr *APIError
//	if errors.As(err, &apiErr) {
//		log.Printf("API error (%d): %s", apiErr.Code, apiErr.Message)
//	}
//
// See the package-level docs on [pkg.go.dev] for complete API documentation.
//
// [pkg.go.dev]: https://pkg.go.dev/github.com/Nightsuki/goschwab/schwab
package schwab
