package store

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	lockName     = ".lock"
	lockTimeout  = 5 * time.Second
	lockInterval = 20 * time.Millisecond
)

// Lock takes an advisory lock on the store directory and returns the function
// that releases it. Exclusive locks are for read-modify-write cycles; shared
// locks let readers wait out a writer without blocking each other.
//
// The lock lives on a dedicated .lock file, never on a ticket or ROOT.md,
// because those are replaced by rename and a lock on a replaced file excludes
// nobody. The kernel drops the lock when the process exits, so a crash leaves
// nothing stale. It waits up to five seconds, then fails.
func (s *Store) Lock(exclusive bool) (func(), error) {
	f, err := os.OpenFile(filepath.Join(s.Dir, lockName), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}
	deadline := time.Now().Add(lockTimeout)
	for {
		ok, err := tryFlock(f, exclusive)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("lock %s: %w", s.Dir, err)
		}
		if ok {
			return func() { _ = f.Close() }, nil // closing the fd releases the lock
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, fmt.Errorf("timed out waiting for the lock on %s (another tk is running)", s.Dir)
		}
		time.Sleep(lockInterval)
	}
}
