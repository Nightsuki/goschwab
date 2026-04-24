package schwab

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMemoryTokenStore_RoundTrip(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryTokenStore()
	t.Cleanup(func() { _ = s.Close() })

	got, err := s.Load(ctx)
	if err != nil {
		t.Fatalf("load empty: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil on empty store")
	}

	tok := &Token{AccessToken: "A", RefreshToken: "R", ExpiresIn: 1800, AccessTokenIssued: time.Now(), RefreshTokenIssued: time.Now()}
	if err := s.Save(ctx, tok); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.AccessToken != "A" || loaded.RefreshToken != "R" {
		t.Fatalf("roundtrip mismatch: %+v", loaded)
	}
}

func TestFileTokenStore_PlaintextRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")

	s, err := NewFileTokenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// Empty path -> (nil, nil)
	got, err := s.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("expected nil on missing file")
	}

	tok := sampleToken()
	if err := s.Save(ctx, tok); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.AccessToken != tok.AccessToken {
		t.Fatalf("roundtrip: got %+v", loaded)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode: got %o want 0600", info.Mode().Perm())
		}
	}
}

func TestFileTokenStore_EncryptedRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")
	key := []byte("this-is-a-test-encryption-key-123")

	s, err := NewFileTokenStore(path, WithFileEncryption(key))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	tok := sampleToken()
	if err := s.Save(ctx, tok); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), encPrefix) {
		t.Fatalf("expected %q prefix, got %q", encPrefix, string(raw[:min(len(raw), 16)]))
	}
	loaded, err := s.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.AccessToken != tok.AccessToken {
		t.Fatalf("encrypted roundtrip: got %+v", loaded)
	}

	// Wrong key should fail with typed error.
	bad, err := NewFileTokenStore(path, WithFileEncryption([]byte("different-key-entirely-different")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = bad.Close() })
	_, err = bad.Load(ctx)
	if err == nil {
		t.Fatal("expected decrypt failure with wrong key")
	}
	var ae *AuthError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AuthError, got %T: %v", err, err)
	}
}

func TestFileTokenStore_CorruptFileReturnsTypedError(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")
	if err := os.WriteFile(path, []byte("not json at all"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := NewFileTokenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	_, err = s.Load(ctx)
	if err == nil {
		t.Fatal("expected parse error")
	}
	var ae *AuthError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AuthError, got %T: %v", err, err)
	}
	if ae.Stage != "parse" {
		t.Fatalf("stage: %q", ae.Stage)
	}
}

func TestFileTokenStore_ConcurrentSave(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")
	s, err := NewFileTokenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	const n = 10
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			tok := sampleToken()
			tok.AccessToken = "A" + string(rune('0'+i))
			if err := s.Save(ctx, tok); err != nil {
				t.Errorf("save %d: %v", i, err)
			}
		}()
	}
	wg.Wait()

	loaded, err := s.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || !strings.HasPrefix(loaded.AccessToken, "A") {
		t.Fatalf("concurrent save result: %+v", loaded)
	}
}

func TestFileTokenStore_NilDeletes(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")
	s, err := NewFileTokenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if err := s.Save(ctx, sampleToken()); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file gone; stat err: %v", err)
	}
}

func TestToken_RemainingAndExpiresAt(t *testing.T) {
	now := time.Now().UTC()
	tok := &Token{
		AccessTokenIssued:  now.Add(-5 * time.Minute),
		RefreshTokenIssued: now.Add(-24 * time.Hour),
		ExpiresIn:          1800,
	}
	if r := tok.AccessTokenRemaining(); r <= 0 || r > 30*time.Minute {
		t.Fatalf("access remaining: %s", r)
	}
	if r := tok.RefreshTokenRemaining(); r <= 0 || r > 7*24*time.Hour {
		t.Fatalf("refresh remaining: %s", r)
	}
	if tok.AccessTokenExpiresAt().IsZero() {
		t.Fatal("access expiry zero")
	}
	if tok.RefreshTokenExpiresAt().IsZero() {
		t.Fatal("refresh expiry zero")
	}
}

func sampleToken() *Token {
	now := time.Now().UTC()
	return &Token{
		AccessToken:        "access-123",
		RefreshToken:       "refresh-456",
		TokenType:          "Bearer",
		Scope:              "api",
		ExpiresIn:          1800,
		AccessTokenIssued:  now,
		RefreshTokenIssued: now,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
