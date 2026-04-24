package schwab

import (
	"log/slog"
	"net/http"
	"time"
)

// Option configures a Client. Apply options with NewClient(..., opts...).
type Option func(*clientConfig)

// clientConfig collects the tunables resolved by the Option helpers.
// It is purely internal; users only ever touch it via Option values.
type clientConfig struct {
	callbackURL    string
	tokenStore     TokenStore
	tokenPath      string
	encryptionKey  []byte
	httpClient     *http.Client
	timeout        time.Duration
	logger         *slog.Logger
	authorizer     Authorizer
	baseURL string
}

// WithCallbackURL overrides the OAuth callback URL. Default: "https://127.0.0.1".
// The URL must begin with "https://" and must not have a trailing slash.
func WithCallbackURL(url string) Option {
	return func(c *clientConfig) { c.callbackURL = url }
}

// WithTokenStore provides a custom token persistence backend.
// Default: JSON file store at ~/.schwab/tokens.json.
func WithTokenStore(store TokenStore) Option {
	return func(c *clientConfig) { c.tokenStore = store }
}

// WithTokenPath overrides the default JSON token file path.
// Has no effect when WithTokenStore is also provided.
func WithTokenPath(path string) Option {
	return func(c *clientConfig) { c.tokenPath = path }
}

// WithEncryptionKey enables AES-256-GCM token encryption at rest. The key may
// be any length; it is HKDF-SHA256 expanded to 32 bytes internally.
// Has no effect when WithTokenStore is also provided.
func WithEncryptionKey(key []byte) Option {
	return func(c *clientConfig) {
		c.encryptionKey = append([]byte(nil), key...)
	}
}

// WithHTTPClient injects a custom *http.Client (for proxies, retries, tests).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *clientConfig) { c.httpClient = hc }
}

// WithTimeout sets the per-request timeout (default 10 s).
func WithTimeout(d time.Duration) Option {
	return func(c *clientConfig) { c.timeout = d }
}

// WithLogger injects an *slog.Logger. Default: slog.Default().
func WithLogger(l *slog.Logger) Option {
	return func(c *clientConfig) { c.logger = l }
}

// WithAuthorizer overrides the interactive OAuth flow. Useful for headless
// environments, test fixtures, and GUI integrations.
func WithAuthorizer(a Authorizer) Option {
	return func(c *clientConfig) { c.authorizer = a }
}

// WithBaseURL overrides the REST base URL (testing only).
func WithBaseURL(url string) Option {
	return func(c *clientConfig) { c.baseURL = url }
}
