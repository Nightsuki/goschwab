package schwab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetQuotes_CSVEncoding(t *testing.T) {
	var gotPath, gotQuery, gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "T", "expires_in": 1800})
			return
		}
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"AMD":{"symbol":"AMD","assetMainType":"EQUITY"},"INTC":{"symbol":"INTC","assetMainType":"EQUITY"}}`))
	}))
	defer srv.Close()

	store := NewMemoryTokenStore()
	now := time.Now().UTC()
	_ = store.Save(context.Background(), &Token{
		AccessToken:        "TEST-TOKEN",
		RefreshToken:       "RT",
		ExpiresIn:          1800,
		AccessTokenIssued:  now,
		RefreshTokenIssued: now,
	})
	c, err := NewClient(context.Background(), testAppKey32, testAppSecret16,
		WithCallbackURL(validCallback),
		WithTokenStore(store),
		WithBaseURL(srv.URL),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()

	quotes, err := c.GetQuotes(context.Background(), []string{"AMD", "INTC"})
	if err != nil {
		t.Fatalf("GetQuotes: %v", err)
	}

	// Verify path
	if gotPath != "/marketdata/v1/quotes" {
		t.Errorf("path: got %q want %q", gotPath, "/marketdata/v1/quotes")
	}

	// Verify symbols is CSV (single key, not repeated)
	if !strings.Contains(gotQuery, "symbols=AMD%2CINTC") && !strings.Contains(gotQuery, "symbols=AMD,INTC") {
		t.Errorf("query %q does not contain CSV symbols", gotQuery)
	}
	// Must NOT have two separate symbols= keys
	if strings.Count(gotQuery, "symbols=") != 1 {
		t.Errorf("query has multiple symbols keys: %q", gotQuery)
	}

	// Verify Authorization header
	if gotAuth != "Bearer TEST-TOKEN" {
		t.Errorf("auth: got %q want %q", gotAuth, "Bearer TEST-TOKEN")
	}

	// Verify response decoded
	if len(quotes) != 2 {
		t.Errorf("quotes count: got %d want 2", len(quotes))
	}
	if q, ok := quotes["AMD"]; !ok || q.AssetMainType != "EQUITY" {
		t.Errorf("AMD quote: %+v", quotes["AMD"])
	}
}

func TestGetQuotes_WithFields(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "T", "expires_in": 1800})
			return
		}
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"AAPL":{"symbol":"AAPL"}}`))
	}))
	defer srv.Close()

	store := NewMemoryTokenStore()
	now := time.Now().UTC()
	_ = store.Save(context.Background(), &Token{
		AccessToken: "T", RefreshToken: "RT", ExpiresIn: 1800,
		AccessTokenIssued: now, RefreshTokenIssued: now,
	})
	c, err := NewClient(context.Background(), testAppKey32, testAppSecret16,
		WithCallbackURL(validCallback), WithTokenStore(store), WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()

	_, err = c.GetQuotes(context.Background(), []string{"AAPL"}, WithQuoteFields(QuoteFieldsQuote))
	if err != nil {
		t.Fatalf("GetQuotes: %v", err)
	}
	// fields must be lowercase
	if !strings.Contains(gotQuery, "fields=quote") {
		t.Errorf("query %q missing fields=quote", gotQuery)
	}
}

func TestGetQuotes_EmptySymbols(t *testing.T) {
	// Seed a valid token so NewClient doesn't trigger the browser authorizer.
	store := NewMemoryTokenStore()
	now := time.Now().UTC()
	_ = store.Save(context.Background(), &Token{
		AccessToken: "T", RefreshToken: "RT", ExpiresIn: 1800,
		AccessTokenIssued: now, RefreshTokenIssued: now,
	})
	c, err := NewClient(context.Background(), testAppKey32, testAppSecret16,
		WithCallbackURL(validCallback), WithTokenStore(store))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()

	_, err = c.GetQuotes(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for empty symbols")
	}
}

func TestGetQuote_PathEscape(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "T", "expires_in": 1800})
			return
		}
		// Use RawPath when available (set by Go's net/http when percent-encoding is present)
		raw := r.URL.RawPath
		if raw == "" {
			raw = r.URL.Path
		}
		gotPath = raw
		w.Header().Set("Content-Type", "application/json")
		// Return with the original key so GetQuote can find it
		_, _ = w.Write([]byte(`{"BRK/B":{"symbol":"BRK/B","assetMainType":"EQUITY"}}`))
	}))
	defer srv.Close()

	store := NewMemoryTokenStore()
	now := time.Now().UTC()
	_ = store.Save(context.Background(), &Token{
		AccessToken: "T", RefreshToken: "RT", ExpiresIn: 1800,
		AccessTokenIssued: now, RefreshTokenIssued: now,
	})
	c, err := NewClient(context.Background(), testAppKey32, testAppSecret16,
		WithCallbackURL(validCallback), WithTokenStore(store), WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()

	q, err := c.GetQuote(context.Background(), "BRK/B")
	if err != nil {
		t.Fatalf("GetQuote: %v", err)
	}
	// Path must have / escaped
	if !strings.Contains(gotPath, "BRK%2FB") {
		t.Errorf("path %q: expected BRK%%2FB", gotPath)
	}
	if q.Symbol != "BRK/B" {
		t.Errorf("symbol: %q", q.Symbol)
	}
}

func TestGetQuote_ZeroParamsOmitted(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "T", "expires_in": 1800})
			return
		}
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"MSFT":{"symbol":"MSFT"}}`))
	}))
	defer srv.Close()

	store := NewMemoryTokenStore()
	now := time.Now().UTC()
	_ = store.Save(context.Background(), &Token{
		AccessToken: "T", RefreshToken: "RT", ExpiresIn: 1800,
		AccessTokenIssued: now, RefreshTokenIssued: now,
	})
	c, err := NewClient(context.Background(), testAppKey32, testAppSecret16,
		WithCallbackURL(validCallback), WithTokenStore(store), WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()

	_, err = c.GetQuote(context.Background(), "MSFT")
	if err != nil {
		t.Fatalf("GetQuote: %v", err)
	}
	// No optional params should appear
	if strings.Contains(gotQuery, "fields=") {
		t.Errorf("unexpected fields in query: %q", gotQuery)
	}
	if strings.Contains(gotQuery, "indicative=") {
		t.Errorf("unexpected indicative in query: %q", gotQuery)
	}
}
