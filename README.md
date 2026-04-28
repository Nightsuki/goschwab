# goschwab

A Go client for the Charles Schwab Trader API and Market Data API, ported from [tylerebowers/Schwabdev](https://github.com/tylerebowers/Schwabdev).

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-blue?style=flat-square)](https://golang.org/dl/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](LICENSE)

## Features

- **OAuth2 3-legged flow** — automatic token refresh with browser-based authorization
- **Pluggable token storage** — JSON file (default) or custom backend; optional AES-256-GCM encryption at rest
- **Concurrent-safe refresh** — singleflight deduplication in-process; optional `TokenStoreLocker` interface for cross-process coordination (POSIX flock built into the file store on Unix)
- **Full REST parity** — complete market data and trading endpoint coverage
- **Resilient WebSocket streamer** — automatic reconnection with exponential backoff and subscription replay
- **Idiomatic Go** — context-aware, `errors.Is/As` error handling, concurrent-safe client
- **Zero CGO by default** — pure Go build; optional SQLite support available via build tag

## Status

**Alpha.** Port of tylerebowers/Schwabdev. Public API may change before v1.0.

## Install

```
go get github.com/Nightsuki/goschwab
```

Requires Go 1.22+.

## Quickstart

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/Nightsuki/goschwab/schwab"
)

func main() {
	ctx := context.Background()

	// NewClient handles OAuth2 on first run: opens browser, prompts for redirect URL on stdin.
	// Tokens are persisted to ~/.schwab/tokens.json.
	c, err := schwab.NewClient(
		ctx,
		os.Getenv("SCHWAB_APP_KEY"),
		os.Getenv("SCHWAB_APP_SECRET"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// Fetch quotes
	quotes, err := c.GetQuotes(ctx, []string{"AAPL", "AMD"})
	if err != nil {
		log.Fatal(err)
	}

	for symbol, quote := range quotes {
		log.Printf("%s: $%.2f", symbol, quote.Mark)
	}
}
```

On first run, `NewClient` opens your default browser to the Schwab authorize URL. Paste the redirect URL into the stdin prompt. Tokens are persisted to `~/.schwab/tokens.json` by default and automatically refreshed before expiration on subsequent calls.

## Authentication

### 3-Legged OAuth2 Flow

Schwab uses a 3-legged OAuth2 flow:

1. **First run:** `NewClient` generates an authorization URL and opens it in your browser.
2. **User authorization:** You grant permission in the browser, which redirects to a callback URL (e.g., `https://127.0.0.1`).
3. **Token exchange:** Copy the full redirect URL and paste it into the stdin prompt.
4. **Persistence:** Access and refresh tokens are stored locally (default: `~/.schwab/tokens.json`).
5. **Auto-refresh:** Before each API request, the client automatically refreshes the access token if its TTL is low.

### Custom Token Storage and Encryption

Store tokens in a different location or encrypt them at rest:

```go
// Use custom token path
c, err := schwab.NewClient(
	ctx,
	key, secret,
	schwab.WithTokenPath("/tmp/my-tokens.json"),
)

// Encrypt tokens with a 32-byte key
encryptionKey := []byte("my-secret-key-must-be-32-bytes!")
c, err := schwab.NewClient(
	ctx,
	key, secret,
	schwab.WithEncryptionKey(encryptionKey),
)
```

### Multi-Process Deployments

If several processes share the same `TokenStore` (a shared file path, a Redis-backed implementation, etc.), refreshes need to be coordinated across processes — otherwise two peers may simultaneously consume the same `refresh_token` and one will fail with `invalid_grant` (Schwab rotates the refresh token on every grant).

goschwab handles this with the optional `TokenStoreLocker` interface:

```go
type TokenStoreLocker interface {
    AcquireRefreshLock(ctx context.Context) (release func(), err error)
}
```

**Built-in support:** `NewFileTokenStore` implements `TokenStoreLocker` on Unix using POSIX `flock` on a sibling `<path>.lock` file. Multiple processes pointing at the same token file automatically serialize refreshes — no extra configuration required.

**External stores:** Backends like Redis, Consul, etcd, or a SQL advisory lock can opt in by implementing the interface. Stores that don't implement it still work; in that case refreshes are only deduplicated within each process (occasional `invalid_grant` losses on peer rotation are recoverable on the next refresh cycle).

Implementation contract (documented on the interface):

- `AcquireRefreshLock` blocks until the lock is acquired or `ctx` expires.
- TTL must exceed the refresh HTTP timeout so a crashed peer doesn't deadlock survivors.
- The returned `release` is idempotent and safe to call from multiple goroutines.
- `ctx` errors propagate verbatim so callers can distinguish lock-wait timeouts from auth failures.

In-process, refreshes are always deduplicated via `singleflight` regardless of store type — N concurrent goroutines triggering an expired-token refresh produce exactly one HTTP call and share the result.

### Headless / Custom Authorization

For headless environments or custom UI, implement the `Authorizer` interface:

```go
type MyAuthorizer struct{}

func (a *MyAuthorizer) Authorize(ctx context.Context, url string) (string, error) {
	// Custom logic: send URL to frontend, wait for callback, etc.
	return callbackURL, nil
}

c, err := schwab.NewClient(
	ctx,
	key, secret,
	schwab.WithAuthorizer(&MyAuthorizer{}),
)
```

## Market Data Endpoints

| Method | Signature | Endpoint |
|--------|-----------|----------|
| `GetQuotes` | `(ctx, symbols []string, opts ...QuoteOption) (map[string]Quote, error)` | `GET /marketdata/v1/quotes` |
| `GetQuote` | `(ctx, symbol string, opts ...QuoteOption) (*Quote, error)` | `GET /marketdata/v1/{symbol}/quotes` |
| `GetOptionChain` | `(ctx, req OptionChainRequest) (*OptionChain, error)` | `GET /marketdata/v1/chains` |
| `GetOptionExpirationChain` | `(ctx, symbol string) (*ExpirationChain, error)` | `GET /marketdata/v1/expirationchain` |
| `GetPriceHistory` | `(ctx, req PriceHistoryRequest) (*PriceHistory, error)` | `GET /marketdata/v1/pricehistory` |
| `GetMovers` | `(ctx, symbol string, opts ...MoverOption) (*Movers, error)` | `GET /marketdata/v1/movers/{symbol}` |
| `GetMarketHours` | `(ctx, markets []string, date time.Time) (*MarketHours, error)` | `GET /marketdata/v1/markets` |
| `GetMarketHour` | `(ctx, market string, date time.Time) (*MarketHours, error)` | `GET /marketdata/v1/markets/{market}` |
| `GetInstruments` | `(ctx, symbols []string, projection string) (*Instruments, error)` | `GET /marketdata/v1/instruments` |
| `GetInstrumentByCUSIP` | `(ctx, cusip string) (*Instrument, error)` | `GET /marketdata/v1/instruments/{cusip}` |

## Trading Endpoints

| Method | Signature | Endpoint |
|--------|-----------|----------|
| `ListLinkedAccounts` | `(ctx) ([]LinkedAccount, error)` | `GET /trader/v1/accounts/accountNumbers` |
| `GetAllAccounts` | `(ctx, fields string) ([]Account, error)` | `GET /trader/v1/accounts/` |
| `GetAccount` | `(ctx, accountHash, fields string) (*Account, error)` | `GET /trader/v1/accounts/{accountHash}` |
| `ListOrders` | `(ctx, req OrderListRequest) ([]Order, error)` | `GET /trader/v1/orders` |
| `ListAccountOrders` | `(ctx, accountHash string, req OrderListRequest) ([]Order, error)` | `GET /trader/v1/accounts/{accountHash}/orders` |
| `PlaceOrder` | `(ctx, accountHash string, order *Order) (orderID string, error)` | `POST /trader/v1/accounts/{accountHash}/orders` |
| `PreviewOrder` | `(ctx, accountHash string, order *Order) (*OrderPreview, error)` | `POST /trader/v1/accounts/{accountHash}/previewOrder` |
| `GetOrder` | `(ctx, accountHash, orderID string) (*Order, error)` | `GET /trader/v1/accounts/{accountHash}/orders/{orderID}` |
| `CancelOrder` | `(ctx, accountHash, orderID string) error` | `DELETE /trader/v1/accounts/{accountHash}/orders/{orderID}` |
| `ReplaceOrder` | `(ctx, accountHash, orderID string, order *Order) (newOrderID string, error)` | `PUT /trader/v1/accounts/{accountHash}/orders/{orderID}` |
| `ListTransactions` | `(ctx, accountHash string, req TransactionsRequest) ([]Transaction, error)` | `GET /trader/v1/accounts/{accountHash}/transactions` |
| `GetTransaction` | `(ctx, accountHash, transactionID string) (*Transaction, error)` | `GET /trader/v1/accounts/{accountHash}/transactions/{transactionID}` |
| `GetUserPreferences` | `(ctx) (*UserPreferences, error)` | `GET /trader/v1/userPreference` |

## Streaming

Subscribe to real-time market data and account activity via WebSocket:

```go
import "github.com/Nightsuki/goschwab/schwab/stream"

// Create a streamer from your REST client
s := stream.New(c)

// Define a handler for parsed messages
s.Subscribe(stream.LevelOneEquities, []string{"AAPL", "AMD"})

if err := s.Start(ctx); err != nil {
	log.Fatal(err)
}
defer s.Stop()

// Handler receives updates in a background goroutine
// (see examples/stream_equities/main.go for a complete example)
```

### Available Services

| Service | Description |
|---------|-------------|
| `LevelOneEquities` | Real-time quotes for stocks |
| `LevelOneOptions` | Real-time option quotes |
| `LevelOneFutures` | Real-time futures quotes |
| `LevelOneFuturesOptions` | Real-time futures option quotes |
| `LevelOneForex` | Real-time forex quotes |
| `NYSEBook` | NYSE Level 2 book |
| `NasdaqBook` | NASDAQ Level 2 book |
| `OptionsBook` | Options Level 2 book |
| `ChartEquity` | Equity chart updates |
| `ChartFutures` | Futures chart updates |
| `ScreenerEquity` | Equity screener updates |
| `ScreenerOption` | Option screener updates |
| `AccountActivity` | Account trade confirmations and activity |

The streamer is concurrent-safe: `Subscribe`, `Unsubscribe`, and `Send` can be called from any goroutine while the streamer is running.

## Error Handling

Use `errors.Is` and `errors.As` for error inspection:

```go
quotes, err := c.GetQuotes(ctx, []string{"AAPL"})
if err != nil {
	// Check for rate limit
	if errors.Is(err, schwab.ErrRateLimited) {
		log.Println("Rate limited; retry later")
	}

	// Check for auth errors
	var authErr *schwab.AuthError
	if errors.As(err, &authErr) {
		log.Printf("Auth failed: %v", authErr)
	}

	// Check for API errors
	var apiErr *schwab.APIError
	if errors.As(err, &apiErr) {
		log.Printf("API error (%d): %s", apiErr.Code, apiErr.Message)
	}

	log.Fatal(err)
}
```

## Configuration

All client options are passed to `NewClient`:

| Option | Purpose |
|--------|---------|
| `WithCallbackURL(url)` | Override the OAuth callback URL (default: `https://127.0.0.1`) |
| `WithTokenStore(store)` | Provide a custom `TokenStore` implementation |
| `WithTokenPath(path)` | Override the default token file location (`~/.schwab/tokens.json`) |
| `WithEncryptionKey(key)` | Enable AES-256-GCM encryption for tokens at rest |
| `WithHTTPClient(hc)` | Use a custom `*http.Client` for API requests |
| `WithTimeout(duration)` | Set per-request timeout (default: 10 seconds) |
| `WithLogger(logger)` | Provide a custom `*slog.Logger` for diagnostics |
| `WithAuthorizer(auth)` | Use a custom `Authorizer` for OAuth flow |
| `WithBaseURL(url)` | Override the Schwab API base URL (for testing) |

## Examples

Complete working examples are in the `examples/` directory:

- `examples/quotes/` — fetch and display quotes
- `examples/chain/` — fetch option chains
- `examples/pricehistory/` — fetch OHLCV candles
- `examples/place_order/` — place a trading order
- `examples/stream_equities/` — stream real-time equity quotes
- `examples/stream_account_activity/` — stream account activity

Run an example:

```
cd examples/quotes
go run main.go
```

## Differences from Schwabdev (Python)

| Aspect | Schwabdev | goschwab |
|--------|-----------|----------|
| **Client design** | `Client` (sync) + `ClientAsync` (async) | Single `*Client` (concurrent, context-aware) |
| **Token storage** | SQLite at `~/.schwabdev/tokens.db` | JSON at `~/.schwab/tokens.json` (pluggable) |
| **Token encryption** | Fernet (Python-specific) | AES-256-GCM (standard, PBKDF2 key derivation) |
| **Error handling** | Python exceptions | `errors.Is/As` with typed sentinel errors |
| **WebSocket** | `websockets` library | `nhooyr.io/websocket` (thin, pure Go) |
| **Authorization UI** | Blocking `input()` on stdin | `Authorizer` interface (headless-friendly) |
| **CGO** | Not required | Not required (default); optional SQLite support |

## Testing

Run the full test suite:

```
cd /Users/test/Codes/goschwab
GOTOOLCHAIN=local go test ./...
```

Verify no build or vet errors:

```
GOTOOLCHAIN=local go vet ./...
GOTOOLCHAIN=local go build ./...
```

## License

MIT. See [LICENSE](LICENSE).

## Credits

goschwab is a port of [tylerebowers/Schwabdev](https://github.com/tylerebowers/Schwabdev), the reference implementation of the Charles Schwab Trader API in Python. All architectural and design decisions are informed by Schwabdev's public API.
