package schwab

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGetPriceHistory_Basic(t *testing.T) {
	var gotPath, gotQuery, gotAuth string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "TEST-TOKEN", "expires_in": 1800})
	})
	mux.HandleFunc("/marketdata/v1/pricehistory", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"symbol":"AAPL","empty":false,"candles":[{"open":150.0,"high":155.0,"low":149.0,"close":153.0,"volume":1000000,"datetime":1700000000000}]}`))
	})

	c := newTestClientWithMux(t, mux)

	period := 5
	ph, err := c.GetPriceHistory(context.Background(), PriceHistoryRequest{
		Symbol:     "AAPL",
		PeriodType: "month",
		Period:     &period,
	})
	if err != nil {
		t.Fatalf("GetPriceHistory: %v", err)
	}

	if gotPath != "/marketdata/v1/pricehistory" {
		t.Errorf("path: %q", gotPath)
	}
	if !strings.Contains(gotQuery, "symbol=AAPL") {
		t.Errorf("query missing symbol: %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "periodType=month") {
		t.Errorf("query missing periodType: %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "period=5") {
		t.Errorf("query missing period: %q", gotQuery)
	}
	if gotAuth != "Bearer TEST-TOKEN" {
		t.Errorf("auth: %q", gotAuth)
	}
	if ph.Symbol != "AAPL" || len(ph.Candles) != 1 {
		t.Errorf("response: %+v", ph)
	}
}

func TestGetPriceHistory_EpochMSDates(t *testing.T) {
	var gotQuery string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "T", "expires_in": 1800})
	})
	mux.HandleFunc("/marketdata/v1/pricehistory", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"symbol":"MSFT","empty":true,"candles":[]}`))
	})

	c := newTestClientWithMux(t, mux)

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	_, err := c.GetPriceHistory(context.Background(), PriceHistoryRequest{
		Symbol:    "MSFT",
		StartDate: start,
		EndDate:   end,
	})
	if err != nil {
		t.Fatalf("GetPriceHistory: %v", err)
	}

	// Dates must be epoch-ms integers
	if !strings.Contains(gotQuery, "startDate=") {
		t.Errorf("query missing startDate: %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "endDate=") {
		t.Errorf("query missing endDate: %q", gotQuery)
	}
	// Must not be ISO date format
	if strings.Contains(gotQuery, "2024-01-01") {
		t.Errorf("startDate must be epoch-ms, not ISO: %q", gotQuery)
	}
}

func TestGetPriceHistory_ZeroParamsOmitted(t *testing.T) {
	var gotQuery string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "T", "expires_in": 1800})
	})
	mux.HandleFunc("/marketdata/v1/pricehistory", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"symbol":"GOOG","empty":true,"candles":[]}`))
	})

	c := newTestClientWithMux(t, mux)

	_, err := c.GetPriceHistory(context.Background(), PriceHistoryRequest{Symbol: "GOOG"})
	if err != nil {
		t.Fatalf("GetPriceHistory: %v", err)
	}

	for _, key := range []string{"startDate=", "endDate=", "period=", "frequency=", "needExtendedHoursData=", "needPreviousClose="} {
		if strings.Contains(gotQuery, key) {
			t.Errorf("unexpected param %q in query: %q", key, gotQuery)
		}
	}
}
