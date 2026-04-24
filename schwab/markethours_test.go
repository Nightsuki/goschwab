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

func TestGetMarketHours_Basic(t *testing.T) {
	var gotPath, gotQuery, gotAuth string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "TEST-TOKEN", "expires_in": 1800})
	})
	mux.HandleFunc("/marketdata/v1/markets", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"equity":{"EQ":{"date":"2025-01-15","marketType":"EQUITY","isOpen":true}}}`))
	})

	store := NewMemoryTokenStore()
	now := time.Now().UTC()
	_ = store.Save(context.Background(), &Token{
		AccessToken:        "TEST-TOKEN",
		RefreshToken:       "RT",
		ExpiresIn:          1800,
		AccessTokenIssued:  now,
		RefreshTokenIssued: now,
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := NewClient(context.Background(), testAppKey32, testAppSecret16,
		WithCallbackURL(validCallback), WithTokenStore(store), WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()

	date := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	mh, err := c.GetMarketHours(context.Background(), []string{"equity", "option"}, date)
	if err != nil {
		t.Fatalf("GetMarketHours: %v", err)
	}

	if gotPath != "/marketdata/v1/markets" {
		t.Errorf("path: %q", gotPath)
	}
	// markets must be CSV
	if !strings.Contains(gotQuery, "markets=equity%2Coption") && !strings.Contains(gotQuery, "markets=equity,option") {
		t.Errorf("query missing CSV markets: %q", gotQuery)
	}
	if strings.Count(gotQuery, "markets=") != 1 {
		t.Errorf("markets appears multiple times: %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "date=2025-01-15") {
		t.Errorf("query missing date: %q", gotQuery)
	}
	if gotAuth != "Bearer TEST-TOKEN" {
		t.Errorf("auth: %q", gotAuth)
	}
	if len(mh.Raw) == 0 {
		t.Error("Raw should be populated")
	}
}

func TestGetMarketHour_Single(t *testing.T) {
	var gotPath, gotQuery string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "T", "expires_in": 1800})
	})
	mux.HandleFunc("/marketdata/v1/markets/equity", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"equity":{"EQ":{"isOpen":true}}}`))
	})

	c := newTestClientWithMux(t, mux)

	date := time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC)
	_, err := c.GetMarketHour(context.Background(), "equity", date)
	if err != nil {
		t.Fatalf("GetMarketHour: %v", err)
	}

	if gotPath != "/marketdata/v1/markets/equity" {
		t.Errorf("path: %q", gotPath)
	}
	if !strings.Contains(gotQuery, "date=2025-06-10") {
		t.Errorf("query missing date: %q", gotQuery)
	}
}

func TestGetMarketHours_ZeroDateOmitted(t *testing.T) {
	var gotQuery string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "T", "expires_in": 1800})
	})
	mux.HandleFunc("/marketdata/v1/markets", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})

	c := newTestClientWithMux(t, mux)

	_, err := c.GetMarketHours(context.Background(), []string{"bond"}, time.Time{})
	if err != nil {
		t.Fatalf("GetMarketHours: %v", err)
	}

	if strings.Contains(gotQuery, "date=") {
		t.Errorf("unexpected date in query: %q", gotQuery)
	}
}
