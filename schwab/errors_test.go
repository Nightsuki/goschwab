package schwab

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestAPIError_IsSentinels(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   error
	}{
		{"bad request", 400, ErrBadRequest},
		{"forbidden", 403, ErrForbidden},
		{"not found", 404, ErrNotFound},
		{"rate limited", 429, ErrRateLimited},
		{"server error", 500, ErrServer},
		{"server error 599", 599, ErrServer},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			e := &APIError{StatusCode: tc.status, Raw: json.RawMessage(`{}`)}
			if !errors.Is(e, tc.want) {
				t.Fatalf("errors.Is returned false for status %d / %v", tc.status, tc.want)
			}
		})
	}
}

func TestAPIError_IsDoesNotMatchWrongSentinel(t *testing.T) {
	e := &APIError{StatusCode: 404}
	if errors.Is(e, ErrRateLimited) {
		t.Fatal("errors.Is matched wrong sentinel")
	}
}

func TestAPIError_ErrorMessages(t *testing.T) {
	e := &APIError{StatusCode: 500}
	if got := e.Error(); got == "" {
		t.Fatal("expected non-empty Error()")
	}
	e.Message = "oops"
	if got := e.Error(); got == "" {
		t.Fatal("expected non-empty Error()")
	}
	e.Code = "X-5"
	if got := e.Error(); got == "" {
		t.Fatal("expected non-empty Error()")
	}
}

func TestAuthError_UnwrapAs(t *testing.T) {
	cause := errors.New("root cause")
	wrapped := &AuthError{Stage: "refresh", Err: cause}
	if !errors.Is(wrapped, cause) {
		t.Fatal("errors.Is should find wrapped cause")
	}
	var target *AuthError
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As should find *AuthError")
	}
	if target.Stage != "refresh" {
		t.Fatalf("stage: got %q want %q", target.Stage, "refresh")
	}
}

func TestRateLimitError_IsAndUnwrap(t *testing.T) {
	e := &RateLimitError{
		APIError:   APIError{StatusCode: 429, Message: "slow down"},
		RetryAfter: 3 * time.Second,
	}
	if !errors.Is(e, ErrRateLimited) {
		t.Fatal("expected errors.Is(ErrRateLimited)")
	}
	var api *APIError
	if !errors.As(e, &api) {
		t.Fatal("errors.As into *APIError failed")
	}
	if api.StatusCode != 429 {
		t.Fatalf("wrong status code after As: %d", api.StatusCode)
	}
	if e.Error() == "" {
		t.Fatal("expected non-empty Error()")
	}
}

func TestStreamError_Unwrap(t *testing.T) {
	cause := errors.New("wire gone")
	e := &StreamError{Op: "read", Err: cause}
	if !errors.Is(e, cause) {
		t.Fatal("errors.Is should find wrapped cause")
	}
	var se *StreamError
	if !errors.As(e, &se) {
		t.Fatal("errors.As should find *StreamError")
	}
}

func TestNilMethodsAreSafe(t *testing.T) {
	var (
		a  *APIError
		r  *RateLimitError
		au *AuthError
		s  *StreamError
	)
	if a.Error() == "" {
		t.Fatal("nil APIError.Error should return a placeholder")
	}
	if r.Error() == "" {
		t.Fatal("nil RateLimitError.Error should return a placeholder")
	}
	if au.Error() == "" {
		t.Fatal("nil AuthError.Error should return a placeholder")
	}
	if s.Error() == "" {
		t.Fatal("nil StreamError.Error should return a placeholder")
	}
}
