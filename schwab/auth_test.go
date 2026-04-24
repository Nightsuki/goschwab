package schwab

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractCodeFromRedirect(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"full url", "https://127.0.0.1/?code=abc123", "abc123", false},
		{"url with extras", "https://127.0.0.1/path?code=abc123&state=xyz", "abc123", false},
		{"url-encoded", "https://127.0.0.1/?code=ab%2Fcd", "ab/cd", false},
		{"bare code", "rawcode", "rawcode", false},
		{"missing code", "https://127.0.0.1/?state=x", "", true},
		{"empty", "", "", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractCodeFromRedirect(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestBuildAuthorizeURL(t *testing.T) {
	u := buildAuthorizeURL("https://api.example.com", "KEY", "https://127.0.0.1")
	if !strings.HasPrefix(u, "https://api.example.com/v1/oauth/authorize?") {
		t.Fatalf("prefix mismatch: %s", u)
	}
	if !strings.Contains(u, "client_id=KEY") {
		t.Fatalf("missing client_id: %s", u)
	}
	if !strings.Contains(u, "redirect_uri=https%3A%2F%2F127.0.0.1") {
		t.Fatalf("missing encoded redirect_uri: %s", u)
	}
}

func TestExchangeCode_Success(t *testing.T) {
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/oauth/token" {
			t.Errorf("path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method: %s", r.Method)
		}
		if got, want := r.Header.Get("Authorization"), "Basic "; !strings.HasPrefix(got, want) {
			t.Errorf("basic auth missing: %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Errorf("content-type: %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		form := string(body)
		if !strings.Contains(form, "grant_type=authorization_code") {
			t.Errorf("grant_type missing: %s", form)
		}
		if !strings.Contains(form, "code=mycode") {
			t.Errorf("code missing: %s", form)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "AT",
			"refresh_token": "RT",
			"token_type":    "Bearer",
			"scope":         "api",
			"expires_in":    1800,
		})
	}))
	defer srv.Close()

	tok, err := exchangeCode(ctx, http.DefaultClient, srv.URL, "AK", "AS", "mycode", "https://127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "AT" || tok.RefreshToken != "RT" || tok.ExpiresIn != 1800 {
		t.Fatalf("token mismatch: %+v", tok)
	}
	if tok.AccessTokenIssued.IsZero() || tok.RefreshTokenIssued.IsZero() {
		t.Fatal("issuance timestamps should be set")
	}
}

func TestRefreshToken_Success(t *testing.T) {
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "grant_type=refresh_token") {
			t.Errorf("grant_type missing: %s", body)
		}
		if !strings.Contains(string(body), "refresh_token=OLD") {
			t.Errorf("refresh_token missing: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "NEW",
			"token_type":   "Bearer",
			"expires_in":   1800,
		})
	}))
	defer srv.Close()

	tok, err := refreshToken(ctx, http.DefaultClient, srv.URL, "AK", "AS", "OLD")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "NEW" {
		t.Fatalf("access token mismatch: %q", tok.AccessToken)
	}
}

func TestExchangeCode_ServerError(t *testing.T) {
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
	}))
	defer srv.Close()
	_, err := exchangeCode(ctx, http.DefaultClient, srv.URL, "AK", "AS", "bad", "https://127.0.0.1")
	if err == nil {
		t.Fatal("expected error")
	}
	var ae *AuthError
	if !errorsAs(err, &ae) {
		t.Fatalf("expected *AuthError, got %T", err)
	}
	if ae.Stage != "exchange" {
		t.Fatalf("stage: %q", ae.Stage)
	}
}

// errorsAs is a local thin wrapper around errors.As to avoid importing
// "errors" twice in the test file.
func errorsAs(err error, target any) bool {
	return errorsAsImpl(err, target)
}
