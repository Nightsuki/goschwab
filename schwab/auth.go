package schwab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Default Schwab OAuth endpoints.
const (
	defaultBaseURL      = "https://api.schwabapi.com"
	authorizeURLPath    = "/v1/oauth/authorize"
	tokenURLPath        = "/v1/oauth/token"
	defaultCallbackURL  = "https://127.0.0.1"
	accessTokenRefreshThreshold  = 61 * time.Second
	refreshTokenRefreshThreshold = 60*time.Minute + 30*time.Second
)

// buildAuthorizeURL returns the URL the user must open to authorize the app.
// Shape: https://api.schwabapi.com/v1/oauth/authorize?client_id={key}&redirect_uri={cb}
func buildAuthorizeURL(base, appKey, callback string) string {
	if base == "" {
		base = defaultBaseURL
	}
	v := url.Values{}
	v.Set("client_id", appKey)
	v.Set("redirect_uri", callback)
	return base + authorizeURLPath + "?" + v.Encode()
}

// extractCodeFromRedirect parses a Schwab redirect URL and returns the
// URL-decoded ?code= parameter. If raw is already a bare code (no "?code="),
// it is returned unchanged.
func extractCodeFromRedirect(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("schwab: empty redirect URL")
	}
	// If the caller pasted just the code, accept it verbatim.
	if !strings.Contains(raw, "?") && !strings.HasPrefix(raw, "http") {
		return raw, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("schwab: parse redirect URL: %w", err)
	}
	code := u.Query().Get("code")
	if code == "" {
		return "", errors.New("schwab: redirect URL missing ?code= parameter")
	}
	// url.Parse has already decoded the value once. Accept it as-is.
	return code, nil
}

// exchangeCode performs the authorization-code → token exchange.
func exchangeCode(ctx context.Context, hc *http.Client, base, appKey, appSecret, code, callback string) (*Token, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", callback)
	return postTokenRequest(ctx, hc, base, appKey, appSecret, form, "exchange")
}

// refreshToken performs the refresh-token → access-token exchange.
func refreshToken(ctx context.Context, hc *http.Client, base, appKey, appSecret, refresh string) (*Token, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refresh)
	return postTokenRequest(ctx, hc, base, appKey, appSecret, form, "refresh")
}

// postTokenRequest is the shared HTTP body for both token grant flows.
func postTokenRequest(ctx context.Context, hc *http.Client, base, appKey, appSecret string, form url.Values, stage string) (*Token, error) {
	if base == "" {
		base = defaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+tokenURLPath, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, &AuthError{Stage: stage, Err: err}
	}
	req.SetBasicAuth(appKey, appSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := hc.Do(req)
	if err != nil {
		return nil, &AuthError{Stage: stage, Err: err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &AuthError{Stage: stage, Err: fmt.Errorf("read body: %w", err)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &AuthError{
			Stage: stage,
			Err:   fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(body))),
		}
	}
	var tok Token
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, &AuthError{Stage: "parse", Err: fmt.Errorf("unmarshal token: %w", err)}
	}
	now := time.Now().UTC()
	tok.AccessTokenIssued = now
	// On refresh-token grant, Schwab re-issues a fresh refresh token, so we
	// reset its issued timestamp as well. On access-token-only responses the
	// refresh token field is unchanged and we preserve the original issuance.
	if form.Get("grant_type") == "authorization_code" || tok.RefreshToken != "" {
		tok.RefreshTokenIssued = now
	}
	return &tok, nil
}

// UpdateTokens refreshes tokens when they are within their refresh thresholds.
// Returns true when a refresh or re-auth was performed. Normally called
// automatically before every request; exposed for explicit pre-flight checks.
func (c *Client) UpdateTokens(ctx context.Context) (bool, error) {
	c.mu.Lock()
	tok := c.token
	c.mu.Unlock()

	if tok == nil {
		loaded, err := c.cfg.tokenStore.Load(ctx)
		if err != nil {
			return false, err
		}
		if loaded == nil {
			// No token — need full reauth.
			if err := c.doFullReauth(ctx); err != nil {
				return false, err
			}
			return true, nil
		}
		c.mu.Lock()
		c.token = loaded
		tok = loaded
		c.mu.Unlock()
	}

	// If the refresh token itself is about to expire, we must do full reauth.
	if tok.RefreshTokenRemaining() < refreshTokenRefreshThreshold {
		if err := c.doFullReauth(ctx); err != nil {
			return false, err
		}
		return true, nil
	}

	// If the access token is near expiry, refresh it.
	if tok.AccessTokenRemaining() < accessTokenRefreshThreshold {
		if err := c.doRefresh(ctx); err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}

// ForceRefresh triggers a full OAuth reauthorization immediately.
func (c *Client) ForceRefresh(ctx context.Context) error {
	return c.doFullReauth(ctx)
}

// doRefresh exchanges the current refresh token for a new access token.
func (c *Client) doRefresh(ctx context.Context) error {
	c.mu.Lock()
	tok := c.token
	c.mu.Unlock()
	if tok == nil || tok.RefreshToken == "" {
		return ErrNoToken
	}
	newTok, err := refreshToken(ctx, c.httpClient, c.cfg.baseURL, c.appKey, c.appSecret, tok.RefreshToken)
	if err != nil {
		return err
	}
	// Schwab's refresh response sometimes omits the refresh token itself;
	// preserve the previous one in that case along with its original issuance.
	if newTok.RefreshToken == "" {
		newTok.RefreshToken = tok.RefreshToken
		newTok.RefreshTokenIssued = tok.RefreshTokenIssued
	}
	c.mu.Lock()
	c.token = newTok
	c.mu.Unlock()
	return c.cfg.tokenStore.Save(ctx, newTok)
}

// doFullReauth runs the interactive authorize → code-exchange flow.
func (c *Client) doFullReauth(ctx context.Context) error {
	if c.cfg.authorizer == nil {
		return &AuthError{Stage: "authorize", Err: errors.New("no Authorizer configured")}
	}
	authURL := buildAuthorizeURL(c.cfg.baseURL, c.appKey, c.cfg.callbackURL)
	redirect, err := c.cfg.authorizer.Authorize(ctx, authURL)
	if err != nil {
		return &AuthError{Stage: "authorize", Err: err}
	}
	code, err := extractCodeFromRedirect(redirect)
	if err != nil {
		return &AuthError{Stage: "parse", Err: err}
	}
	tok, err := exchangeCode(ctx, c.httpClient, c.cfg.baseURL, c.appKey, c.appSecret, code, c.cfg.callbackURL)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.token = tok
	c.mu.Unlock()
	return c.cfg.tokenStore.Save(ctx, tok)
}
