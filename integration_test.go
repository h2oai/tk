package main_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// tkRun runs the built binary in dir and returns stdout, stderr and the exit code.
func tkRun(t *testing.T, bin, dir string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("tk %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return strings.TrimSpace(out.String()), strings.TrimSpace(errb.String()), code
}

func TestEndToEnd(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "tk")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	dir := t.TempDir()

	run := func(args ...string) string {
		t.Helper()
		out, errs, code := tkRun(t, bin, dir, args...)
		if code != 0 {
			t.Fatalf("tk %v: exit %d: %s %s", args, code, out, errs)
		}
		return out
	}
	wantExit := func(want int, args ...string) string {
		t.Helper()
		out, errs, code := tkRun(t, bin, dir, args...)
		if code != want {
			t.Fatalf("tk %v: exit %d, want %d: %s %s", args, code, want, out, errs)
		}
		return out + errs
	}
	readyID := func() string {
		t.Helper()
		f := strings.Fields(run("ready"))
		if len(f) == 0 {
			t.Fatal("empty ready output")
		}
		return f[0]
	}

	// Epic with three children, plus a second root.
	epic := run("new", "Epic")
	a := run("new", "A", "--under", epic)
	b := run("new", "B", "--under", epic)
	c := run("new", "C", "--under", epic)
	other := run("new", "Other", "--type", "chore")
	run("type", other, "bug")
	if ls := run("ls"); !strings.Contains(ls, other+"  [bug] Other") {
		t.Fatalf("ls should show type:\n%s", ls)
	}

	if got := readyID(); got != a {
		t.Fatalf("ready = %s, want %s (A)", got, a)
	}

	// start/close advance.
	run("start", a)
	if got := readyID(); got != a {
		t.Fatalf("in_progress leaf should be resumed, got %s", got)
	}
	run("note", a, "halfway there")
	run("close", a)
	if got := readyID(); got != b {
		t.Fatalf("ready = %s, want %s (B)", got, b)
	}

	// Reorder: move C before B.
	run("mv", c, "--before", b)
	if got := readyID(); got != c {
		t.Fatalf("after mv ready = %s, want %s (C)", got, c)
	}

	// Dependency disagreeing with order: C waits on B -> exit 2.
	run("dep", c, b)
	msg := wantExit(2, "ready")
	if !strings.Contains(msg, "blocked:") || !strings.Contains(msg, b) {
		t.Fatalf("blocked message = %q", msg)
	}
	// Scoped ready is also blocked, and exit 1 when scope is done.
	wantExit(2, "ready", epic)

	// Fix by moving B to the top.
	run("top", b)
	if got := readyID(); got != b {
		t.Fatalf("after top ready = %s, want %s", got, b)
	}
	run("start", b)
	run("close", b)
	if got := readyID(); got != c {
		t.Fatalf("ready = %s, want %s (C)", got, c)
	}

	// Closing the epic fails while C is open; --force cascades.
	wantExit(3, "close", epic)
	run("close", "--force", epic)
	if got := readyID(); got != other {
		t.Fatalf("ready = %s, want %s (Other)", got, other)
	}
	if ls := run("ls", "--all"); !strings.Contains(ls, "[x]") {
		t.Fatalf("ls --all should show closed tickets:\n%s", ls)
	}

	// rm refuses a ticket with children, --force removes the subtree.
	wantExit(3, "rm", epic)
	run("rm", "--force", epic)
	if _, err := os.Stat(filepath.Join(dir, ".tickets", epic+".md")); !os.IsNotExist(err) {
		t.Fatalf("epic file should be gone: %v", err)
	}

	// Finish the remaining work.
	run("start", other)
	run("close", other)
	wantExit(1, "ready")

	if out := run("fsck"); !strings.Contains(out, "ok") {
		t.Fatalf("fsck = %q", out)
	}
}

// TestConcurrentWriters runs many `tk new` processes at once; without the
// store lock they overwrite each other's ROOT.md and tickets go missing.
func TestConcurrentWriters(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "tk")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	dir := t.TempDir()
	const n = 30
	errs := make(chan string, n)
	for i := 0; i < n; i++ {
		go func() {
			out, e, code := tkRun(t, bin, dir, "new", "t")
			if code != 0 {
				errs <- out + e
				return
			}
			errs <- ""
		}()
	}
	for i := 0; i < n; i++ {
		if e := <-errs; e != "" {
			t.Fatalf("tk new failed: %s", e)
		}
	}
	out, _, _ := tkRun(t, bin, dir, "ls")
	if got := len(strings.Split(out, "\n")); got != n {
		t.Fatalf("ls shows %d tickets, want %d:\n%s", got, n, out)
	}
	if out, _, code := tkRun(t, bin, dir, "fsck"); code != 0 {
		t.Fatalf("fsck: %s", out)
	}
}
