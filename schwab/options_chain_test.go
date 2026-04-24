package schwab

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGetOptionChain_Basic(t *testing.T) {
	var gotPath, gotQuery, gotAuth string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "TEST-TOKEN", "expires_in": 1800})
	})
	mux.HandleFunc("/marketdata/v1/chains", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"symbol":"AAPL","status":"SUCCESS","strategy":"SINGLE"}`))
	})

	c := newTestClientWithMux(t, mux)

	chain, err := c.GetOptionChain(context.Background(), OptionChainRequest{
		Symbol:       "AAPL",
		ContractType: ContractTypeCall,
	})
	if err != nil {
		t.Fatalf("GetOptionChain: %v", err)
	}

	if gotPath != "/marketdata/v1/chains" {
		t.Errorf("path: %q", gotPath)
	}
	if !strings.Contains(gotQuery, "symbol=AAPL") {
		t.Errorf("query missing symbol: %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "contractType=CALL") {
		t.Errorf("query missing contractType: %q", gotQuery)
	}
	if gotAuth != "Bearer TEST-TOKEN" {
		t.Errorf("auth: %q", gotAuth)
	}
	if chain.Symbol != "AAPL" || chain.Status != "SUCCESS" {
		t.Errorf("chain: %+v", chain)
	}
}

func TestGetOptionChain_DateFormat(t *testing.T) {
	var gotQuery string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "T", "expires_in": 1800})
	})
	mux.HandleFunc("/marketdata/v1/chains", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"symbol":"SPY","status":"SUCCESS","strategy":"SINGLE"}`))
	})

	c := newTestClientWithMux(t, mux)

	from := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 3, 21, 0, 0, 0, 0, time.UTC)

	_, err := c.GetOptionChain(context.Background(), OptionChainRequest{
		Symbol:   "SPY",
		FromDate: from,
		ToDate:   to,
	})
	if err != nil {
		t.Fatalf("GetOptionChain: %v", err)
	}

	if !strings.Contains(gotQuery, "fromDate=2025-01-15") {
		t.Errorf("query missing fromDate: %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "toDate=2025-03-21") {
		t.Errorf("query missing toDate: %q", gotQuery)
	}
}

func TestGetOptionChain_ZeroDatesOmitted(t *testing.T) {
	var gotQuery string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "T", "expires_in": 1800})
	})
	mux.HandleFunc("/marketdata/v1/chains", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"symbol":"QQQ","status":"SUCCESS","strategy":"SINGLE"}`))
	})

	c := newTestClientWithMux(t, mux)

	_, err := c.GetOptionChain(context.Background(), OptionChainRequest{Symbol: "QQQ"})
	if err != nil {
		t.Fatalf("GetOptionChain: %v", err)
	}

	if strings.Contains(gotQuery, "fromDate=") {
		t.Errorf("unexpected fromDate in query: %q", gotQuery)
	}
	if strings.Contains(gotQuery, "toDate=") {
		t.Errorf("unexpected toDate in query: %q", gotQuery)
	}
}

func TestGetOptionExpirationChain(t *testing.T) {
	var gotPath, gotQuery string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "T", "expires_in": 1800})
	})
	mux.HandleFunc("/marketdata/v1/expirationchain", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"SUCCESS"}`))
	})

	c := newTestClientWithMux(t, mux)

	chain, err := c.GetOptionExpirationChain(context.Background(), "TSLA")
	if err != nil {
		t.Fatalf("GetOptionExpirationChain: %v", err)
	}
	if gotPath != "/marketdata/v1/expirationchain" {
		t.Errorf("path: %q", gotPath)
	}
	if !strings.Contains(gotQuery, "symbol=TSLA") {
		t.Errorf("query missing symbol: %q", gotQuery)
	}
	if chain.Status != "SUCCESS" {
		t.Errorf("status: %q", chain.Status)
	}
}
