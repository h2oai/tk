package cmd

import (
	"testing"
	"time"
)

// waits reports whether running tk with args blocks while the lock is held
// exclusively, and that it finishes once the lock is released.
func waits(t *testing.T, e *env, args ...string) bool {
	t.Helper()
	unlock, err := newApp(e.dir).Lock(true)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan int, 1)
	go func() {
		_, _, code, _ := e.runIn("", args...)
		done <- code
	}()
	blocked := false
	select {
	case <-done:
	case <-time.After(150 * time.Millisecond):
		blocked = true
	}
	unlock()
	if blocked {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatalf("tk %v still blocked after unlock", args)
		}
	}
	return blocked
}

func TestCommandsTakeTheLock(t *testing.T) {
	e := newEnv(t)
	id := e.newT("A")
	for _, args := range [][]string{
		{"new", "B"}, {"start", id}, {"type", id, "bug"}, {"note", id, "hi"},
		{"ls"}, {"show", id}, {"ready"}, {"fsck"},
	} {
		if !waits(t, e, args...) {
			t.Errorf("tk %v ran while the store was locked", args)
		}
	}
}

func TestTuiAndEditManageTheirOwnLock(t *testing.T) {
	// `edit` and `tui` must not hold the lock for the whole command (an
	// editor or interactive session can stay open for minutes). edit fails
	// fast here without $EDITOR, so it must finish even while locked.
	e := newEnv(t)
	id := e.newT("A")
	t.Setenv("EDITOR", "")
	if waits(t, e, "edit", id) {
		t.Error("edit blocked on the lock before reaching the editor")
	}
}
