package schwab

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/hkdf"
)

// Token is a single OAuth2 credential set returned by the Schwab token
// endpoint and persisted by a TokenStore.
type Token struct {
	// AccessToken is the short-lived bearer token (~30 min TTL).
	AccessToken string `json:"access_token"`
	// RefreshToken is the long-lived token used to mint new access tokens (~7-day TTL).
	RefreshToken string `json:"refresh_token"`
	// IDToken is the OpenID-Connect ID token when returned.
	IDToken string `json:"id_token,omitempty"`
	// TokenType is typically "Bearer".
	TokenType string `json:"token_type,omitempty"`
	// Scope is the space-separated list of scopes.
	Scope string `json:"scope,omitempty"`
	// AccessTokenIssued records when the access token was minted.
	AccessTokenIssued time.Time `json:"access_token_issued"`
	// RefreshTokenIssued records when the refresh token was minted.
	RefreshTokenIssued time.Time `json:"refresh_token_issued"`
	// ExpiresIn is the access-token TTL in seconds as returned by the server.
	ExpiresIn int `json:"expires_in"`
}

// Default TTLs per Schwab docs (used when the server response lacks them).
const (
	defaultAccessTokenTTL  = 30 * time.Minute
	defaultRefreshTokenTTL = 7 * 24 * time.Hour
)

// AccessTokenExpiresAt returns the access-token expiry wall-clock time.
// Falls back to issued + 30m if ExpiresIn is zero.
func (t *Token) AccessTokenExpiresAt() time.Time {
	if t == nil || t.AccessTokenIssued.IsZero() {
		return time.Time{}
	}
	ttl := time.Duration(t.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = defaultAccessTokenTTL
	}
	return t.AccessTokenIssued.Add(ttl)
}

// RefreshTokenExpiresAt returns the refresh-token expiry wall-clock time.
func (t *Token) RefreshTokenExpiresAt() time.Time {
	if t == nil || t.RefreshTokenIssued.IsZero() {
		return time.Time{}
	}
	return t.RefreshTokenIssued.Add(defaultRefreshTokenTTL)
}

// AccessTokenRemaining returns the remaining time until the access token expires.
func (t *Token) AccessTokenRemaining() time.Duration {
	exp := t.AccessTokenExpiresAt()
	if exp.IsZero() {
		return 0
	}
	return time.Until(exp)
}

// RefreshTokenRemaining returns the remaining time until the refresh token expires.
func (t *Token) RefreshTokenRemaining() time.Duration {
	exp := t.RefreshTokenExpiresAt()
	if exp.IsZero() {
		return 0
	}
	return time.Until(exp)
}

// TokenStore is a pluggable persistence backend for OAuth tokens.
// Implementations MUST be safe for concurrent use within a single process.
type TokenStore interface {
	// Load returns the persisted token, or (nil, nil) if none has been saved.
	Load(ctx context.Context) (*Token, error)
	// Save atomically persists the supplied token.
	Save(ctx context.Context, t *Token) error
	// Close releases any backing resources (file handles, DBs).
	Close() error
}

// ------------------------------------------------------------------
// Memory token store
// ------------------------------------------------------------------

type memoryTokenStore struct {
	mu  sync.RWMutex
	tok *Token
}

// NewMemoryTokenStore returns an in-process TokenStore. Useful for tests.
func NewMemoryTokenStore() TokenStore { return &memoryTokenStore{} }

// Load implements TokenStore.Load.
func (m *memoryTokenStore) Load(_ context.Context) (*Token, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.tok == nil {
		return nil, nil
	}
	clone := *m.tok
	return &clone, nil
}

// Save implements TokenStore.Save.
func (m *memoryTokenStore) Save(_ context.Context, t *Token) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t == nil {
		m.tok = nil
		return nil
	}
	clone := *t
	m.tok = &clone
	return nil
}

// Close implements TokenStore.Close.
func (m *memoryTokenStore) Close() error { return nil }

// ------------------------------------------------------------------
// File token store
// ------------------------------------------------------------------

// fileStoreConfig collects the tunable options for a file token store.
type fileStoreConfig struct {
	key  []byte
	mode os.FileMode
}

// FileStoreOption configures the file token store.
type FileStoreOption func(*fileStoreConfig)

// WithFileEncryption enables AES-256-GCM encryption of the token file.
// The provided key may be any length; it is HKDF-SHA256 expanded to 32 bytes.
// Empty keys disable encryption.
func WithFileEncryption(key []byte) FileStoreOption {
	return func(c *fileStoreConfig) {
		if len(key) > 0 {
			c.key = append([]byte(nil), key...)
		}
	}
}

// WithFileMode overrides the POSIX permissions (default 0600).
func WithFileMode(mode os.FileMode) FileStoreOption {
	return func(c *fileStoreConfig) { c.mode = mode }
}

// fileTokenStore persists a Token to a JSON file with atomic writes.
type fileTokenStore struct {
	path string
	cfg  fileStoreConfig
	mu   sync.Mutex
}

// NewFileTokenStore returns a TokenStore backed by a JSON file at path.
// Parent directories are created as needed. Writes use os.CreateTemp +
// os.Rename for atomicity and enforce mode 0600 by default.
func NewFileTokenStore(path string, opts ...FileStoreOption) (TokenStore, error) {
	if path == "" {
		return nil, errors.New("schwab: NewFileTokenStore requires a non-empty path")
	}
	cfg := fileStoreConfig{mode: 0o600}
	for _, o := range opts {
		o(&cfg)
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("schwab: create token dir: %w", err)
		}
	}
	return &fileTokenStore{path: path, cfg: cfg}, nil
}

// Load implements TokenStore.Load.
func (f *fileTokenStore) Load(_ context.Context) (*Token, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	raw, err := os.ReadFile(f.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("schwab: read token file: %w", err)
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var data []byte
	if len(f.cfg.key) > 0 || hasEncPrefix(raw) {
		plain, derr := decryptBlob(raw, f.cfg.key)
		if derr != nil {
			return nil, &AuthError{Stage: "parse", Err: fmt.Errorf("decrypt token file: %w", derr)}
		}
		data = plain
	} else {
		data = raw
	}
	var tok Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, &AuthError{Stage: "parse", Err: fmt.Errorf("unmarshal token file: %w", err)}
	}
	return &tok, nil
}

// Save implements TokenStore.Save.
func (f *fileTokenStore) Save(_ context.Context, t *Token) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t == nil {
		if err := os.Remove(f.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("schwab: remove token file: %w", err)
		}
		return nil
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("schwab: marshal token: %w", err)
	}
	if len(f.cfg.key) > 0 {
		data, err = encryptBlob(data, f.cfg.key)
		if err != nil {
			return fmt.Errorf("schwab: encrypt token: %w", err)
		}
	}
	return atomicWrite(f.path, data, f.cfg.mode)
}

// Close implements TokenStore.Close.
func (f *fileTokenStore) Close() error { return nil }

// atomicWrite writes data to path via a temp file + rename, applying the
// requested mode to both the temp file and the final name.
func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tok-*")
	if err != nil {
		return fmt.Errorf("schwab: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if we bail before rename.
	defer func() {
		if _, statErr := os.Stat(tmpName); statErr == nil {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return fmt.Errorf("schwab: chmod temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("schwab: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("schwab: sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("schwab: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("schwab: rename temp file: %w", err)
	}
	// Ensure mode on the final file (rename preserves perms but be explicit).
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("schwab: chmod token file: %w", err)
	}
	return nil
}

// ------------------------------------------------------------------
// Encryption helpers (AES-256-GCM + HKDF-SHA256)
// ------------------------------------------------------------------

const encPrefix = "enc:"

// hasEncPrefix reports whether data begins with the "enc:" magic.
func hasEncPrefix(data []byte) bool {
	return len(data) >= len(encPrefix) && string(data[:len(encPrefix)]) == encPrefix
}

// deriveKey HKDF-SHA256 expands raw to exactly 32 bytes.
func deriveKey(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("schwab: empty encryption key")
	}
	r := hkdf.New(sha256.New, raw, nil, []byte("goschwab:token:v1"))
	out := make([]byte, 32)
	if _, err := io.ReadFull(r, out); err != nil {
		return nil, fmt.Errorf("hkdf: %w", err)
	}
	return out, nil
}

// encryptBlob returns "enc:" + base64(nonce||ciphertext||tag).
func encryptBlob(plaintext, key []byte) ([]byte, error) {
	dk, err := deriveKey(key)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(dk)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("rand: %w", err)
	}
	sealed := gcm.Seal(nil, nonce, plaintext, nil)
	full := make([]byte, 0, len(nonce)+len(sealed))
	full = append(full, nonce...)
	full = append(full, sealed...)
	b64 := base64.StdEncoding.EncodeToString(full)
	return []byte(encPrefix + b64), nil
}

// decryptBlob inverts encryptBlob. Accepts input with or without the "enc:" prefix.
func decryptBlob(data, key []byte) ([]byte, error) {
	if hasEncPrefix(data) {
		data = data[len(encPrefix):]
	}
	// The blob may have trailing whitespace/newlines when hand-edited.
	for len(data) > 0 && (data[len(data)-1] == '\n' || data[len(data)-1] == '\r' || data[len(data)-1] == ' ') {
		data = data[:len(data)-1]
	}
	raw, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return nil, fmt.Errorf("base64: %w", err)
	}
	dk, err := deriveKey(key)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(dk)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns+gcm.Overhead() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := raw[:ns], raw[ns:]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("aead open: %w", err)
	}
	return plain, nil
}

// defaultTokenPath returns ~/.schwab/tokens.json.
func defaultTokenPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("schwab: resolve home dir: %w", err)
	}
	return filepath.Join(home, ".schwab", "tokens.json"), nil
}
