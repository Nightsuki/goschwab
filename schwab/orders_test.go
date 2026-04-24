package schwab

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func stubToken(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "TEST-TOKEN", "expires_in": 1800})
}

func TestListAccountOrders(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			stubToken(w)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("method: got %s want GET", r.Method)
		}
		if r.URL.Path != "/trader/v1/accounts/HASH1/orders" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer TEST-TOKEN" {
			t.Errorf("auth header: %q", auth)
		}
		q := r.URL.Query()
		if got := q.Get("fromEnteredTime"); got != "2024-01-01T00:00:00.000Z" {
			t.Errorf("fromEnteredTime: got %q", got)
		}
		if got := q.Get("toEnteredTime"); got != "2024-01-31T23:59:59.000Z" {
			t.Errorf("toEnteredTime: got %q", got)
		}
		if got := q.Get("maxResults"); got != "50" {
			t.Errorf("maxResults: got %q want 50", got)
		}
		if got := q.Get("status"); got != "FILLED" {
			t.Errorf("status: got %q want FILLED", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]Order{{Session: "NORMAL"}})
	}))
	got, err := c.ListAccountOrders(context.Background(), "HASH1", OrderListRequest{
		From:       from,
		To:         to,
		MaxResults: 50,
		Status:     "FILLED",
	})
	if err != nil {
		t.Fatalf("ListAccountOrders: %v", err)
	}
	if len(got) != 1 || got[0].Session != "NORMAL" {
		t.Fatalf("got %+v", got)
	}
}

func TestListAccountOrders_DefaultMaxResults(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			stubToken(w)
			return
		}
		if got := r.URL.Query().Get("maxResults"); got != "3000" {
			t.Errorf("maxResults default: got %q want 3000", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]Order{})
	}))
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	_, err := c.ListAccountOrders(context.Background(), "HASH1", OrderListRequest{From: from, To: to})
	if err != nil {
		t.Fatalf("ListAccountOrders: %v", err)
	}
}

func TestListOrders(t *testing.T) {
	from := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)

	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			stubToken(w)
			return
		}
		if r.URL.Path != "/trader/v1/orders" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer TEST-TOKEN" {
			t.Errorf("auth header: %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]Order{{OrderType: "LIMIT"}})
	}))
	got, err := c.ListOrders(context.Background(), OrderListRequest{From: from, To: to})
	if err != nil {
		t.Fatalf("ListOrders: %v", err)
	}
	if len(got) != 1 || got[0].OrderType != "LIMIT" {
		t.Fatalf("got %+v", got)
	}
}

func TestPlaceOrder_LocationHeader(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			stubToken(w)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s want POST", r.Method)
		}
		if r.URL.Path != "/trader/v1/accounts/ABC/orders" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer TEST-TOKEN" {
			t.Errorf("auth header: %q", auth)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("content-type: %q", ct)
		}
		w.Header().Set("Location", "https://api.schwabapi.com/trader/v1/accounts/ABC/orders/999")
		w.WriteHeader(http.StatusCreated)
	}))
	orderID, err := c.PlaceOrder(context.Background(), "ABC", &Order{OrderType: "MARKET"})
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if orderID != "999" {
		t.Fatalf("orderID: got %q want 999", orderID)
	}
}

func TestPlaceOrder_NoLocationHeader(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			stubToken(w)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	orderID, err := c.PlaceOrder(context.Background(), "ABC", &Order{OrderType: "MARKET"})
	if err != nil {
		t.Fatalf("PlaceOrder no-location: %v", err)
	}
	if orderID != "" {
		t.Fatalf("expected empty orderID, got %q", orderID)
	}
}

func TestReplaceOrder_LocationHeader(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			stubToken(w)
			return
		}
		if r.Method != http.MethodPut {
			t.Errorf("method: got %s want PUT", r.Method)
		}
		if r.URL.Path != "/trader/v1/accounts/ABC/orders/111" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		w.Header().Set("Location", "https://api.schwabapi.com/trader/v1/accounts/ABC/orders/222")
		w.WriteHeader(http.StatusCreated)
	}))
	newID, err := c.ReplaceOrder(context.Background(), "ABC", "111", &Order{OrderType: "LIMIT"})
	if err != nil {
		t.Fatalf("ReplaceOrder: %v", err)
	}
	if newID != "222" {
		t.Fatalf("newOrderID: got %q want 222", newID)
	}
}

func TestPreviewOrder(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			stubToken(w)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s want POST", r.Method)
		}
		if r.URL.Path != "/trader/v1/accounts/HASH2/previewOrder" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(OrderPreview{})
	}))
	got, err := c.PreviewOrder(context.Background(), "HASH2", &Order{OrderType: "LIMIT"})
	if err != nil {
		t.Fatalf("PreviewOrder: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil preview")
	}
}

func TestGetOrder(t *testing.T) {
	q := int64(42)
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			stubToken(w)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("method: got %s want GET", r.Method)
		}
		if r.URL.Path != "/trader/v1/accounts/HASH3/orders/42" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Order{OrderID: &q, OrderType: "STOP"})
	}))
	got, err := c.GetOrder(context.Background(), "HASH3", "42")
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if got.OrderType != "STOP" {
		t.Fatalf("OrderType: got %q want STOP", got.OrderType)
	}
}

func TestCancelOrder(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			stubToken(w)
			return
		}
		if r.Method != http.MethodDelete {
			t.Errorf("method: got %s want DELETE", r.Method)
		}
		if r.URL.Path != "/trader/v1/accounts/HASH4/orders/77" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer TEST-TOKEN" {
			t.Errorf("auth header: %q", auth)
		}
		w.WriteHeader(http.StatusOK)
	}))
	if err := c.CancelOrder(context.Background(), "HASH4", "77"); err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
}

func TestListOrders_RequiresTimes(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			stubToken(w)
		}
	}))
	_, err := c.ListOrders(context.Background(), OrderListRequest{})
	if err == nil || !strings.Contains(err.Error(), "From and To") {
		t.Fatalf("expected time validation error, got %v", err)
	}
}
