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

func TestGetMovers_Basic(t *testing.T) {
	var gotPath, gotQuery, gotAuth string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "TEST-TOKEN", "expires_in": 1800})
	})
	mux.HandleFunc("/marketdata/v1/movers/", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"screeners":[{"symbol":"NVDA","totalVolume":5000000}]}`))
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

	movers, err := c.GetMovers(context.Background(), "$DJI",
		WithMoverSort(MoverSortVolume),
		WithMoverFrequency(5),
	)
	if err != nil {
		t.Fatalf("GetMovers: %v", err)
	}

	if gotPath != "/marketdata/v1/movers/%24DJI" && gotPath != "/marketdata/v1/movers/$DJI" {
		t.Errorf("path: %q", gotPath)
	}
	if !strings.Contains(gotQuery, "sort=VOLUME") {
		t.Errorf("query missing sort: %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "frequency=5") {
		t.Errorf("query missing frequency: %q", gotQuery)
	}
	if gotAuth != "Bearer TEST-TOKEN" {
		t.Errorf("auth: %q", gotAuth)
	}
	if movers.Screeners == nil {
		t.Errorf("screeners nil")
	}
}

func TestGetMovers_ZeroParamsOmitted(t *testing.T) {
	var gotQuery string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "T", "expires_in": 1800})
	})
	mux.HandleFunc("/marketdata/v1/movers/", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})

	c := newTestClientWithMux(t, mux)

	_, err := c.GetMovers(context.Background(), "$SPX")
	if err != nil {
		t.Fatalf("GetMovers: %v", err)
	}

	// No sort or frequency without options
	if strings.Contains(gotQuery, "sort=") {
		t.Errorf("unexpected sort in query: %q", gotQuery)
	}
	if strings.Contains(gotQuery, "frequency=") {
		t.Errorf("unexpected frequency in query: %q", gotQuery)
	}
}
