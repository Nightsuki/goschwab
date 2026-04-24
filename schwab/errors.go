package schwab

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Sentinel errors. Use errors.Is(err, schwab.ErrXxx) to match against them.
var (
	// ErrNoToken indicates no token has been persisted. Caller must perform
	// the OAuth authorization flow.
	ErrNoToken = errors.New("schwab: no token persisted; must authorize first")

	// ErrRefreshTokenExpired indicates the refresh token itself has expired
	// (7-day TTL). A full reauthorization is required.
	ErrRefreshTokenExpired = errors.New("schwab: refresh token expired; full reauth required")

	// ErrStreamClosed indicates the streamer has been permanently closed.
	ErrStreamClosed = errors.New("schwab: streamer is closed")

	// ErrStreamInactive indicates the streamer is not currently connected;
	// requests are queued and replayed on the next successful Start.
	ErrStreamInactive = errors.New("schwab: streamer is inactive; request queued")

	// ErrRateLimited is a class-match sentinel for HTTP 429 responses.
	// Use with errors.Is to detect rate-limit errors regardless of type.
	ErrRateLimited = errors.New("schwab: rate limited")

	// ErrBadRequest is a class-match sentinel for HTTP 400 responses.
	ErrBadRequest = errors.New("schwab: bad request")

	// ErrForbidden is a class-match sentinel for HTTP 403 responses.
	ErrForbidden = errors.New("schwab: forbidden")

	// ErrNotFound is a class-match sentinel for HTTP 404 responses.
	ErrNotFound = errors.New("schwab: not found")

	// ErrServer is a class-match sentinel for HTTP 5xx responses.
	ErrServer = errors.New("schwab: server error")
)

// APIError is a structured error returned for non-2xx HTTP responses.
type APIError struct {
	// StatusCode is the HTTP status code.
	StatusCode int
	// Code is the server-supplied error code if present.
	Code string
	// Message is a human-readable error message extracted from the body.
	Message string
	// Raw is the full response body for diagnostics.
	Raw json.RawMessage
	// RequestID is the value of the schwab-client-correl-id response header.
	RequestID string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Code != "" {
		return fmt.Sprintf("schwab: http %d: %s (%s)", e.StatusCode, e.Message, e.Code)
	}
	if e.Message != "" {
		return fmt.Sprintf("schwab: http %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("schwab: http %d", e.StatusCode)
}

// Is reports whether target matches this APIError. Callers may compare
// against sentinel errors such as ErrBadRequest/ErrForbidden/ErrNotFound/
// ErrServer/ErrRateLimited for status-class matching.
func (e *APIError) Is(target error) bool {
	if e == nil {
		return false
	}
	switch target {
	case ErrBadRequest:
		return e.StatusCode == 400
	case ErrForbidden:
		return e.StatusCode == 403
	case ErrNotFound:
		return e.StatusCode == 404
	case ErrRateLimited:
		return e.StatusCode == 429
	case ErrServer:
		return e.StatusCode >= 500 && e.StatusCode <= 599
	}
	return false
}

// AuthError is returned for any failure in the OAuth2 flow.
type AuthError struct {
	// Stage is one of "authorize", "exchange", "refresh", "parse".
	Stage string
	// Err is the underlying cause.
	Err error
}

// Error implements the error interface.
func (e *AuthError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err != nil {
		return fmt.Sprintf("schwab: auth %s: %v", e.Stage, e.Err)
	}
	return fmt.Sprintf("schwab: auth %s", e.Stage)
}

// Unwrap returns the underlying cause for errors.Is / errors.As.
func (e *AuthError) Unwrap() error { return e.Err }

// RateLimitError wraps an APIError for HTTP 429 responses, carrying the
// parsed Retry-After duration when present.
type RateLimitError struct {
	APIError
	// RetryAfter is the duration to wait before retrying, parsed from the
	// Retry-After response header. Zero if not present.
	RetryAfter time.Duration
}

// Error implements the error interface.
func (e *RateLimitError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.RetryAfter > 0 {
		return fmt.Sprintf("schwab: http 429: rate limited, retry after %s", e.RetryAfter)
	}
	return "schwab: http 429: rate limited"
}

// Is reports whether target matches. Matches ErrRateLimited as well as the
// embedded APIError's sentinels.
func (e *RateLimitError) Is(target error) bool {
	if e == nil {
		return false
	}
	if target == ErrRateLimited {
		return true
	}
	return e.APIError.Is(target)
}

// Unwrap returns the embedded APIError so callers can errors.As into it.
func (e *RateLimitError) Unwrap() error { return &e.APIError }

// StreamError wraps errors from the streamer lifecycle.
type StreamError struct {
	// Op is the failed operation: "connect", "login", "read", "write", "close".
	Op string
	// Err is the underlying cause.
	Err error
}

// Error implements the error interface.
func (e *StreamError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err != nil {
		return fmt.Sprintf("schwab: stream %s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("schwab: stream %s", e.Op)
}

// Unwrap returns the underlying cause for errors.Is / errors.As.
func (e *StreamError) Unwrap() error { return e.Err }
