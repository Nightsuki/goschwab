// Package schwab is a Go client for the Charles Schwab Trader and Market
// Data APIs. The main entry point is NewClient, which returns a *Client
// configured with an OAuth2 token store, an HTTP client, and a logger.
//
// The client is safe for concurrent use; tokens are refreshed lazily before
// every request.
package schwab

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client is the Schwab REST client. Safe for concurrent use.
type Client struct {
	cfg        clientConfig
	appKey     string
	appSecret  string
	httpClient *http.Client
	logger     *slog.Logger

	mu    sync.Mutex
	token *Token
}

// DefaultTimeout is the default per-request timeout.
const DefaultTimeout = 10 * time.Second

// NewClient creates a new Schwab API client.
//
// appKey / appSecret are the credentials from developer.schwab.com. On first
// construction, if no valid refresh token is persisted, the configured
// Authorizer is invoked (default: open browser + prompt on stdin).
func NewClient(ctx context.Context, appKey, appSecret string, opts ...Option) (*Client, error) {
	cfg := clientConfig{
		callbackURL: defaultCallbackURL,
		timeout:     DefaultTimeout,
		baseURL:     defaultBaseURL,
	}
	for _, o := range opts {
		o(&cfg)
	}

	if err := validateAppKey(appKey); err != nil {
		return nil, err
	}
	if err := validateAppSecret(appSecret); err != nil {
		return nil, err
	}
	if err := validateCallbackURL(cfg.callbackURL); err != nil {
		return nil, err
	}

	if cfg.logger == nil {
		cfg.logger = slog.Default()
	}
	if cfg.authorizer == nil {
		cfg.authorizer = &BrowserAuthorizer{}
	}
	if cfg.httpClient == nil {
		cfg.httpClient = &http.Client{Timeout: cfg.timeout}
	} else if cfg.httpClient.Timeout == 0 && cfg.timeout > 0 {
		// Respect user-provided client, but set a timeout if none.
		hc := *cfg.httpClient
		hc.Timeout = cfg.timeout
		cfg.httpClient = &hc
	}
	if cfg.tokenStore == nil {
		path := cfg.tokenPath
		if path == "" {
			p, err := defaultTokenPath()
			if err != nil {
				return nil, err
			}
			path = p
		}
		var fsOpts []FileStoreOption
		if len(cfg.encryptionKey) > 0 {
			fsOpts = append(fsOpts, WithFileEncryption(cfg.encryptionKey))
		}
		store, err := NewFileTokenStore(path, fsOpts...)
		if err != nil {
			return nil, err
		}
		cfg.tokenStore = store
	}

	c := &Client{
		cfg:        cfg,
		appKey:     appKey,
		appSecret:  appSecret,
		httpClient: cfg.httpClient,
		logger:     cfg.logger,
	}

	// Perform an initial token refresh / load so that ErrNoToken surfaces
	// before the first API call.
	if _, err := c.UpdateTokens(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// Close flushes outstanding state and releases backing stores. It is safe to
// call more than once.
func (c *Client) Close() error {
	c.mu.Lock()
	store := c.cfg.tokenStore
	tok := c.token
	c.mu.Unlock()
	if store == nil {
		return nil
	}
	if tok != nil {
		// Best-effort final flush; ignore errors beyond logging.
		if err := store.Save(context.Background(), tok); err != nil {
			c.logger.Warn("schwab: token flush on close failed", "error", err)
		}
	}
	return store.Close()
}

// validateAppKey enforces the length constraints documented by Schwabdev.
func validateAppKey(k string) error {
	switch len(k) {
	case 32, 48:
		return nil
	default:
		return fmt.Errorf("schwab: app key length must be 32 or 48 (got %d)", len(k))
	}
}

// validateAppSecret enforces the length constraints documented by Schwabdev.
func validateAppSecret(s string) error {
	switch len(s) {
	case 16, 64:
		return nil
	default:
		return fmt.Errorf("schwab: app secret length must be 16 or 64 (got %d)", len(s))
	}
}

// validateCallbackURL enforces HTTPS + no trailing slash.
func validateCallbackURL(u string) error {
	if u == "" {
		return errors.New("schwab: callback URL must not be empty")
	}
	if !strings.HasPrefix(u, "https://") {
		return fmt.Errorf("schwab: callback URL must begin with https:// (got %q)", u)
	}
	if strings.HasSuffix(u, "/") {
		return fmt.Errorf("schwab: callback URL must not end with '/' (got %q)", u)
	}
	return nil
}

// accessToken returns the current access token under the mutex.
func (c *Client) accessToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == nil {
		return ""
	}
	return c.token.AccessToken
}

// AccessToken returns the current OAuth2 access token. It does NOT refresh
// the token; callers that need a guaranteed-fresh token should call
// UpdateTokens(ctx) first. Returns the empty string when no token is loaded.
//
// This accessor exists primarily for the streamer sub-package, which must
// include the bearer token in its LOGIN frame.
func (c *Client) AccessToken() string { return c.accessToken() }

// CurrentToken refreshes tokens if needed and returns a defensive copy of
// the current Token. The returned pointer is safe for the caller to read or
// modify; mutations do not propagate back to the client.
func (c *Client) CurrentToken(ctx context.Context) (*Token, error) {
	if _, err := c.UpdateTokens(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == nil {
		return nil, ErrNoToken
	}
	clone := *c.token
	return &clone, nil
}

// Logger returns the client's slog.Logger. Useful for sub-packages that want
// to emit structured logs sharing the client's logger.
func (c *Client) Logger() *slog.Logger { return c.logger }

// do performs a single HTTP request to the Schwab REST API. It refreshes
// tokens lazily, applies the Authorization header, marshals a JSON body when
// present, and decodes the 2xx response into out (when non-nil).
//
// 401 responses trigger exactly one refresh-and-retry attempt to cover the
// window between our expiry estimate and the server's actual revocation.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	return c.doImpl(ctx, method, path, query, body, out, false)
}

// doImpl is the recursive core of do, bounded by the "retried" flag.
func (c *Client) doImpl(ctx context.Context, method, path string, query url.Values, body any, out any, retried bool) error {
	if _, err := c.UpdateTokens(ctx); err != nil {
		return err
	}

	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("schwab: marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	fullURL := c.cfg.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return fmt.Errorf("schwab: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken())
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("schwab: http: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("schwab: read response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		if retried {
			return &AuthError{Stage: "refresh", Err: fmt.Errorf("http 401 after refresh: %s", strings.TrimSpace(string(respBody)))}
		}
		if err := c.doRefresh(ctx); err != nil {
			return err
		}
		return c.doImpl(ctx, method, path, query, body, out, true)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		apiErr := newAPIError(resp, respBody)
		var ra time.Duration
		if s := resp.Header.Get("Retry-After"); s != "" {
			if secs, perr := strconv.Atoi(strings.TrimSpace(s)); perr == nil {
				ra = time.Duration(secs) * time.Second
			} else if when, perr2 := http.ParseTime(s); perr2 == nil {
				ra = time.Until(when)
				if ra < 0 {
					ra = 0
				}
			}
		}
		return &RateLimitError{APIError: *apiErr, RetryAfter: ra}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newAPIError(resp, respBody)
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("schwab: unmarshal response: %w", err)
		}
	}
	return nil
}

// doRaw performs an HTTP request like do but returns the full *http.Response
// so that callers can inspect response headers (e.g. Location) before closing
// the body. The caller is responsible for closing resp.Body.
//
// On non-2xx status codes (except 401 which is retried once) doRaw returns a
// nil response and a typed error consistent with do.
func (c *Client) doRaw(ctx context.Context, method, path string, query url.Values, body any) (*http.Response, error) {
	return c.doRawImpl(ctx, method, path, query, body, false)
}

// doRawImpl is the recursive core of doRaw, bounded by the "retried" flag.
func (c *Client) doRawImpl(ctx context.Context, method, path string, query url.Values, body any, retried bool) (*http.Response, error) {
	if _, err := c.UpdateTokens(ctx); err != nil {
		return nil, err
	}

	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("schwab: marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	fullURL := c.cfg.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("schwab: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken())
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("schwab: http: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		if retried {
			return nil, &AuthError{Stage: "refresh", Err: fmt.Errorf("http 401 after refresh")}
		}
		if err := c.doRefresh(ctx); err != nil {
			return nil, err
		}
		return c.doRawImpl(ctx, method, path, query, body, true)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		apiErr := newAPIError(resp, respBody)
		var ra time.Duration
		if s := resp.Header.Get("Retry-After"); s != "" {
			if secs, perr := strconv.Atoi(strings.TrimSpace(s)); perr == nil {
				ra = time.Duration(secs) * time.Second
			} else if when, perr2 := http.ParseTime(s); perr2 == nil {
				ra = time.Until(when)
				if ra < 0 {
					ra = 0
				}
			}
		}
		return nil, &RateLimitError{APIError: *apiErr, RetryAfter: ra}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, newAPIError(resp, respBody)
	}

	return resp, nil
}

// newAPIError assembles an *APIError from a response + body.
func newAPIError(resp *http.Response, body []byte) *APIError {
	e := &APIError{
		StatusCode: resp.StatusCode,
		Raw:        append(json.RawMessage(nil), body...),
		RequestID:  resp.Header.Get("schwab-client-correl-id"),
	}
	// Best-effort body parse: Schwab error bodies usually have {"errors":[{"message":...,"code":...}]}
	// or {"message":...,"error":...}. Tolerate both.
	var generic struct {
		Message string `json:"message"`
		Error   string `json:"error"`
		Code    string `json:"code"`
		Errors  []struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"errors"`
	}
	if len(body) > 0 && json.Unmarshal(body, &generic) == nil {
		switch {
		case generic.Message != "":
			e.Message = generic.Message
		case generic.Error != "":
			e.Message = generic.Error
		case len(generic.Errors) > 0:
			e.Message = generic.Errors[0].Message
			if generic.Errors[0].Code != "" {
				e.Code = generic.Errors[0].Code
			}
		}
		if e.Code == "" && generic.Code != "" {
			e.Code = generic.Code
		}
	}
	if e.Message == "" {
		e.Message = strings.TrimSpace(string(body))
	}
	return e
}
