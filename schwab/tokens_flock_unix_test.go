//go:build unix

package schwab

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestFileTokenStore_FlockMutualExclusion verifies that two FileTokenStores
// pointing at the same path serialize via flock: while one holds the lock,
// the other waits. After release, the second acquires.
func TestFileTokenStore_FlockMutualExclusion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")

	storeA, err := NewFileTokenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	storeB, err := NewFileTokenStore(path)
	if err != nil {
		t.Fatal(err)
	}

	lockerA, ok := storeA.(TokenStoreLocker)
	if !ok {
		t.Fatal("fileTokenStore must implement TokenStoreLocker on unix")
	}
	lockerB := storeB.(TokenStoreLocker)

	releaseA, err := lockerA.AcquireRefreshLock(context.Background())
	if err != nil {
		t.Fatalf("A acquire: %v", err)
	}

	// B should block until A releases.
	type result struct {
		dur time.Duration
		err error
	}
	done := make(chan result, 1)
	go func() {
		start := time.Now()
		release, err := lockerB.AcquireRefreshLock(context.Background())
		done <- result{time.Since(start), err}
		if release != nil {
			release()
		}
	}()

	// Hold A's lock briefly, then release.
	time.Sleep(150 * time.Millisecond)
	releaseA()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("B acquire: %v", r.err)
		}
		if r.dur < 100*time.Millisecond {
			t.Fatalf("B acquired too fast (%s) — flock not actually serializing", r.dur)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("B never acquired after A released")
	}
}

// TestFileTokenStore_FlockCtxTimeout verifies that AcquireRefreshLock returns
// ctx.DeadlineExceeded verbatim when the lock cannot be acquired in time.
func TestFileTokenStore_FlockCtxTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")

	storeA, _ := NewFileTokenStore(path)
	storeB, _ := NewFileTokenStore(path)

	releaseA, err := storeA.(TokenStoreLocker).AcquireRefreshLock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer releaseA()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = storeB.(TokenStoreLocker).AcquireRefreshLock(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected ctx.DeadlineExceeded, got %v", err)
	}
}

// TestFileTokenStore_FlockReleaseIdempotent verifies that calling release
// multiple times is safe (no panic, no error).
func TestFileTokenStore_FlockReleaseIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.json")

	store, _ := NewFileTokenStore(path)
	release, err := store.(TokenStoreLocker).AcquireRefreshLock(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release() // must not panic
		}()
	}
	wg.Wait()
}
