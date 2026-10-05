package tree

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/h2oai/tk/internal/store"
)

// build creates a tree from an indented outline: two spaces per level, each
// line "id" or "id:status". Ids are used verbatim.
func build(t *testing.T, outline string) (*Tree, *store.Store) {
	t.Helper()
	st := store.New(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	tr, err := Load(st)
	if err != nil {
		t.Fatal(err)
	}
	var stack []string
	for _, line := range strings.Split(strings.Trim(outline, "\n"), "\n") {
		if line == "" {
			continue
		}
		depth := (len(line) - len(strings.TrimLeft(line, " "))) / 2
		id, status, _ := strings.Cut(strings.TrimSpace(line), ":")
		if status == "" {
			status = "open"
		}
		stack = stack[:depth]
		parent := ""
		if depth > 0 {
			parent = stack[depth-1]
		}
		tk := &store.Ticket{ID: id, Status: store.Status(status), Created: time.Now().UTC(), Title: "T " + id}
		if err := tr.Add(tk, parent, Place{}); err != nil {
			t.Fatalf("add %s: %v", id, err)
		}
		stack = append(stack, id)
	}
	return tr, st
}

// reloaded reads the tree back from disk to check persistence.
func reloaded(t *testing.T, st *store.Store) *Tree {
	t.Helper()
	tr, err := Load(st)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

func outline(tr *Tree) string {
	var sb strings.Builder
	var walk func(ids []string, d int)
	walk = func(ids []string, d int) {
		for _, id := range ids {
			sb.WriteString(strings.Repeat(" ", d) + id + "\n")
			walk(tr.Get(id).Children, d+1)
		}
	}
	walk(tr.roots, 0)
	return sb.String()
}

const sample = `
a
  a1
  a2
    a2x
b
c
  c1
`

func TestPositionsAndQueries(t *testing.T) {
	tr, st := build(t, sample)
	tr = reloaded(t, st)
	tests := []struct{ id, pos string }{{"a", "1"}, {"a1", "1.1"}, {"a2x", "1.2.1"}, {"b", "2"}, {"c1", "3.1"}}
	for _, tt := range tests {
		if got := tr.Position(tt.id); got != tt.pos {
			t.Errorf("Position(%s) = %q, want %q", tt.id, got, tt.pos)
		}
	}
	if got, want := tr.Order(), strings.Fields("a a1 a2 a2x b c c1"); !reflect.DeepEqual(got, want) {
		t.Errorf("Order = %v", got)
	}
	if got, want := tr.Ancestors("a2x"), []string{"a2", "a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Ancestors = %v", got)
	}
	if got, want := tr.Descendants("a"), []string{"a1", "a2", "a2x"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Descendants = %v", got)
	}
	if !tr.IsLeaf("a1") || tr.IsLeaf("a") {
		t.Error("IsLeaf wrong")
	}
	if tr.Parent("a2") != "a" || tr.Parent("a") != "" {
		t.Error("Parent wrong")
	}
	if id, err := tr.Resolve("2x"); err != nil || id != "a2x" {
		t.Errorf("Resolve = %q, %v", id, err)
	}
}

func TestReady(t *testing.T) {
	tests := []struct {
		name    string
		outline string
		deps    [][2]string
		scope   string
		want    Outcome
		id      string // expected ticket id
		blocker string
	}{
		{"first open leaf", "a\n  a1\n  a2\nb", nil, "", Found, "a1", ""},
		{"in_progress beats earlier open", "a\n  a1\n  a2:in_progress\nb", nil, "", Found, "a2", ""},
		{"skips closed", "a:closed\nb:closed\nc", nil, "", Found, "c", ""},
		{"nothing left", "a:closed\n  a1:closed", nil, "", NothingLeft, "", ""},
		{"empty", "", nil, "", NothingLeft, "", ""},
		{"wrap-up parent", "a\n  a1:closed\nb", nil, "", Found, "a", ""},
		{"closed parent open child", "a:closed\n  a1\n", nil, "", Found, "a1", ""},
		{"parent with open child not a leaf", "a\n  a1\nb", nil, "", Found, "a1", ""},
		{"scoped", "a\n  a1\nb\n  b1\n  b2", nil, "b", Found, "b1", ""},
		{"scoped nothing", "a\n  a1\nb:closed\n  b1:closed", nil, "b", NothingLeft, "", ""},
		{"scoped in_progress outside ignored", "a:in_progress\nb\n  b1", nil, "b", Found, "b1", ""},
		{"blocked by own dep", "a\nb", [][2]string{{"a", "b"}}, "", Blocked, "a", "b"},
		{"blocked does not skip", "a\n  a1\n  a2\nb", [][2]string{{"a1", "b"}}, "", Blocked, "a1", "b"},
		{"blocked by ancestor dep", "a\n  a1\nb", [][2]string{{"a", "b"}}, "", Blocked, "a1", "b"},
		{"closed blocker satisfied", "a\nb:closed", [][2]string{{"a", "b"}}, "", Found, "a", ""},
		{"in_progress blocker still blocks", "a\nb:in_progress", [][2]string{{"a", "b"}}, "", Found, "b", ""},
		{"scoped inherits outside ancestor block", "p\n  a\nx", [][2]string{{"p", "x"}}, "a", Blocked, "a", "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, _ := build(t, tt.outline)
			for _, d := range tt.deps {
				if err := tr.AddDep(d[0], d[1]); err != nil {
					t.Fatal(err)
				}
			}
			res, err := tr.Ready(tt.scope)
			if err != nil {
				t.Fatal(err)
			}
			if res.Outcome != tt.want {
				t.Fatalf("outcome = %v, want %v", res.Outcome, tt.want)
			}
			if tt.id != "" && res.Ticket.ID != tt.id {
				t.Errorf("ticket = %s, want %s", res.Ticket.ID, tt.id)
			}
			if tt.blocker != "" && res.Block.Blocker.ID != tt.blocker {
				t.Errorf("blocker = %s, want %s", res.Block.Blocker.ID, tt.blocker)
			}
		})
	}
}

func TestReadyBlockedMessage(t *testing.T) {
	tr, _ := build(t, "a\nb\n  b1")
	if err := tr.AddDep("a", "b1"); err != nil {
		t.Fatal(err)
	}
	res, _ := tr.Ready("")
	got := res.Block.Describe(res.Ticket)
	if want := `a "T a" waits on b1 "T b1" (2.1, open)`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if _, err := tr.Ready("nope"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown scope err = %v", err)
	}
}

func TestStatus(t *testing.T) {
	t.Run("close fails with open descendants", func(t *testing.T) {
		tr, st := build(t, "a\n  a1\n  a2:closed")
		if _, err := tr.Close("a", false); !errors.Is(err, ErrOpenDescendants) {
			t.Fatalf("err = %v", err)
		}
		if tr.Get("a").Status != store.StatusOpen {
			t.Error("status changed on failure")
		}
		got, err := tr.Close("a", true)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, []string{"a1", "a"}) {
			t.Errorf("changed = %v", got)
		}
		r := reloaded(t, st)
		for _, id := range []string{"a", "a1", "a2"} {
			if r.Get(id).Status != store.StatusClosed {
				t.Errorf("%s not closed on disk", id)
			}
		}
	})
	t.Run("close parent when children closed", func(t *testing.T) {
		tr, _ := build(t, "a\n  a1:closed")
		if _, err := tr.Close("a", false); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("close idempotent", func(t *testing.T) {
		tr, _ := build(t, "a:closed")
		if got, err := tr.Close("a", false); err != nil || len(got) != 0 {
			t.Errorf("got %v, %v", got, err)
		}
	})
	t.Run("start leaf", func(t *testing.T) {
		tr, st := build(t, "a\n  a1")
		if _, err := tr.Start("a1"); err != nil {
			t.Fatal(err)
		}
		if reloaded(t, st).Get("a1").Status != store.StatusInProgress {
			t.Error("not persisted")
		}
	})
	t.Run("start non-leaf with open child fails", func(t *testing.T) {
		tr, _ := build(t, "a\n  a1")
		if _, err := tr.Start("a"); !errors.Is(err, ErrNotLeaf) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("start reopens closed ancestors", func(t *testing.T) {
		tr, _ := build(t, "a:closed\n  a1:closed")
		got, err := tr.Start("a1")
		if err != nil || !reflect.DeepEqual(got, []string{"a1", "a"}) {
			t.Fatalf("got %v, %v", got, err)
		}
		if tr.Get("a").Status != store.StatusOpen {
			t.Error("ancestor not reopened")
		}
	})
	t.Run("reopen", func(t *testing.T) {
		tr, _ := build(t, "a:closed\n  a1:closed")
		if _, err := tr.Reopen("a1"); err != nil {
			t.Fatal(err)
		}
		if tr.Get("a1").Status != store.StatusOpen || tr.Get("a").Status != store.StatusOpen {
			t.Error("reopen did not cascade up")
		}
	})
	t.Run("unknown", func(t *testing.T) {
		tr, _ := build(t, "a")
		if _, err := tr.Close("zz", false); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestAdd(t *testing.T) {
	newTk := func(id string) *store.Ticket {
		return &store.Ticket{ID: id, Status: store.StatusOpen, Created: time.Now().UTC(), Title: id}
	}
	tests := []struct {
		name   string
		parent string
		place  Place
		want   string
		err    error
	}{
		{"append root", "", Place{}, "a\n a1\n a2\nb\nn\n", nil},
		{"append child", "a", Place{}, "a\n a1\n a2\n n\nb\n", nil},
		{"index 1", "a", Place{Index: 1}, "a\n n\n a1\n a2\nb\n", nil},
		{"index -1 appends", "a", Place{Index: -1}, "a\n a1\n a2\n n\nb\n", nil},
		{"index beyond clamps", "a", Place{Index: 99}, "a\n a1\n a2\n n\nb\n", nil},
		{"before", "a", Place{Before: "a2"}, "a\n a1\n n\n a2\nb\n", nil},
		{"after", "a", Place{After: "a1"}, "a\n a1\n n\n a2\nb\n", nil},
		{"after at root", "", Place{After: "a"}, "a\n a1\n a2\nn\nb\n", nil},
		{"anchor not sibling", "a", Place{Before: "b"}, "", ErrBadPlace},
		{"two modes", "a", Place{Index: 1, Before: "a1"}, "", ErrBadPlace},
		{"bad index", "a", Place{Index: -5}, "", ErrBadPlace},
		{"missing parent", "zz", Place{}, "", store.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, st := build(t, "a\n  a1\n  a2\nb")
			err := tr.Add(newTk("n"), tt.parent, tt.place)
			if !errors.Is(err, tt.err) {
				t.Fatalf("err = %v, want %v", err, tt.err)
			}
			if tt.err != nil {
				if outline(tr) != "a\n a1\n a2\nb\n" || tr.Get("n") != nil {
					t.Error("failed add changed memory state")
				}
				return
			}
			if got := outline(tr); got != tt.want {
				t.Errorf("memory:\n%s\nwant:\n%s", got, tt.want)
			}
			if got := outline(reloaded(t, st)); got != tt.want {
				t.Errorf("disk:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
	t.Run("duplicate id", func(t *testing.T) {
		tr, _ := build(t, "a")
		if err := tr.Add(newTk("a"), "", Place{}); !errors.Is(err, ErrExists) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("parent status reset", func(t *testing.T) {
		for _, s := range []string{"closed", "in_progress"} {
			tr, st := build(t, "p:"+s+"\ngp:closed\n  q:closed")
			if err := tr.Add(newTk("n"), "p", Place{}); err != nil {
				t.Fatal(err)
			}
			if got := reloaded(t, st).Get("p").Status; got != store.StatusOpen {
				t.Errorf("%s parent -> %s", s, got)
			}
			if err := tr.Add(newTk("m"), "q", Place{}); err != nil {
				t.Fatal(err)
			}
			if tr.Get("q").Status != store.StatusOpen || tr.Get("gp").Status != store.StatusOpen {
				t.Error("closed ancestors not reopened")
			}
		}
	})
	t.Run("with blocked-by", func(t *testing.T) {
		tr, _ := build(t, "a\nb")
		tk := newTk("n")
		tk.BlockedBy = []string{"a"}
		if err := tr.Add(tk, "", Place{}); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tr.Get("n").BlockedBy, []string{"a"}) {
			t.Error("blocked-by lost")
		}
		bad := newTk("m")
		bad.BlockedBy = []string{"n", "zz"}
		if err := tr.Add(bad, "", Place{}); !errors.Is(err, store.ErrNotFound) || tr.Get("m") != nil {
			t.Errorf("err = %v", err)
		}
	})
}

func TestMove(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		parent string
		place  Place
		want   string
		err    error
	}{
		{"reparent", "a1", "b", Place{}, "a\n a2\n  a2x\nb\n a1\nc\n c1\n", nil},
		{"to root first", "a2x", "", Place{Index: 1}, "a2x\na\n a1\n a2\nb\nc\n c1\n", nil},
		{"within siblings", "a2", "a", Place{Index: 1}, "a\n a2\n  a2x\n a1\nb\nc\n c1\n", nil},
		{"root reorder after", "a", "", Place{After: "c"}, "b\nc\n c1\na\n a1\n a2\n  a2x\n", nil},
		{"root reorder before", "c", "", Place{Before: "a"}, "c\n c1\na\n a1\n a2\n  a2x\nb\n", nil},
		{"under self", "a", "a", Place{}, "", ErrCycle},
		{"under descendant", "a", "a2x", Place{}, "", ErrCycle},
		{"missing", "zz", "", Place{}, "", store.ErrNotFound},
		{"missing parent", "a", "zz", Place{}, "", store.ErrNotFound},
		{"bad anchor", "a1", "b", Place{Before: "a2"}, "", ErrBadPlace},
		{"anchor is self", "a1", "a", Place{Before: "a1"}, "", ErrBadPlace},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, st := build(t, sample)
			err := tr.Move(tt.id, tt.parent, tt.place)
			if !errors.Is(err, tt.err) {
				t.Fatalf("err = %v, want %v", err, tt.err)
			}
			if tt.err != nil {
				if got := outline(reloaded(t, st)); got != "a\n a1\n a2\n  a2x\nb\nc\n c1\n" {
					t.Errorf("failed move changed disk:\n%s", got)
				}
				return
			}
			if got := outline(tr); got != tt.want {
				t.Errorf("memory:\n%s\nwant:\n%s", got, tt.want)
			}
			if got := outline(reloaded(t, st)); got != tt.want {
				t.Errorf("disk:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestMoveTouchesOnlyAffectedFiles(t *testing.T) {
	tr, st := build(t, sample)
	snap := func() map[string]string {
		m := map[string]string{}
		for _, id := range []string{"a", "a1", "a2", "a2x", "b", "c", "c1"} {
			tk, _ := st.Load(id)
			d, _ := store.Marshal(tk)
			m[id] = string(d)
		}
		return m
	}
	before := snap()
	if err := tr.Move("a1", "c", Place{}); err != nil {
		t.Fatal(err)
	}
	after := snap()
	for id := range before {
		changed := before[id] != after[id]
		if want := id == "a" || id == "c"; changed != want {
			t.Errorf("%s changed = %v, want %v", id, changed, want)
		}
	}
}

func TestMoveStatusReset(t *testing.T) {
	tr, _ := build(t, "a\nb:closed")
	if err := tr.Move("a", "b", Place{}); err != nil {
		t.Fatal(err)
	}
	if tr.Get("b").Status != store.StatusOpen {
		t.Error("closed parent not reset")
	}
	tr, _ = build(t, "a:closed\nb:closed")
	if err := tr.Move("a", "b", Place{}); err != nil {
		t.Fatal(err)
	}
	if tr.Get("b").Status != store.StatusClosed {
		t.Error("closed subtree should not reopen closed parent")
	}
}

func TestMoveRechecksDeps(t *testing.T) {
	tests := []struct {
		name    string
		outline string
		deps    [][2]string
		id, to  string
		wantErr error
	}{
		{"into blocker's subtree", "a\nb\nc", [][2]string{{"a", "b"}}, "a", "b", ErrRelatedDep},
		{"blocker into dependant's subtree", "a\nb", [][2]string{{"a", "b"}}, "b", "a", ErrRelatedDep},
		{"unrelated ok", "a\nb\nc", [][2]string{{"a", "b"}}, "a", "c", nil},
		{"ancestor would wait on descendant", "p\n  x\ny", [][2]string{{"p", "y"}}, "y", "x", ErrRelatedDep},
		{"inherited blocker cycle", "q\nb\nx", [][2]string{{"q", "b"}, {"b", "x"}}, "x", "q", ErrDepCycle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, st := build(t, tt.outline)
			for _, d := range tt.deps {
				if err := tr.AddDep(d[0], d[1]); err != nil {
					t.Fatalf("setup dep: %v", err)
				}
			}
			before := outline(tr)
			err := tr.Move(tt.id, tt.to, Place{})
			if !errors.Is(err, tt.wantErr) && (err != nil || tt.wantErr != nil) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil && (outline(tr) != before || outline(reloaded(t, st)) != before) {
				t.Error("rejected move left changes behind")
			}
		})
	}
}

func TestReorder(t *testing.T) {
	tests := []struct {
		name string
		op   func(*Tree, string) error
		id   string
		want string
	}{
		{"up", (*Tree).Up, "a2", "a\n a2\n  a2x\n a1\nb\nc\n c1\n"},
		{"up at top", (*Tree).Up, "a1", "a\n a1\n a2\n  a2x\nb\nc\n c1\n"},
		{"down", (*Tree).Down, "a1", "a\n a2\n  a2x\n a1\nb\nc\n c1\n"},
		{"down at bottom", (*Tree).Down, "c", "a\n a1\n a2\n  a2x\nb\nc\n c1\n"},
		{"top root", (*Tree).Top, "c", "c\n c1\na\n a1\n a2\n  a2x\nb\n"},
		{"bottom root", (*Tree).Bottom, "a", "b\nc\n c1\na\n a1\n a2\n  a2x\n"},
		{"top only child", (*Tree).Top, "c1", "a\n a1\n a2\n  a2x\nb\nc\n c1\n"},
		{"bottom child", (*Tree).Bottom, "a1", "a\n a2\n  a2x\n a1\nb\nc\n c1\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, st := build(t, sample)
			if err := tt.op(tr, tt.id); err != nil {
				t.Fatal(err)
			}
			if got := outline(reloaded(t, st)); got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
	tr, _ := build(t, "a")
	if err := tr.Up("zz"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestRemove(t *testing.T) {
	t.Run("leaf", func(t *testing.T) {
		tr, st := build(t, sample)
		if _, err := tr.Remove("a1", false); err != nil {
			t.Fatal(err)
		}
		r := reloaded(t, st)
		if r.Get("a1") != nil || outline(r) != "a\n a2\n  a2x\nb\nc\n c1\n" {
			t.Errorf("got:\n%s", outline(r))
		}
	})
	t.Run("root", func(t *testing.T) {
		tr, st := build(t, sample)
		if _, err := tr.Remove("b", false); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(reloaded(t, st).Roots(), []string{"a", "c"}) {
			t.Error("root not removed")
		}
	})
	t.Run("refuses children", func(t *testing.T) {
		tr, _ := build(t, sample)
		if _, err := tr.Remove("a", false); !errors.Is(err, ErrHasChildren) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("refuses blocker", func(t *testing.T) {
		tr, st := build(t, "a\nb")
		_ = tr.AddDep("a", "b")
		if _, err := tr.Remove("b", false); !errors.Is(err, ErrIsBlocker) {
			t.Errorf("err = %v", err)
		}
		if reloaded(t, st).Get("b") == nil {
			t.Error("b deleted")
		}
	})
	t.Run("force subtree detaches deps", func(t *testing.T) {
		tr, st := build(t, "a\n  a1\n  a2\nb\nc")
		_ = tr.AddDep("b", "a1")
		_ = tr.AddDep("b", "c")
		got, err := tr.Remove("a", true)
		if err != nil || !reflect.DeepEqual(got, []string{"a", "a1", "a2"}) {
			t.Fatalf("got %v, %v", got, err)
		}
		r := reloaded(t, st)
		if r.Get("a1") != nil || r.Get("a2") != nil {
			t.Error("subtree not deleted")
		}
		if !reflect.DeepEqual(r.Get("b").BlockedBy, []string{"c"}) {
			t.Errorf("blocked-by = %v", r.Get("b").BlockedBy)
		}
		if p := r.Fsck(); len(p) != 0 {
			t.Errorf("fsck: %v", p)
		}
	})
	t.Run("blocker inside own subtree ok", func(t *testing.T) {
		tr, _ := build(t, "a\nb")
		_ = tr.AddDep("a", "b")
		if _, err := tr.Remove("b", true); err != nil {
			t.Fatal(err)
		}
		if len(tr.Get("a").BlockedBy) != 0 {
			t.Error("dep not detached")
		}
	})
}

func TestDeps(t *testing.T) {
	const o = "p\n  c1\n  c2\nq\nr"
	tests := []struct {
		name  string
		setup [][2]string
		id    string
		dep   string
		err   error
	}{
		{"ok", nil, "q", "r", nil},
		{"self", nil, "q", "q", ErrSelfDep},
		{"ancestor", nil, "c1", "p", ErrRelatedDep},
		{"descendant", nil, "p", "c2", ErrRelatedDep},
		{"direct cycle", [][2]string{{"q", "r"}}, "r", "q", ErrDepCycle},
		{"indirect cycle", [][2]string{{"q", "r"}, {"r", "c1"}}, "c1", "q", ErrDepCycle},
		{"cycle via parent link", [][2]string{{"q", "c1"}}, "c2", "q", nil},
		{"cycle via parent link 2", [][2]string{{"c1", "q"}}, "q", "p", ErrDepCycle},
		{"cycle via inheritance", [][2]string{{"p", "q"}}, "q", "c1", ErrDepCycle},
		{"duplicate idempotent", [][2]string{{"q", "r"}}, "q", "r", nil},
		{"missing blocker", nil, "q", "zz", store.ErrNotFound},
		{"missing ticket", nil, "zz", "q", store.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, st := build(t, o)
			for _, d := range tt.setup {
				if err := tr.AddDep(d[0], d[1]); err != nil {
					t.Fatalf("setup: %v", err)
				}
			}
			if err := tr.AddDep(tt.id, tt.dep); !errors.Is(err, tt.err) {
				t.Fatalf("err = %v, want %v", err, tt.err)
			}
			r := reloaded(t, st)
			if tt.err == nil {
				if n := count(r.Get(tt.id).BlockedBy, tt.dep); n != 1 {
					t.Errorf("%s listed %d times", tt.dep, n)
				}
			} else if tr.Get(tt.id) != nil && count(r.Get(tt.id).BlockedBy, tt.dep) != 0 {
				t.Error("rejected dep persisted")
			}
			if p := r.Fsck(); len(p) != 0 {
				t.Errorf("fsck after dep: %v", p)
			}
		})
	}
}

func count(l []string, s string) int {
	n := 0
	for _, x := range l {
		if x == s {
			n++
		}
	}
	return n
}

func TestRemoveDep(t *testing.T) {
	tr, st := build(t, "a\nb")
	_ = tr.AddDep("a", "b")
	for i := 0; i < 2; i++ {
		if err := tr.RemoveDep("a", "b"); err != nil {
			t.Fatal(err)
		}
	}
	if len(reloaded(t, st).Get("a").BlockedBy) != 0 {
		t.Error("not removed")
	}
	if err := tr.RemoveDep("zz", "b"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestFsck(t *testing.T) {
	mk := func(t *testing.T, tickets map[string]*store.Ticket, roots []string) *Tree {
		st := store.New(t.TempDir())
		if err := st.Init(); err != nil {
			t.Fatal(err)
		}
		for id, tk := range tickets {
			tk.ID, tk.Created, tk.Title = id, time.Now().UTC(), id
			if tk.Status == "" {
				tk.Status = store.StatusOpen
			}
			if err := st.Save(tk); err != nil {
				t.Fatal(err)
			}
		}
		if err := st.SaveRoots(roots); err != nil {
			t.Fatal(err)
		}
		return reloaded(t, st)
	}
	type tk = store.Ticket
	tests := []struct {
		name    string
		tickets map[string]*tk
		roots   []string
		kinds   []string
	}{
		{"clean", map[string]*tk{"a": {Children: []string{"b"}}, "b": {}}, []string{"a"}, nil},
		{"orphan", map[string]*tk{"a": {}, "b": {}}, []string{"a"}, []string{KindOrphan}},
		{"two parents", map[string]*tk{"a": {Children: []string{"c"}}, "b": {Children: []string{"c"}}, "c": {}}, []string{"a", "b"}, []string{KindDuplicate}},
		{"listed twice", map[string]*tk{"a": {Children: []string{"b", "b"}}, "b": {}}, []string{"a"}, []string{KindDuplicate}},
		{"root and child", map[string]*tk{"a": {Children: []string{"b"}}, "b": {}}, []string{"a", "b"}, []string{KindDuplicate}},
		{"dangling root", map[string]*tk{"a": {}}, []string{"a", "gone"}, []string{KindDangling}},
		{"dangling child", map[string]*tk{"a": {Children: []string{"gone"}}}, []string{"a"}, []string{KindDangling}},
		{"dangling dep", map[string]*tk{"a": {BlockedBy: []string{"gone"}}}, []string{"a"}, []string{KindDangling}},
		{"structural cycle", map[string]*tk{"r": {}, "a": {Children: []string{"b"}}, "b": {Children: []string{"a"}}}, []string{"r"},
			[]string{KindOrphan, KindOrphan, KindCycle}},
		{"dep cycle", map[string]*tk{"a": {BlockedBy: []string{"b"}}, "b": {BlockedBy: []string{"a"}}}, []string{"a", "b"}, []string{KindCycle}},
		{"self dep", map[string]*tk{"a": {BlockedBy: []string{"a"}}}, []string{"a"}, []string{KindDep, KindCycle}},
		{"ancestor dep", map[string]*tk{"a": {Children: []string{"b"}}, "b": {BlockedBy: []string{"a"}}}, []string{"a"}, []string{KindDep, KindCycle}},
		{"descendant dep", map[string]*tk{"a": {Children: []string{"b"}, BlockedBy: []string{"b"}}, "b": {}}, []string{"a"}, []string{KindDep, KindCycle}},
		{"in_progress with open child", map[string]*tk{"a": {Status: store.StatusInProgress, Children: []string{"b"}}, "b": {}}, []string{"a"}, []string{KindStatus}},
		{"in_progress wrap-up parent", map[string]*tk{"a": {Status: store.StatusInProgress, Children: []string{"b"}}, "b": {Status: store.StatusClosed}}, []string{"a"}, nil},
		{"closed with open child", map[string]*tk{"a": {Status: store.StatusClosed, Children: []string{"b"}}, "b": {}}, []string{"a"}, []string{KindStatus}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var kinds []string
			for _, p := range mk(t, tt.tickets, tt.roots).Fsck() {
				kinds = append(kinds, p.Kind)
			}
			if !reflect.DeepEqual(kinds, tt.kinds) {
				t.Errorf("kinds = %v, want %v", kinds, tt.kinds)
			}
		})
	}
	t.Run("unreadable file tolerated", func(t *testing.T) {
		tr := mk(t, map[string]*tk{"a": {}}, []string{"a"})
		if err := writeFile(tr.st.Dir+"/bad.md", "garbage"); err != nil {
			t.Fatal(err)
		}
		tr2 := reloaded(t, tr.st)
		ps := tr2.Fsck()
		if len(ps) != 1 || ps[0].Kind != KindUnreadable {
			t.Errorf("problems = %v", ps)
		}
		if _, err := tr2.Ready(""); err != nil {
			t.Error(err)
		}
	})
	t.Run("corrupt graph does not hang", func(t *testing.T) {
		tr := mk(t, map[string]*tk{"a": {Children: []string{"b"}}, "b": {Children: []string{"a"}}}, []string{"a"})
		_ = tr.Ancestors("a")
		_ = tr.Descendants("a")
		_, _ = tr.Ready("")
		_ = tr.Fsck()
	})
}

func writeFile(path, s string) error { return os.WriteFile(path, []byte(s), 0o644) }

func TestCorruptRootRefusesMutations(t *testing.T) {
	tr, st := build(t, "a\nb\nc")
	if err := writeFile(st.Dir+"/ROOT.md", "garbage"); err != nil {
		t.Fatal(err)
	}
	tr = reloaded(t, st)
	extra := &store.Ticket{ID: "x", Status: store.StatusOpen, Created: time.Now().UTC(), Title: "x"}
	ops := map[string]func() error{
		"add":    func() error { return tr.Add(extra, "", Place{}) },
		"move":   func() error { return tr.Move("a", "b", Place{}) },
		"remove": func() error { _, err := tr.Remove("a", false); return err },
		"start":  func() error { _, err := tr.Start("a"); return err },
		"dep":    func() error { return tr.AddDep("a", "b") },
	}
	for name, op := range ops {
		if err := op(); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: err = %v, want ErrCorrupt", name, err)
		}
	}
	if got, _ := os.ReadFile(st.Dir + "/ROOT.md"); string(got) != "garbage" {
		t.Errorf("ROOT.md modified: %q", got)
	}
	if _, err := os.Stat(st.Dir + "/x.md"); !os.IsNotExist(err) {
		t.Error("new ticket file written")
	}
	if ps := tr.Fsck(); len(ps) == 0 || ps[0].Kind != KindUnreadable || ps[0].ID != "ROOT" {
		t.Errorf("fsck = %v", ps)
	}
}

func TestUnreadableTicketRefusesMutations(t *testing.T) {
	tr, st := build(t, "a\nb")
	if err := writeFile(st.Dir+"/b.md", "garbage"); err != nil {
		t.Fatal(err)
	}
	tr = reloaded(t, st)
	if _, err := tr.Close("a", false); !errors.Is(err, ErrCorrupt) {
		t.Errorf("err = %v", err)
	}
	if got, _ := os.ReadFile(st.Dir + "/b.md"); string(got) != "garbage" {
		t.Error("b.md modified")
	}
}

func TestInProgressAncestorResetByOpenWork(t *testing.T) {
	status := func(tr *Tree, id string) store.Status { return tr.Get(id).Status }
	// The review's sequence: close C; start A; reopen C.
	tr, st := build(t, "a\n  c")
	if _, err := tr.Close("c", false); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.Start("a"); err != nil { // wrap-up parent
		t.Fatal(err)
	}
	if status(tr, "a") != store.StatusInProgress {
		t.Fatal("a should be in_progress")
	}
	changed, err := tr.Reopen("c")
	if err != nil || !reflect.DeepEqual(changed, []string{"c", "a"}) {
		t.Fatalf("changed = %v, err = %v", changed, err)
	}
	if got := status(reloaded(t, st), "a"); got != store.StatusOpen {
		t.Errorf("a = %s, want open", got)
	}
	if ps := tr.Fsck(); len(ps) != 0 {
		t.Errorf("fsck = %v", ps)
	}

	// Start of a descendant under an in_progress wrap-up parent.
	tr, _ = build(t, "a:in_progress\n  b:closed\n  c:closed")
	if _, err := tr.Start("b"); err != nil {
		t.Fatal(err)
	}
	if status(tr, "a") != store.StatusOpen {
		t.Errorf("a = %s after start b", status(tr, "a"))
	}

	// Add under a grandparent that is in_progress.
	tr, _ = build(t, "g\n  p:closed")
	_, _ = tr.Start("g")
	if status(tr, "g") != store.StatusInProgress {
		t.Fatal("g should be in_progress")
	}
	tk := &store.Ticket{ID: "n", Status: store.StatusOpen, Created: time.Now().UTC(), Title: "n"}
	if err := tr.Add(tk, "p", Place{}); err != nil {
		t.Fatal(err)
	}
	if status(tr, "g") != store.StatusOpen || status(tr, "p") != store.StatusOpen {
		t.Errorf("g = %s, p = %s", status(tr, "g"), status(tr, "p"))
	}

	// Move open work under an in_progress wrap-up grandparent.
	tr, _ = build(t, "g\n  p:closed\nx")
	_, _ = tr.Start("g")
	if err := tr.Move("x", "p", Place{}); err != nil {
		t.Fatal(err)
	}
	if status(tr, "g") != store.StatusOpen {
		t.Errorf("g = %s after move", status(tr, "g"))
	}
	if ps := tr.Fsck(); len(ps) != 0 {
		t.Errorf("fsck = %v", ps)
	}
}
