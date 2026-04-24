package schwab

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// errorsAsImpl wraps errors.As for the auth_test helper.
func errorsAsImpl(err error, target any) bool { return errors.As(err, target) }

const (
	testAppKey32    = "aaaaaaaabbbbbbbbccccccccdddddddd" // 32 chars
	testAppSecret16 = "ssssssssxxxxxxxx"                 // 16 chars
	validCallback   = "https://127.0.0.1"
)

func TestValidation_AppKey(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"empty", "", true},
		{"too short", "abc", true},
		{"32 ok", testAppKey32, false},
		{"48 ok", strings.Repeat("x", 48), false},
		{"33 bad", strings.Repeat("x", 33), true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateAppKey(tc.key)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestValidation_AppSecret(t *testing.T) {
	cases := []struct {
		name    string
		s       string
		wantErr bool
	}{
		{"empty", "", true},
		{"16 ok", testAppSecret16, false},
		{"64 ok", strings.Repeat("x", 64), false},
		{"15 bad", strings.Repeat("x", 15), true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateAppSecret(tc.s)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestValidation_CallbackURL(t *testing.T) {
	cases := []struct {
		name    string
		u       string
		wantErr bool
	}{
		{"https ok", "https://127.0.0.1", false},
		{"https port ok", "https://127.0.0.1:8443", false},
		{"empty", "", true},
		{"http", "http://127.0.0.1", true},
		{"trailing slash", "https://127.0.0.1/", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateCallbackURL(tc.u)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestNewClient_CallbackValidationTriggered(t *testing.T) {
	ctx := context.Background()
	_, err := NewClient(ctx, testAppKey32, testAppSecret16,
		WithCallbackURL("http://bad"),
		WithTokenStore(NewMemoryTokenStore()),
	)
	if err == nil || !strings.Contains(err.Error(), "callback URL") {
		t.Fatalf("expected callback validation error, got %v", err)
	}
}

func TestNewClient_AppKeyValidationTriggered(t *testing.T) {
	_, err := NewClient(context.Background(), "short", testAppSecret16,
		WithTokenStore(NewMemoryTokenStore()),
		WithCallbackURL(validCallback),
	)
	if err == nil || !strings.Contains(err.Error(), "app key") {
		t.Fatalf("expected app key validation error, got %v", err)
	}
}

// TestDo_401RefreshAndRetry verifies that a 401 response triggers exactly one
// refresh + retry, and that the second attempt uses the new access token.
func TestDo_401RefreshAndRetry(t *testing.T) {
	ctx := context.Background()

	var apiCalls int32
	var tokenCalls int32

	// Mux: /v1/oauth/token refreshes, everything else is an "API endpoint".
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&tokenCalls, 1)
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "grant_type=refresh_token") {
			t.Errorf("expected refresh grant, got %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "FRESH",
			"token_type":   "Bearer",
			"expires_in":   1800,
		})
	})
	mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&apiCalls, 1)
		auth := r.Header.Get("Authorization")
		if n == 1 {
			if auth != "Bearer STALE" {
				t.Errorf("first call auth: %q", auth)
			}
			http.Error(w, `{"message":"expired"}`, http.StatusUnauthorized)
			return
		}
		if auth != "Bearer FRESH" {
			t.Errorf("second call auth: %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	store := NewMemoryTokenStore()
	now := time.Now().UTC()
	if err := store.Save(ctx, &Token{
		AccessToken:        "STALE",
		RefreshToken:       "RT",
		ExpiresIn:          1800,
		AccessTokenIssued:  now,
		RefreshTokenIssued: now,
	}); err != nil {
		t.Fatal(err)
	}

	c, err := NewClient(ctx, testAppKey32, testAppSecret16,
		WithCallbackURL(validCallback),
		WithTokenStore(store),
		WithBaseURL(srv.URL),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })

	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "/api/test", url.Values{}, nil, &out); err != nil {
		t.Fatalf("do: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("body: %+v", out)
	}
	if n := atomic.LoadInt32(&apiCalls); n != 2 {
		t.Fatalf("api calls: got %d want 2", n)
	}
	if n := atomic.LoadInt32(&tokenCalls); n != 1 {
		t.Fatalf("token calls: got %d want 1", n)
	}
}

func TestDo_Second401ReturnsAuthError(t *testing.T) {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "FRESH",
			"token_type":   "Bearer",
			"expires_in":   1800,
		})
	})
	mux.HandleFunc("/api/test", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"still expired"}`, http.StatusUnauthorized)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	store := NewMemoryTokenStore()
	now := time.Now().UTC()
	_ = store.Save(ctx, &Token{AccessToken: "STALE", RefreshToken: "RT", ExpiresIn: 1800, AccessTokenIssued: now, RefreshTokenIssued: now})

	c, err := NewClient(ctx, testAppKey32, testAppSecret16,
		WithCallbackURL(validCallback),
		WithTokenStore(store),
		WithBaseURL(srv.URL),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })

	err = c.do(ctx, http.MethodGet, "/api/test", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var ae *AuthError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AuthError, got %T: %v", err, err)
	}
	if ae.Stage != "refresh" {
		t.Fatalf("stage: %q", ae.Stage)
	}
}

func TestDo_RateLimitError(t *testing.T) {
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "X", "expires_in": 1800})
			return
		}
		w.Header().Set("Retry-After", "7")
		http.Error(w, `{"message":"slow"}`, http.StatusTooManyRequests)
	}))
	defer srv.Close()

	store := NewMemoryTokenStore()
	now := time.Now().UTC()
	_ = store.Save(ctx, &Token{AccessToken: "A", RefreshToken: "R", ExpiresIn: 1800, AccessTokenIssued: now, RefreshTokenIssued: now})

	c, err := NewClient(ctx, testAppKey32, testAppSecret16,
		WithCallbackURL(validCallback),
		WithTokenStore(store),
		WithBaseURL(srv.URL),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })

	err = c.do(ctx, http.MethodGet, "/api/test", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("expected *RateLimitError, got %T: %v", err, err)
	}
	if rle.RetryAfter != 7*time.Second {
		t.Fatalf("retry-after: got %s want 7s", rle.RetryAfter)
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Fatal("errors.Is(ErrRateLimited) should match")
	}
}

func TestDo_GenericAPIError(t *testing.T) {
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/oauth/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "X", "expires_in": 1800})
			return
		}
		http.Error(w, `{"errors":[{"message":"nope","code":"E42"}]}`, http.StatusNotFound)
	}))
	defer srv.Close()

	store := NewMemoryTokenStore()
	now := time.Now().UTC()
	_ = store.Save(ctx, &Token{AccessToken: "A", RefreshToken: "R", ExpiresIn: 1800, AccessTokenIssued: now, RefreshTokenIssued: now})

	c, err := NewClient(ctx, testAppKey32, testAppSecret16,
		WithCallbackURL(validCallback),
		WithTokenStore(store),
		WithBaseURL(srv.URL),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })

	err = c.do(ctx, http.MethodGet, "/api/missing", nil, nil, nil)
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if ae.StatusCode != 404 {
		t.Fatalf("status: %d", ae.StatusCode)
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatal("errors.Is(ErrNotFound) should match")
	}
}
