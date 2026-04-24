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

func TestGetInstruments_Basic(t *testing.T) {
	var gotPath, gotQuery, gotAuth string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "TEST-TOKEN", "expires_in": 1800})
	})
	mux.HandleFunc("/marketdata/v1/instruments", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"instruments":[{"cusip":"037833100","symbol":"AAPL","description":"Apple Inc","exchange":"NASDAQ","assetType":"EQUITY"}]}`))
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

	instruments, err := c.GetInstruments(context.Background(), []string{"AAPL"}, "symbol-search")
	if err != nil {
		t.Fatalf("GetInstruments: %v", err)
	}

	if gotPath != "/marketdata/v1/instruments" {
		t.Errorf("path: %q", gotPath)
	}
	if !strings.Contains(gotQuery, "symbol=AAPL") {
		t.Errorf("query missing symbol: %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "projection=symbol-search") {
		t.Errorf("query missing projection: %q", gotQuery)
	}
	if gotAuth != "Bearer TEST-TOKEN" {
		t.Errorf("auth: %q", gotAuth)
	}
	if len(instruments.Instruments) != 1 || instruments.Instruments[0].Symbol != "AAPL" {
		t.Errorf("instruments: %+v", instruments)
	}
}

func TestGetInstruments_MultiSymbolCSV(t *testing.T) {
	var gotQuery string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "T", "expires_in": 1800})
	})
	mux.HandleFunc("/marketdata/v1/instruments", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"instruments":[]}`))
	})

	c := newTestClientWithMux(t, mux)

	_, err := c.GetInstruments(context.Background(), []string{"AAPL", "MSFT", "GOOG"}, "symbol-search")
	if err != nil {
		t.Fatalf("GetInstruments: %v", err)
	}

	// symbol must be CSV (single key)
	if strings.Count(gotQuery, "symbol=") != 1 {
		t.Errorf("symbol appears multiple times: %q", gotQuery)
	}
	rawQ, _ := strings.CutPrefix(gotQuery, "symbol=")
	_ = rawQ
	// Check that all three appear
	for _, sym := range []string{"AAPL", "MSFT", "GOOG"} {
		if !strings.Contains(gotQuery, sym) {
			t.Errorf("query missing symbol %q: %q", sym, gotQuery)
		}
	}
}

func TestGetInstrumentByCUSIP(t *testing.T) {
	var gotPath, gotAuth string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "TEST-TOKEN", "expires_in": 1800})
	})
	mux.HandleFunc("/marketdata/v1/instruments/", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"cusip":"037833100","symbol":"AAPL","assetType":"EQUITY"}`))
	})

	c := newTestClientWithMux(t, mux)

	inst, err := c.GetInstrumentByCUSIP(context.Background(), "037833100")
	if err != nil {
		t.Fatalf("GetInstrumentByCUSIP: %v", err)
	}

	if gotPath != "/marketdata/v1/instruments/037833100" {
		t.Errorf("path: %q", gotPath)
	}
	if gotAuth != "Bearer TEST-TOKEN" {
		t.Errorf("auth: %q", gotAuth)
	}
	if inst.CUSIP != "037833100" || inst.Symbol != "AAPL" {
		t.Errorf("instrument: %+v", inst)
	}
}
