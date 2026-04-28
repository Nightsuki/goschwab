//go:build unix

package schwab

import (
	"context"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"
)

// AcquireRefreshLock implements TokenStoreLocker using POSIX flock. The lock
// is held on a sibling "<path>.lock" file so it does not interfere with the
// atomic-rename semantics of Save on the token file itself.
//
// flock is advisory and per-open-file-description, so a single fcntl call
// would block the goroutine and leak it on ctx cancellation. We instead poll
// LOCK_EX|LOCK_NB on a short tick, which both honours ctx and avoids the
// thundering-herd issue (per-process contention rate caps at ~20Hz).
func (f *fileTokenStore) AcquireRefreshLock(ctx context.Context) (func(), error) {
	lockPath := f.path + ".lock"
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("schwab: open lock file: %w", err)
	}

	// Fast path: try non-blocking once before allocating a ticker.
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
		return makeFlockRelease(file), nil
	} else if err != syscall.EWOULDBLOCK {
		_ = file.Close()
		return nil, fmt.Errorf("schwab: flock: %w", err)
	}

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, ctx.Err()
		case <-ticker.C:
			err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
			if err == nil {
				return makeFlockRelease(file), nil
			}
			if err != syscall.EWOULDBLOCK {
				_ = file.Close()
				return nil, fmt.Errorf("schwab: flock: %w", err)
			}
		}
	}
}

// makeFlockRelease returns an idempotent release func that unlocks and closes
// the lock file exactly once. Subsequent calls are no-ops.
func makeFlockRelease(file *os.File) func() {
	var (
		mu       sync.Mutex
		released bool
	)
	return func() {
		mu.Lock()
		defer mu.Unlock()
		if released {
			return
		}
		released = true
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}
}
