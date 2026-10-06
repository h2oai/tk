package cmd

import (
	"strings"
	"testing"

	"github.com/h2oai/tk/internal/store"
)

func lsIDs(e *env) []string {
	var ids []string
	for _, l := range strings.Split(strings.TrimSpace(e.run("ls", "--all")), "\n") {
		f := strings.Fields(l)
		ids = append(ids, f[3])
	}
	return ids
}

func TestMv(t *testing.T) {
	e := newEnv(t)
	a, b, c := e.newT("A"), e.newT("B"), e.newT("C")
	k := e.newT("K", "--under", a)

	contains(t, e.run("mv", c, "--under", a), c+" 1.2 C")
	contains(t, e.run("mv", c, "--at", "1"), c+" 1.1 C")   // reposition only
	contains(t, e.run("mv", b, "--before", k), b+" 1.2 B") // parent from anchor
	contains(t, e.run("mv", b, "--after", a), b+" 2 B")    // anchor at root
	contains(t, e.run("mv", k, "--root", "--at", "1"), k+" 1 K")
	contains(t, e.run("mv", c, "--root"), c+" 4 C")

	for _, args := range [][]string{
		{"mv", a},
		{"mv", a, "--under", a[:3], "--root"},
		{"mv", a, "--before", b, "--after", c},
		{"mv", a, "--before", "nope"},
	} {
		if _, _, code := e.fail(args...); code != ExitGeneric {
			t.Errorf("%v: code %d", args, code)
		}
	}
}

func TestMvCycle(t *testing.T) {
	e := newEnv(t)
	a := e.newT("A")
	b := e.newT("B", "--under", a)
	c := e.newT("C", "--under", b)
	for _, under := range []string{a, b, c} {
		_, errs, code := e.fail("mv", a, "--under", under)
		if code != ExitGeneric || !strings.Contains(errs, "its own ancestor") {
			t.Errorf("under %s: code %d errs %q", under, code, errs)
		}
	}
	contains(t, e.run("ls"), "1.1.1 [ ] "+c)
}

func TestMvBadDepRule(t *testing.T) {
	e := newEnv(t)
	a, b := e.newT("A"), e.newT("B")
	e.run("dep", b, a)
	// moving B under A would make B depend on its own ancestor
	_, errs, code := e.fail("mv", b, "--under", a)
	if code != ExitGeneric || errs == "" {
		t.Errorf("code %d errs %q", code, errs)
	}
	contains(t, e.run("ls"), "2 [ ] "+b)
}

func TestShift(t *testing.T) {
	e := newEnv(t)
	a, b, c := e.newT("A"), e.newT("B"), e.newT("C")
	contains(t, e.run("up", c), c+" 2 C")
	contains(t, e.run("up", c), c+" 1 C")
	contains(t, e.run("up", c), c+" 1 C") // edge no-op
	contains(t, e.run("down", c), c+" 2 C")
	contains(t, e.run("bottom", c), c+" 3 C")
	contains(t, e.run("top", c), c+" 1 C")
	if got := lsIDs(e); strings.Join(got, ",") != strings.Join([]string{c, a, b}, ",") {
		t.Errorf("order %v", got)
	}
	if _, _, code := e.fail("top", "nope"); code != ExitGeneric {
		t.Error("expected failure")
	}
}

func TestDepUndep(t *testing.T) {
	e := newEnv(t)
	a, b := e.newT("A"), e.newT("B")
	k := e.newT("K", "--under", a)
	contains(t, e.run("dep", b[:3], a[:3]), b+" now waits on "+a)
	contains(t, e.run("show", b), a)
	e.run("dep", b, a) // idempotent

	for _, c := range []struct {
		args []string
		msg  string
	}{
		{[]string{"dep", a, b}, "cycle"},
		{[]string{"dep", a, a}, "itself"},
		{[]string{"dep", k, a}, "ancestor or descendant"},
		{[]string{"dep", a, k}, "ancestor or descendant"},
		{[]string{"dep", a, "nope"}, "not found"},
	} {
		_, errs, code := e.fail(c.args...)
		if code != ExitGeneric || !strings.Contains(errs, c.msg) {
			t.Errorf("%v: code %d errs %q", c.args, code, errs)
		}
	}
	contains(t, e.run("undep", b, a), b+" no longer waits on "+a)
	if strings.Contains(e.run("show", b), "blocked") {
		t.Error("still blocked")
	}
}

func TestUndepFailures(t *testing.T) {
	e := newEnv(t)
	a, b, c := e.newT("A"), e.newT("B"), e.newT("C")
	e.run("dep", a, b)
	for _, arg := range []string{"zzzz", "", c, "a"} { // typo, empty, non-blocker, ambiguous partial
		_, errs, code := e.fail("undep", a, arg)
		if code != ExitGeneric || errs == "" {
			t.Errorf("undep %q: code %d errs %q", arg, code, errs)
		}
	}
	_, errs, _ := e.fail("undep", a, c)
	contains(t, errs, "is not blocked by")
	contains(t, e.run("show", a), b) // untouched
	// Already removed: second undep fails.
	e.run("undep", a, b)
	e.fail("undep", a, b)

	// A dangling blocker can still be removed by its exact id.
	e.block(a, "gone9")
	contains(t, e.run("undep", a, "gone9"), a+" no longer waits on gone9")
	e.fail("undep", a, "gone9")
}

func TestRm(t *testing.T) {
	e := newEnv(t)
	a := e.newT("A")
	k := e.newT("K", "--under", a)
	b := e.newT("B")
	x := e.newT("X")
	e.run("dep", b, x)

	_, errs, code := e.fail("rm", a)
	if code != ExitGeneric || !strings.Contains(errs, "has children") {
		t.Errorf("code %d errs %q", code, errs)
	}
	_, errs, code = e.fail("rm", x)
	if code != ExitGeneric || !strings.Contains(errs, "blocks") || !strings.Contains(errs, b) {
		t.Errorf("code %d errs %q", code, errs)
	}
	if got := strings.Fields(e.run("rm", k)); len(got) != 1 || got[0] != k {
		t.Errorf("rm leaf: %v", got)
	}
	e.newT("K2", "--under", a)
	out := e.run("rm", "--force", a)
	if len(strings.Fields(out)) != 2 || !strings.Contains(out, a) {
		t.Errorf("rm --force: %q", out)
	}
	e.run("rm", "--force", x)
	if strings.Contains(e.run("show", b), "blocked") {
		t.Error("dep not detached")
	}
	contains(t, e.run("fsck"), "ok")
	if _, _, code := e.fail("show", a); code != ExitGeneric {
		t.Error("removed ticket still shows")
	}
}

func TestFsck(t *testing.T) {
	e := newEnv(t)
	contains(t, e.run("new", "A"), "") // create dir
	a := strings.TrimSpace(e.run("new", "B"))
	contains(t, e.run("fsck"), "ok")

	st := e.store()
	tk, err := st.Load(a)
	if err != nil {
		t.Fatal(err)
	}
	tk.Children = append(tk.Children, "zzzzz9")
	if err := st.Save(tk); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(&store.Ticket{ID: "orphn1", Status: store.StatusOpen, Type: store.TypeTask, Created: now(), Title: "Orphan"}); err != nil {
		t.Fatal(err)
	}
	out, _, code := e.fail("fsck")
	if code != ExitFsck {
		t.Errorf("code %d", code)
	}
	contains(t, out, "dangling: "+a)
	contains(t, out, "zzzzz9")
	contains(t, out, "orphan: orphn1")
	contains(t, out, "tk mv orphn1 --root")

	_, errOut, _, err := e.runIn("", "ls")
	if err != nil {
		t.Fatal(err)
	}
	contains(t, errOut, "1 unreachable ticket(s)")
}

func (e *env) store() *store.Store { return newApp(e.dir).Store() }

func TestMvWarnsAboveBlocker(t *testing.T) {
	e := newEnv(t)
	a, b, c := e.newT("A"), e.newT("B"), e.newT("C")
	e.run("dep", a, b) // A waits on B, A is above B
	_, errs, _, _ := e.runIn("", "mv", c, "--at", "1")
	if errs != "" {
		t.Errorf("unrelated move warned: %q", errs)
	}
	e.run("mv", a, "--after", b)
	_, errs, _, _ = e.runIn("", "mv", a, "--before", b)
	contains(t, errs, "warning: "+a+` "A"`)
	contains(t, errs, "is above its blocker "+b)
	_, errs, _, _ = e.runIn("", "down", c)
	if errs != "" {
		t.Errorf("preexisting misorder warned: %q", errs)
	}
	// moving the blocker below its dependent
	e.run("mv", a, "--after", b)
	_, errs, _, _ = e.runIn("", "bottom", b)
	contains(t, errs, "is above its blocker "+b)
	// closed blockers never warn
	e.run("close", b)
	_, errs, _, _ = e.runIn("", "top", a)
	if errs != "" {
		t.Errorf("closed blocker warned: %q", errs)
	}
}

func TestDepAndFsckWarnMisorder(t *testing.T) {
	e := newEnv(t)
	a, b := e.newT("A"), e.newT("B")
	_, errs, _, _ := e.runIn("", "dep", a, b)
	contains(t, errs, "is above its blocker "+b)
	out, _, code, err := e.runIn("", "fsck")
	if err != nil || code != 0 {
		t.Fatalf("fsck code %d err %v", code, err)
	}
	contains(t, out, "warn: "+a)
	e.run("mv", b, "--before", a)
	if out := e.run("fsck"); strings.TrimSpace(out) != "ok" {
		t.Errorf("fsck: %q", out)
	}
}
