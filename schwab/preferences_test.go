package schwab

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetUserPreferences(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			stubToken(w)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("method: got %s want GET", r.Method)
		}
		if r.URL.Path != "/trader/v1/userPreference" {
			t.Errorf("path: got %q", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer TEST-TOKEN" {
			t.Errorf("auth header: %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(UserPreferences{})
	}))
	got, err := c.GetUserPreferences(context.Background())
	if err != nil {
		t.Fatalf("GetUserPreferences: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil UserPreferences")
	}
}
