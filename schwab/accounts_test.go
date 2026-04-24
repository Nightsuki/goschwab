package schwab

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestListLinkedAccounts(t *testing.T) {
	want := []LinkedAccount{
		{AccountNumber: "12345678", HashValue: "HASH_ABC"},
	}
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "TEST-TOKEN", "expires_in": 1800})
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("method: got %s want GET", r.Method)
		}
		if r.URL.Path != "/trader/v1/accounts/accountNumbers" {
			t.Errorf("path: got %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer TEST-TOKEN" {
			t.Errorf("auth header: %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))
	got, err := c.ListLinkedAccounts(context.Background())
	if err != nil {
		t.Fatalf("ListLinkedAccounts: %v", err)
	}
	if len(got) != 1 || got[0].HashValue != "HASH_ABC" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetAllAccounts(t *testing.T) {
	rawAcct := json.RawMessage(`{"securitiesAccount":{"accountNumber":"12345678"}}`)
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "TEST-TOKEN", "expires_in": 1800})
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("method: got %s want GET", r.Method)
		}
		if r.URL.Path != "/trader/v1/accounts/" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		if q := r.URL.Query().Get("fields"); q != "positions" {
			t.Errorf("fields param: got %q want positions", q)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer TEST-TOKEN" {
			t.Errorf("auth header: %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]json.RawMessage{rawAcct})
	}))
	got, err := c.GetAllAccounts(context.Background(), "positions")
	if err != nil {
		t.Fatalf("GetAllAccounts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d accounts", len(got))
	}
	if got[0].Raw == nil {
		t.Fatal("Raw should be set")
	}
}

func TestGetAllAccounts_InvalidFields(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "TEST-TOKEN", "expires_in": 1800})
			return
		}
	}))
	_, err := c.GetAllAccounts(context.Background(), "invalid")
	if err == nil || !strings.Contains(err.Error(), "fields") {
		t.Fatalf("expected fields validation error, got %v", err)
	}
}

func TestGetAccount(t *testing.T) {
	rawAcct := json.RawMessage(`{"securitiesAccount":{"accountNumber":"99887766"}}`)
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "TEST-TOKEN", "expires_in": 1800})
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("method: got %s want GET", r.Method)
		}
		if r.URL.Path != "/trader/v1/accounts/HASH123" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		if q := r.URL.Query().Get("fields"); q != "" {
			t.Errorf("fields should be absent, got %q", q)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(rawAcct)
	}))
	got, err := c.GetAccount(context.Background(), "HASH123", "")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if got.Raw == nil {
		t.Fatal("Raw should be set")
	}
}
