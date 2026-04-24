package schwab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestClient creates a Client backed by an httptest.Server running handler.
// The handler must respond to /v1/oauth/token for token refresh calls.
func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	store := NewMemoryTokenStore()
	now := time.Now().UTC()
	if err := store.Save(context.Background(), &Token{
		AccessToken:        "TEST-TOKEN",
		RefreshToken:       "RT",
		ExpiresIn:          1800,
		AccessTokenIssued:  now,
		RefreshTokenIssued: now,
	}); err != nil {
		t.Fatal(err)
	}

	c, err := NewClient(context.Background(), testAppKey32, testAppSecret16,
		WithCallbackURL(validCallback),
		WithTokenStore(store),
		WithBaseURL(srv.URL),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// newTestClientWithMux creates a Client backed by an httptest.Server using a mux.
func newTestClientWithMux(t *testing.T, mux *http.ServeMux) *Client {
	t.Helper()
	return newTestClient(t, mux)
}
