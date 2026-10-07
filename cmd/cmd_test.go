package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type env struct {
	t   *testing.T
	dir string
}

func newEnv(t *testing.T) *env { return &env{t: t, dir: filepath.Join(t.TempDir(), ".tickets")} }

// run executes tk in-process; it returns stdout, stderr text and the exit code.
func (e *env) runIn(stdin string, args ...string) (string, string, int, error) {
	root := newRootCmd()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(append([]string{"--dir", e.dir}, args...))
	err := root.Execute()
	code := 0
	if err != nil {
		code = report(&errb, err)
	}
	return out.String(), errb.String(), code, err
}

func (e *env) run(args ...string) string {
	e.t.Helper()
	out, errs, code, err := e.runIn("", args...)
	if err != nil {
		e.t.Fatalf("tk %v: code %d: %v\n%s", args, code, err, errs)
	}
	return out
}

func (e *env) fail(args ...string) (string, string, int) {
	e.t.Helper()
	out, errs, code, err := e.runIn("", args...)
	if err == nil {
		e.t.Fatalf("tk %v: expected failure", args)
	}
	return out, errs, code
}

func (e *env) newT(title string, args ...string) string {
	e.t.Helper()
	return strings.TrimSpace(e.run(append([]string{"new", title}, args...)...))
}

func contains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("want %q in:\n%s", want, got)
	}
}

func (e *env) block(id, blocker string) {
	e.t.Helper()
	st := newApp(e.dir).Store()
	tk, err := st.Load(id)
	if err != nil {
		e.t.Fatal(err)
	}
	tk.BlockedBy = append(tk.BlockedBy, blocker)
	if err := st.Save(tk); err != nil {
		e.t.Fatal(err)
	}
}

func newApp(dir string) *App { return &App{Dir: dir} }

func TestNewLsShow(t *testing.T) {
	e := newEnv(t)
	a := e.newT("Alpha", "-b", "alpha body")
	b := e.newT("Beta")
	c := e.newT("Child", "--under", a[:4])
	d := e.newT("First", "--at", "1")
	f := e.newT("AfterFirst", "--after", d)
	g := e.newT("BeforeBeta", "--before", b)

	out := e.run("ls")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"1 [ ] " + d, "2 [ ] " + f, "3 [ ] " + a, "  3.1 [ ] " + c, "4 [ ] " + g, "5 [ ] " + b}
	if len(lines) != len(want) {
		t.Fatalf("ls:\n%s", out)
	}
	for i, w := range want {
		contains(t, lines[i], w)
	}

	contains(t, e.run("ls", a), "3.1 [ ] "+c)
	if strings.Contains(e.run("ls", a), b) {
		t.Error("scoped ls leaked other tickets")
	}

	show := e.run("show", a)
	contains(t, show, "# Alpha")
	contains(t, show, "alpha body")
	contains(t, show, "position: 3")
	contains(t, show, "children:")
	contains(t, show, c)

	_, errs, code := e.fail("new", "x", "--under", "nope")
	if code != ExitGeneric || !strings.Contains(errs, "not found") {
		t.Errorf("code %d errs %q", code, errs)
	}
}

func TestNewBodyInput(t *testing.T) {
	e := newEnv(t)
	out, _, _, err := e.runIn("from stdin\n", "new", "S", "-b", "-")
	if err != nil {
		t.Fatal(err)
	}
	contains(t, e.run("show", strings.TrimSpace(out)), "from stdin")

	path := filepath.Join(t.TempDir(), "b.txt")
	os.WriteFile(path, []byte("from file\n"), 0o644)
	id := e.newT("F", "-F", path)
	contains(t, e.run("show", id), "from file")

	if _, _, code := e.fail("new", "X", "-b", "a", "-F", path); code != ExitGeneric {
		t.Errorf("code %d", code)
	}
}

func TestReadText(t *testing.T) {
	got, err := ReadText(strings.NewReader("in\n\n"), "-", true, "")
	if err != nil || got != "in" {
		t.Errorf("stdin: %q %v", got, err)
	}
	got, _ = ReadText(nil, "inline", true, "")
	if got != "inline" {
		t.Errorf("inline: %q", got)
	}
	got, _ = ReadText(nil, "", false, "")
	if got != "" {
		t.Errorf("absent: %q", got)
	}
	if _, err := ReadText(nil, "x", true, "f"); err == nil {
		t.Error("want conflict error")
	}
	if _, err := ReadText(nil, "", false, filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("want missing file error")
	}
}

func TestNote(t *testing.T) {
	e := newEnv(t)
	a := e.newT("Alpha", "-b", "body")
	e.run("note", a, "first note")
	out, _, _, err := e.runIn("piped note\n", "note", a, "-")
	if err != nil {
		t.Fatal(err)
	}
	contains(t, out, "noted "+a)
	path := filepath.Join(t.TempDir(), "n.txt")
	os.WriteFile(path, []byte("file note"), 0o644)
	e.run("note", a, "-F", path)
	show := e.run("show", a)
	for _, w := range []string{"body", "first note", "piped note", "file note"} {
		contains(t, show, w)
	}
	if strings.Count(show, "## Note ") != 3 {
		t.Errorf("want 3 notes:\n%s", show)
	}
	if _, _, code := e.fail("note", a); code != ExitGeneric {
		t.Errorf("empty note should fail")
	}
}

func TestStatusCommands(t *testing.T) {
	e := newEnv(t)
	p := e.newT("Parent")
	c := e.newT("Child", "--under", p)

	contains(t, e.run("start", c), c+" in_progress")
	_, errs, code := e.fail("close", p)
	if code != ExitGeneric || !strings.Contains(errs, "not closed") {
		t.Errorf("close parent: %d %q", code, errs)
	}
	contains(t, e.run("close", p, "--force"), c+" closed")
	if strings.Contains(e.run("ls"), p) {
		t.Error("closed subtree should be hidden")
	}
	contains(t, e.run("ls", "--all"), "[x] "+p)
	contains(t, e.run("reopen", c), c+" open")
	contains(t, e.run("show", p), "status:   open")
	contains(t, e.run("reopen", c), "unchanged")
}

func TestReadyExitCodes(t *testing.T) {
	e := newEnv(t)
	// No tickets at all: nothing left.
	e.run("new", "tmp")
	_, errs, code := e.fail("ready", "zzzz")
	if code != ExitGeneric || !strings.Contains(errs, "not found") {
		t.Errorf("unknown scope: %d %q", code, errs)
	}

	e = newEnv(t)
	a := e.newT("Alpha")
	b := e.newT("Beta")

	out := e.run("ready")
	contains(t, out, a+" 1 [task] Alpha")

	// blocked: Alpha waits on Beta.
	e.block(a, b)
	out, _, code = e.fail("ready")
	if code != 2 {
		t.Errorf("blocked code %d", code)
	}
	contains(t, out, "blocked: "+a+` "Alpha" waits on `+b+` "Beta" (2, open)`)

	// nothing left.
	e.run("close", a)
	e.run("close", b)
	out, errs, code = e.fail("ready")
	if code != 1 || out != "" || !strings.Contains(errs, "nothing left") {
		t.Errorf("nothing left: %d %q %q", code, out, errs)
	}
}

func TestReadyScope(t *testing.T) {
	e := newEnv(t)
	a := e.newT("A")
	b := e.newT("B")
	bc := e.newT("BChild", "--under", b)
	contains(t, e.run("ready"), a)
	contains(t, e.run("ready", b), bc+" 2.1 [task] BChild")
}

func TestEdit(t *testing.T) {
	e := newEnv(t)
	a := e.newT("Alpha")
	dir := t.TempDir()
	good := filepath.Join(dir, "good.sh")
	os.WriteFile(good, []byte("#!/bin/sh\nsed -i.bak 's/^# Alpha/# Renamed/' \"$1\"\n"), 0o755)
	t.Setenv("EDITOR", good)
	contains(t, e.run("edit", a), "edited "+a)
	contains(t, e.run("show", a), "# Renamed")

	t.Setenv("EDITOR", "true")
	contains(t, e.run("edit", a), "unchanged "+a)

	bad := filepath.Join(dir, "bad.sh")
	os.WriteFile(bad, []byte("#!/bin/sh\necho garbage > \"$1\"\n"), 0o755)
	t.Setenv("EDITOR", bad)
	_, errs, code := e.fail("edit", a)
	if code != ExitGeneric || !strings.Contains(errs, "invalid") {
		t.Errorf("bad edit: %d %q", code, errs)
	}
	contains(t, e.run("show", a), "# Renamed") // untouched

	t.Setenv("EDITOR", "")
	e.fail("edit", a)
}

func TestMissingDir(t *testing.T) {
	e := newEnv(t)
	_, errs, code := e.fail("ls")
	if code != ExitGeneric || !strings.Contains(errs, "no tickets directory") {
		t.Errorf("%d %q", code, errs)
	}
}

func TestExitErrorType(t *testing.T) {
	var ee *ExitError
	if !errors.As(error(&ExitError{Code: 2}), &ee) || ee.Code != 2 {
		t.Error("ExitError not matched")
	}
}

func TestGenericErrorsAvoidReadyCodes(t *testing.T) {
	e := newEnv(t)
	e.run("new", "x")
	for _, args := range [][]string{{"ready", "zzzz"}, {"show", "zzzz"}, {"bogus"}, {"ls", "--nope"}, {"show"}} {
		_, _, code := e.fail(args...)
		if code != ExitGeneric || code == 0 || code == 1 || code == 2 {
			t.Errorf("tk %v: code %d", args, code)
		}
	}
}

func TestCorruptRootRefusesMutations(t *testing.T) {
	e := newEnv(t)
	a, b := e.newT("A"), e.newT("B")
	rootPath := filepath.Join(e.dir, "ROOT.md")
	if err := os.WriteFile(rootPath, []byte("garbage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"new", "X"}, {"mv", a, "--root"}, {"rm", b}, {"close", a}, {"dep", a, b}} {
		_, errs, code := e.fail(args...)
		if code != ExitGeneric || !strings.Contains(errs, "refusing to modify") {
			t.Errorf("tk %v: code %d errs %q", args, code, errs)
		}
	}
	if got, _ := os.ReadFile(rootPath); string(got) != "garbage\n" {
		t.Errorf("ROOT.md was modified: %q", got)
	}
	// Reads and fsck still work and report the problem.
	e.run("ls")
	out, _, code := e.fail("fsck")
	if code != ExitFsck {
		t.Errorf("fsck code %d", code)
	}
	contains(t, out, "unreadable: ROOT")
}

func TestUnknownFrontmatterKeyIsReported(t *testing.T) {
	e := newEnv(t)
	a := e.newT("A")
	b := e.newT("B")
	path := filepath.Join(e.dir, b+".md")
	data, _ := os.ReadFile(path)
	bad := strings.Replace(string(data), "status:", "blockd-by: ["+a+"]\nstatus:", 1)
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, code := e.fail("fsck")
	if code != ExitFsck {
		t.Errorf("fsck code %d", code)
	}
	contains(t, out, "unreadable: "+b)
	contains(t, out, "blockd-by")
	_, errs, code := e.fail("new", "C")
	if code != ExitGeneric || !strings.Contains(errs, "refusing to modify") {
		t.Errorf("new: %d %q", code, errs)
	}
	if got, _ := os.ReadFile(path); string(got) != bad {
		t.Error("file rewritten")
	}
}

func TestTypes(t *testing.T) {
	e := newEnv(t)
	a := e.newT("Fix crash", "--type", "bug")
	b := e.newT("Plain")
	contains(t, e.run("show", a), "type:     bug")
	contains(t, e.run("show", b), "type:     task")
	contains(t, e.run("ls"), a+"  [bug] Fix crash")
	contains(t, e.run("ready"), a+" 1 [bug] Fix crash")

	_, errs, _ := e.fail("new", "X", "--type", "epic")
	contains(t, errs, "task, bug, feature, chore")

	// Case-insensitive, stored lowercase, works on closed tickets, no-op silent.
	e.run("close", b)
	contains(t, e.run("type", b, "FEATURE"), "feature")
	contains(t, e.run("show", b), "type:     feature")
	contains(t, e.run("show", b), "status:   closed")
	if out := e.run("type", b, "feature"); out != "" {
		t.Errorf("no-op printed %q", out)
	}
	raw, err := os.ReadFile(filepath.Join(e.dir, b+".md"))
	if err != nil || !strings.Contains(string(raw), "type: feature\n") {
		t.Errorf("file: %v\n%s", err, raw)
	}
	_, errs, _ = e.fail("type", b, "epic")
	contains(t, errs, "task, bug, feature, chore")
}

func TestArchive(t *testing.T) {
	e := newEnv(t)
	p := e.newT("Parent")
	c := e.newT("Child", "--under", p)
	other := e.newT("Other")

	_, errs, code := e.fail("archive", p)
	if code != ExitGeneric || !strings.Contains(errs, "not closed") {
		t.Errorf("archive open: %d %q", code, errs)
	}
	e.run("dep", other, c)
	_, errs, _ = e.fail("archive", p, "--force")
	contains(t, errs, other+" waits on "+c)
	e.run("undep", other, c)

	out := e.run("archive", p, "--force")
	if out != p+" archived \"Parent\"\n"+c+" archived \"Child\"\n" {
		t.Errorf("archive output:\n%s", out)
	}
	for _, f := range []string{p, c} {
		if _, err := os.Stat(filepath.Join(e.dir, "archive", f+".md")); err != nil {
			t.Error(err)
		}
	}
	if ls := e.run("ls", "--all"); strings.Contains(ls, p) || !strings.Contains(ls, other) {
		t.Errorf("ls:\n%s", ls)
	}
	_, errs, _ = e.fail("show", c)
	contains(t, errs, "not found")
	contains(t, e.run("fsck"), "ok")
}

func TestArchiveAll(t *testing.T) {
	e := newEnv(t)
	done := e.newT("Done")
	held := e.newT("Held")
	waiter := e.newT("Waiter")
	e.run("dep", waiter, held)
	e.run("close", done)
	e.run("close", held)

	for _, args := range [][]string{{"archive", "--all", done}, {"archive", "--all", "--force"}, {"archive"}} {
		if _, _, code := e.fail(args...); code != ExitGeneric {
			t.Errorf("%v: code %d", args, code)
		}
	}

	out, errs, _, err := e.runIn("", "archive", "--all", "-n")
	if err != nil || out != done+" would archive \"Done\"\n" || errs != "skipped "+held+": blocks "+waiter+"\n" {
		t.Errorf("dry run: %v\nout: %q\nerr: %q", err, out, errs)
	}
	if out := e.run("archive", done, "--dry-run"); out != done+" would archive \"Done\"\n" {
		t.Errorf("single dry run: %q", out)
	}
	if _, err := os.Stat(filepath.Join(e.dir, "archive")); !os.IsNotExist(err) {
		t.Errorf("dry run created archive dir: %v", err)
	}

	out, errs, _, err = e.runIn("", "archive", "--all")
	if err != nil || out != done+" archived \"Done\"\n" || errs != "skipped "+held+": blocks "+waiter+"\n" {
		t.Errorf("archive --all: %v\nout: %q\nerr: %q", err, out, errs)
	}
	if ls := e.run("ls", "--all"); strings.Contains(ls, done) || !strings.Contains(ls, held) {
		t.Errorf("ls:\n%s", ls)
	}
	if out, errs, _, err = e.runIn("", "archive", "--all"); err != nil || out != "" {
		t.Errorf("second run: %v %q %q", err, out, errs)
	}
}
