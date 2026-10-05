package store

import (
	"strings"
	"testing"
	"time"
)

func TestLockExclusiveBlocksAndReleases(t *testing.T) {
	s := New(t.TempDir())
	unlock, err := s.Lock(true)
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan error, 1)
	go func() {
		u, err := s.Lock(true)
		if err == nil {
			u()
		}
		got <- err
	}()
	select {
	case err := <-got:
		t.Fatalf("second exclusive lock did not wait (err=%v)", err)
	case <-time.After(150 * time.Millisecond):
	}
	unlock()
	if err := <-got; err != nil {
		t.Fatalf("lock after release: %v", err)
	}
}

func TestLockSharedCoexists(t *testing.T) {
	s := New(t.TempDir())
	u1, err := s.Lock(false)
	if err != nil {
		t.Fatal(err)
	}
	defer u1()
	u2, err := s.Lock(false)
	if err != nil {
		t.Fatalf("second shared lock: %v", err)
	}
	u2()
}

func TestLockFileIsNotATicket(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	unlock, err := s.Lock(true)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ids, err := s.IDs()
	if err != nil || len(ids) != 0 {
		t.Fatalf("ids %v err %v", ids, err)
	}
	if strings.Contains(lockName, ".md") {
		t.Fatal("lock file must not look like a ticket")
	}
}
