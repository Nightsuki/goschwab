package schwab

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestListTransactions(t *testing.T) {
	start := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 3, 31, 23, 59, 59, 0, time.UTC)

	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			stubToken(w)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("method: got %s want GET", r.Method)
		}
		if r.URL.Path != "/trader/v1/accounts/HASH5/transactions" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer TEST-TOKEN" {
			t.Errorf("auth header: %q", auth)
		}
		q := r.URL.Query()
		if got := q.Get("startDate"); got != "2024-03-01T00:00:00.000Z" {
			t.Errorf("startDate: got %q", got)
		}
		if got := q.Get("endDate"); got != "2024-03-31T23:59:59.000Z" {
			t.Errorf("endDate: got %q", got)
		}
		if got := q.Get("types"); got != "TRADE" {
			t.Errorf("types: got %q want TRADE", got)
		}
		if got := q.Get("symbol"); got != "AAPL" {
			t.Errorf("symbol: got %q want AAPL", got)
		}
		w.Header().Set("Content-Type", "application/json")
		actID := int64(100)
		_ = json.NewEncoder(w).Encode([]Transaction{{ActivityID: &actID, Type: "TRADE"}})
	}))
	got, err := c.ListTransactions(context.Background(), "HASH5", TransactionsRequest{
		Start:  start,
		End:    end,
		Types:  "TRADE",
		Symbol: "AAPL",
	})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if len(got) != 1 || got[0].Type != "TRADE" {
		t.Fatalf("got %+v", got)
	}
}

func TestListTransactions_RequiresTimes(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			stubToken(w)
		}
	}))
	_, err := c.ListTransactions(context.Background(), "HASH5", TransactionsRequest{})
	if err == nil || !strings.Contains(err.Error(), "Start and End") {
		t.Fatalf("expected time validation error, got %v", err)
	}
}

func TestGetTransaction(t *testing.T) {
	actID := int64(555)
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			stubToken(w)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("method: got %s want GET", r.Method)
		}
		if r.URL.Path != "/trader/v1/accounts/HASH6/transactions/555" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer TEST-TOKEN" {
			t.Errorf("auth header: %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Transaction{ActivityID: &actID, Type: "DIVIDEND_OR_INTEREST"})
	}))
	got, err := c.GetTransaction(context.Background(), "HASH6", "555")
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if got.Type != "DIVIDEND_OR_INTEREST" {
		t.Fatalf("Type: got %q", got.Type)
	}
	if got.Raw == nil {
		t.Fatal("Raw should be set")
	}
}
