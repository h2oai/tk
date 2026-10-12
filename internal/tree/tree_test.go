package tree

import (
	"errors"
	"os"
	"path/filepath"
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
		tk := &store.Ticket{ID: id, Status: store.Status(status), Type: store.TypeTask, Effort: store.EffortNormal, Created: time.Now().UTC(), Title: "T " + id}
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
		return &store.Ticket{ID: id, Status: store.StatusOpen, Type: store.TypeTask, Effort: store.EffortNormal, Created: time.Now().UTC(), Title: id}
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
			if tk.Type == "" {
				tk.Type = store.TypeTask
			}
			if tk.Effort == "" {
				tk.Effort = store.EffortNormal
			}
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
	extra := &store.Ticket{ID: "x", Status: store.StatusOpen, Type: store.TypeTask, Effort: store.EffortNormal, Created: time.Now().UTC(), Title: "x"}
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
	tk := &store.Ticket{ID: "n", Status: store.StatusOpen, Type: store.TypeTask, Effort: store.EffortNormal, Created: time.Now().UTC(), Title: "n"}
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

func TestSetType(t *testing.T) {
	tr, st := build(t, "a:closed\n  a1:closed")
	ok, err := tr.SetType("a1", store.TypeBug)
	if err != nil || !ok {
		t.Fatalf("SetType = %v, %v", ok, err)
	}
	r := reloaded(t, st)
	if r.Get("a1").Type != store.TypeBug {
		t.Errorf("type = %q", r.Get("a1").Type)
	}
	if r.Get("a").Status != store.StatusClosed || r.Get("a1").Status != store.StatusClosed {
		t.Error("set-type changed status")
	}
	if ok, err := tr.SetType("a1", store.TypeBug); err != nil || ok {
		t.Errorf("same type = %v, %v; want silent no-op", ok, err)
	}
	if _, err := tr.SetType("a1", "epic"); err == nil {
		t.Error("expected error for invalid type")
	}
	if _, err := tr.SetType("nope", store.TypeChore); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestSetEffort(t *testing.T) {
	tr, st := build(t, "a:closed\n  a1:closed")
	ok, err := tr.SetEffort("a1", store.EffortHard)
	if err != nil || !ok {
		t.Fatalf("SetEffort = %v, %v", ok, err)
	}
	r := reloaded(t, st)
	if r.Get("a1").Effort != store.EffortHard {
		t.Errorf("effort = %q", r.Get("a1").Effort)
	}
	if r.Get("a").Status != store.StatusClosed || r.Get("a1").Status != store.StatusClosed {
		t.Error("set-effort changed status")
	}
	if ok, err := tr.SetEffort("a1", store.EffortHard); err != nil || ok {
		t.Errorf("same effort = %v, %v; want silent no-op", ok, err)
	}
	if _, err := tr.SetEffort("a1", "medium"); err == nil {
		t.Error("expected error for invalid effort")
	}
	if _, err := tr.SetEffort("nope", store.EffortEasy); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestIndentOutdent(t *testing.T) {
	tests := []struct {
		name string
		op   func(*Tree, string) error
		id   string
		want string
		err  error
	}{
		{"indent under previous", (*Tree).Indent, "a2", "a\n a1\n  a2\n   a2x\nb\nc\n c1\n", nil},
		{"indent root appends", (*Tree).Indent, "b", "a\n a1\n a2\n  a2x\n b\nc\n c1\n", nil},
		{"indent first is no-op", (*Tree).Indent, "a1", "a\n a1\n a2\n  a2x\nb\nc\n c1\n", nil},
		{"indent first root is no-op", (*Tree).Indent, "a", "a\n a1\n a2\n  a2x\nb\nc\n c1\n", nil},
		{"outdent to after parent", (*Tree).Outdent, "a1", "a\n a2\n  a2x\na1\nb\nc\n c1\n", nil},
		{"outdent nested", (*Tree).Outdent, "a2x", "a\n a1\n a2\n a2x\nb\nc\n c1\n", nil},
		{"outdent root is no-op", (*Tree).Outdent, "b", "a\n a1\n a2\n  a2x\nb\nc\n c1\n", nil},
		{"indent missing", (*Tree).Indent, "zz", "", store.ErrNotFound},
		{"outdent missing", (*Tree).Outdent, "zz", "", store.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, st := build(t, sample)
			if err := tt.op(tr, tt.id); !errors.Is(err, tt.err) {
				t.Fatalf("err = %v, want %v", err, tt.err)
			}
			if tt.err != nil {
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

func TestIndentRechecksDeps(t *testing.T) {
	tr, st := build(t, "a\nb\n")
	if err := tr.AddDep("b", "a"); err != nil {
		t.Fatal(err)
	}
	// indenting b under a would make b depend on its own ancestor
	if err := tr.Indent("b"); err == nil {
		t.Fatal("expected dep rule error")
	}
	if got := outline(reloaded(t, st)); got != "a\nb\n" {
		t.Errorf("failed indent changed disk:\n%s", got)
	}
}

func ids(tks []*store.Ticket) []string {
	var out []string
	for _, tk := range tks {
		out = append(out, tk.ID)
	}
	return out
}

// archivedIDs lists the archived tickets on disk, sorted.
func archivedIDs(t *testing.T, st *store.Store) []string {
	t.Helper()
	r := reloaded(t, st)
	var out []string
	for _, id := range r.sortedIDs() {
		if r.Get(id).Archived {
			out = append(out, id)
		}
	}
	return out
}

func TestArchive(t *testing.T) {
	t.Run("closed subtree", func(t *testing.T) {
		tr, st := build(t, "a\n  a1:closed\n  a2:closed\n    a2y:closed\n    a2x:closed\nb")
		got, err := tr.Archive("a2", false, false)
		if err != nil || !reflect.DeepEqual(ids(got), []string{"a2", "a2y", "a2x"}) {
			t.Fatalf("got %v, %v", ids(got), err)
		}
		if got := archivedIDs(t, st); !reflect.DeepEqual(got, []string{"a2", "a2x", "a2y"}) {
			t.Errorf("archived = %v", got)
		}
		for _, r := range []*Tree{tr, reloaded(t, st)} {
			if outline(r) != "a\n a1\n a2\n  a2y\n  a2x\nb\n" {
				t.Errorf("tree changed:\n%s", outline(r))
			}
			if id, err := r.Resolve("a2x"); err != nil || id != "a2x" {
				t.Errorf("resolve archived: %v, %v", id, err)
			}
			if r.Position("a2") != "-" || r.Position("a2x") != "-" || r.Position("b") != "2" {
				t.Errorf("positions: %q %q %q", r.Position("a2"), r.Position("a2x"), r.Position("b"))
			}
			if p := r.Fsck(); len(p) != 0 {
				t.Errorf("fsck: %v", p)
			}
		}
		// Archiving again changes nothing.
		if got, err := tr.Archive("a2", false, false); err != nil || len(got) != 0 {
			t.Errorf("again: %v, %v", ids(got), err)
		}
	})
	t.Run("refuses unclosed", func(t *testing.T) {
		tr, st := build(t, "a:closed\n  a1\nb")
		if _, err := tr.Archive("a", false, false); !errors.Is(err, ErrNotClosed) {
			t.Errorf("err = %v", err)
		}
		if archivedIDs(t, st) != nil {
			t.Error("archive changed something")
		}
	})
	t.Run("force closes first", func(t *testing.T) {
		tr, st := build(t, "a\n  a1:in_progress\n  a2:closed\nb")
		if _, err := tr.Archive("a", true, false); err != nil {
			t.Fatal(err)
		}
		r := reloaded(t, st)
		for _, id := range []string{"a", "a1", "a2"} {
			if tk := r.Get(id); tk.Status != store.StatusClosed || !tk.Archived {
				t.Errorf("%s = %+v", id, tk)
			}
		}
	})
	t.Run("deps to live tickets are fine", func(t *testing.T) {
		tr, st := build(t, "a:closed\n  a1:closed\nb\nc:closed")
		if err := tr.AddDep("b", "a1"); err != nil {
			t.Fatal(err)
		}
		if err := tr.AddDep("a1", "c"); err != nil {
			t.Fatal(err)
		}
		if _, err := tr.Archive("a", false, false); err != nil {
			t.Fatal(err)
		}
		r := reloaded(t, st)
		if res, _ := r.Ready(""); res.Outcome != Found || res.Ticket.ID != "b" {
			t.Errorf("ready = %+v", res)
		}
		if p := r.Fsck(); len(p) != 0 {
			t.Errorf("fsck: %v", p)
		}
	})
	t.Run("refuses corrupt tree", func(t *testing.T) {
		tr, st := build(t, "a:closed\nb")
		os.WriteFile(filepath.Join(st.Dir, "c.md"), []byte("garbage"), 0o644)
		_ = tr.Reload()
		if _, err := tr.Archive("a", false, false); !errors.Is(err, ErrCorrupt) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestArchiveClosed(t *testing.T) {
	t.Run("every closed subtree", func(t *testing.T) {
		tr, st := build(t, "a\n  a1:closed\n  a2:closed\n    a2x:closed\n  a3\nb:closed\n  b1:closed\nc")
		if err := tr.AddDep("c", "a1"); err != nil { // crossing deps no longer matter
			t.Fatal(err)
		}
		got, err := tr.ArchiveClosed(false)
		if err != nil || !reflect.DeepEqual(ids(got), []string{"a1", "a2", "a2x", "b", "b1"}) {
			t.Fatalf("got %v, %v", ids(got), err)
		}
		r := reloaded(t, st)
		if r.Get("a").Status != store.StatusOpen || r.Get("a").Archived || len(r.Fsck()) != 0 {
			t.Errorf("a = %+v, fsck %v", r.Get("a"), r.Fsck())
		}
		if r.Position("a3") != "1.1" || r.Position("c") != "2" {
			t.Errorf("positions: %q %q", r.Position("a3"), r.Position("c"))
		}
	})
	t.Run("closed epic over archived children", func(t *testing.T) {
		tr, _ := build(t, "a:closed\n  a1:closed")
		if _, err := tr.Archive("a1", false, false); err != nil {
			t.Fatal(err)
		}
		if got, err := tr.ArchiveClosed(false); err != nil || !reflect.DeepEqual(ids(got), []string{"a"}) {
			t.Errorf("got %v, %v", ids(got), err)
		}
	})
	t.Run("dry run", func(t *testing.T) {
		tr, st := build(t, "a:closed\nb\n  b1")
		got, err := tr.ArchiveClosed(true)
		if err != nil || !reflect.DeepEqual(ids(got), []string{"a"}) {
			t.Fatalf("got %v, %v", ids(got), err)
		}
		got, err = tr.Archive("b", true, true)
		if err != nil || !reflect.DeepEqual(ids(got), []string{"b", "b1"}) {
			t.Fatalf("got %v, %v", ids(got), err)
		}
		if archivedIDs(t, st) != nil || reloaded(t, st).Get("b1").Status != store.StatusOpen {
			t.Error("dry run changed disk")
		}
	})
	t.Run("refuses corrupt tree", func(t *testing.T) {
		tr, st := build(t, "a:closed")
		os.WriteFile(filepath.Join(st.Dir, "c.md"), []byte("garbage"), 0o644)
		_ = tr.Reload()
		for _, dry := range []bool{false, true} {
			if _, err := tr.ArchiveClosed(dry); !errors.Is(err, ErrCorrupt) {
				t.Errorf("dry=%v: err = %v", dry, err)
			}
		}
	})
}

func TestUnarchive(t *testing.T) {
	tr, st := build(t, "a:closed\n  a1:closed\n    a1x:closed\n  a2:closed\nb")
	if _, err := tr.Archive("a", false, false); err != nil {
		t.Fatal(err)
	}
	got, err := tr.Unarchive("a1", true)
	if err != nil || !reflect.DeepEqual(ids(got), []string{"a", "a1", "a1x"}) {
		t.Fatalf("dry: %v, %v", ids(got), err)
	}
	if len(archivedIDs(t, st)) != 4 {
		t.Error("dry run changed disk")
	}
	if got, err = tr.Unarchive("a1", false); err != nil || !reflect.DeepEqual(ids(got), []string{"a", "a1", "a1x"}) {
		t.Fatalf("got %v, %v", ids(got), err)
	}
	r := reloaded(t, st)
	if got := archivedIDs(t, st); !reflect.DeepEqual(got, []string{"a2"}) {
		t.Errorf("archived = %v", got)
	}
	if r.Get("a1").Status != store.StatusClosed || len(r.Fsck()) != 0 {
		t.Errorf("a1 = %+v, fsck %v", r.Get("a1"), r.Fsck())
	}
	if got, err = tr.Unarchive("b", false); err != nil || len(got) != 0 {
		t.Errorf("live: %v, %v", ids(got), err)
	}
}

func TestArchivedRules(t *testing.T) {
	setup := func(t *testing.T) (*Tree, *store.Store) {
		t.Helper()
		tr, st := build(t, "a\n  x:closed\n  a1\n  y:closed\n  a2\n  a3\nz:closed\n  z1:closed\nb")
		for _, id := range []string{"x", "y", "z"} {
			if _, err := tr.Archive(id, false, false); err != nil {
				t.Fatal(err)
			}
		}
		return tr, st
	}
	t.Run("status changes refused", func(t *testing.T) {
		tr, _ := setup(t)
		if _, err := tr.Start("x"); !errors.Is(err, ErrArchived) {
			t.Errorf("start: %v", err)
		}
		if _, err := tr.Reopen("z1"); !errors.Is(err, ErrArchived) {
			t.Errorf("reopen: %v", err)
		}
		if _, err := tr.Close("x", false); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	t.Run("no live work under archived", func(t *testing.T) {
		tr, st := setup(t)
		tk := &store.Ticket{ID: "n", Status: store.StatusOpen, Type: store.TypeTask, Effort: store.EffortNormal, Created: time.Now().UTC(), Title: "N"}
		if err := tr.Add(tk, "z", Place{}); !errors.Is(err, ErrArchived) {
			t.Errorf("add: %v", err)
		}
		if err := tr.Move("b", "z", Place{}); !errors.Is(err, ErrArchived) {
			t.Errorf("move live: %v", err)
		}
		if err := tr.Indent("a2"); err != nil || tr.Parent("a2") != "a1" {
			t.Errorf("indent skips archived sibling: %v, parent %q", err, tr.Parent("a2"))
		}
		// An archived ticket may move under an archived or live parent.
		if err := tr.Move("x", "z", Place{}); err != nil {
			t.Errorf("move archived: %v", err)
		}
		if err := tr.Move("x", "b", Place{}); err != nil {
			t.Errorf("move archived under live: %v", err)
		}
		if p := reloaded(t, st).Fsck(); len(p) != 0 {
			t.Errorf("fsck: %v", p)
		}
	})
	t.Run("positions skip archived", func(t *testing.T) {
		tr, _ := setup(t)
		want := map[string]string{"a": "1", "x": "-", "a1": "1.1", "y": "-", "a2": "1.2", "a3": "1.3", "z": "-", "z1": "-", "b": "2"}
		for id, p := range want {
			if got := tr.Position(id); got != p {
				t.Errorf("%s at %q, want %q", id, got, p)
			}
		}
	})
	t.Run("reorder counts shown siblings", func(t *testing.T) {
		tr, _ := setup(t)
		if err := tr.Up("a2"); err != nil || !reflect.DeepEqual(tr.Get("a").Children, []string{"x", "a2", "a1", "y", "a3"}) {
			t.Errorf("up: %v, %v", err, tr.Get("a").Children)
		}
		if err := tr.Down("a2"); err != nil || !reflect.DeepEqual(tr.Get("a").Children, []string{"x", "a1", "y", "a2", "a3"}) {
			t.Errorf("down: %v, %v", err, tr.Get("a").Children)
		}
		if err := tr.Top("a3"); err != nil || !reflect.DeepEqual(tr.Get("a").Children, []string{"x", "a3", "a1", "y", "a2"}) {
			t.Errorf("top: %v, %v", err, tr.Get("a").Children)
		}
		if err := tr.Move("a3", "a", Place{Index: 2}); err != nil || !reflect.DeepEqual(tr.Get("a").Children, []string{"x", "a1", "y", "a3", "a2"}) {
			t.Errorf("at 2: %v, %v", err, tr.Get("a").Children)
		}
		if err := tr.Move("a3", "a", Place{Index: 9}); err != nil || tr.Position("a3") != "1.3" {
			t.Errorf("at 9: %v, %v", err, tr.Get("a").Children)
		}
		if err := tr.Move("b", "a", Place{Before: "y"}); err != nil || tr.Position("b") != "1.2" {
			t.Errorf("before archived: %v, %v", err, tr.Get("a").Children)
		}
	})
	t.Run("archived has no position to shift", func(t *testing.T) {
		tr, _ := setup(t)
		for name, op := range map[string]func(string) error{"up": tr.Up, "down": tr.Down, "top": tr.Top, "bottom": tr.Bottom} {
			if err := op("y"); !errors.Is(err, ErrArchived) {
				t.Errorf("%s: %v", name, err)
			}
		}
		if err := tr.Move("y", "a", Place{Index: 1}); !errors.Is(err, ErrArchived) {
			t.Errorf("move --at: %v", err)
		}
		if err := tr.Move("y", "", Place{After: "b"}); err != nil {
			t.Errorf("move --after: %v", err)
		}
	})
	t.Run("edit cannot flip archived", func(t *testing.T) {
		tr, _ := setup(t)
		tk := *tr.Get("x")
		tk.Archived = false
		if err := tr.Replace(&tk); err == nil || !strings.Contains(err.Error(), "archived cannot be edited") {
			t.Errorf("replace: %v", err)
		}
		tk = *tr.Get("x")
		tk.Title = "New"
		if err := tr.Replace(&tk); err != nil {
			t.Errorf("replace title: %v", err)
		}
	})
	t.Run("fsck", func(t *testing.T) {
		_, st := setup(t)
		for _, edit := range []struct{ id, from, to string }{
			{"z", "status: closed", "status: open"},
			{"z1", "archived: true\n", ""},
		} {
			path := filepath.Join(st.Dir, edit.id+".md")
			data, _ := os.ReadFile(path)
			os.WriteFile(path, []byte(strings.Replace(string(data), edit.from, edit.to, 1)), 0o644)
		}
		var got []string
		for _, p := range reloaded(t, st).Fsck() {
			if p.Kind == KindArchived {
				got = append(got, p.String())
			}
		}
		want := []string{
			"archived: z: archived but open (close it, or tk unarchive z)",
			"archived: z: archived but descendant z1 is not (tk archive z or tk unarchive z1)",
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("fsck = %q", got)
		}
	})
}
