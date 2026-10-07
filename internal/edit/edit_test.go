package edit

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/h2oai/tk/internal/store"
	"github.com/h2oai/tk/internal/tree"
)

func setup(t *testing.T) (*tree.Tree, *store.Store) {
	t.Helper()
	st := store.New(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	tr, err := tree.Load(st)
	if err != nil {
		t.Fatal(err)
	}
	tk := &store.Ticket{ID: "aaa", Status: store.StatusOpen, Type: store.TypeTask, Created: time.Now().UTC().Truncate(time.Second), Title: "Alpha"}
	if err := tr.Add(tk, "", tree.Place{}); err != nil {
		t.Fatal(err)
	}
	return tr, st
}

// write starts a session and replaces the temp file with change(contents).
func write(t *testing.T, tr *tree.Tree, change func(string) string) *Session {
	t.Helper()
	s, err := Start(tr, "aaa", nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Path, []byte(change(string(data))), 0o644); err != nil {
		t.Fatal(err)
	}
	return s
}

func gone(t *testing.T, s *Session) {
	t.Helper()
	if _, err := os.Stat(s.Path); !os.IsNotExist(err) {
		t.Errorf("temp file left behind: %v", err)
	}
}

func TestFinishSaves(t *testing.T) {
	tr, st := setup(t)
	locks := []bool{}
	lock := func(ex bool) (func(), error) { locks = append(locks, ex); return func() {}, nil }
	s := write(t, tr, func(s string) string { return strings.Replace(s, "# Alpha", "# Beta", 1) })
	changed, err := s.Finish(lock)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if tk, _ := st.Load("aaa"); tk.Title != "Beta" {
		t.Errorf("title %q", tk.Title)
	}
	if len(locks) != 1 || !locks[0] {
		t.Errorf("locks %v, want one exclusive", locks)
	}
	gone(t, s)
}

func TestFinishUnchangedWritesNothing(t *testing.T) {
	tr, st := setup(t)
	s := write(t, tr, func(s string) string { return s })
	before, _ := os.Stat(st.Dir + "/aaa.md")
	changed, err := s.Finish(func(bool) (func(), error) {
		t.Error("unchanged edit took the lock")
		return func() {}, nil
	})
	if err != nil || changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if after, _ := os.Stat(st.Dir + "/aaa.md"); !after.ModTime().Equal(before.ModTime()) {
		t.Error("file rewritten")
	}
	gone(t, s)
}

func TestFinishRejects(t *testing.T) {
	for name, change := range map[string]func(string) string{
		"garbage":    func(string) string { return "garbage" },
		"id changed": func(s string) string { return strings.Replace(s, "id: aaa", "id: bbb", 1) },
		"bad status": func(s string) string { return strings.Replace(s, "status: open", "status: nope", 1) },
	} {
		t.Run(name, func(t *testing.T) {
			tr, st := setup(t)
			s := write(t, tr, change)
			if _, err := s.Finish(nil); err == nil || !strings.Contains(err.Error(), "invalid, nothing saved") {
				t.Errorf("err %v", err)
			}
			if tk, _ := st.Load("aaa"); tk.Title != "Alpha" {
				t.Errorf("title %q", tk.Title)
			}
			gone(t, s)
		})
	}
}

func TestFinishRefusesConcurrentChange(t *testing.T) {
	tr, st := setup(t)
	s := write(t, tr, func(s string) string { return strings.Replace(s, "# Alpha", "# Beta", 1) })
	other, err := tree.Load(st)
	if err != nil {
		t.Fatal(err)
	}
	if err := other.AppendNote("aaa", "meanwhile", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Finish(nil); err == nil || !strings.Contains(err.Error(), "changed while editing") {
		t.Errorf("err %v", err)
	}
	if tk, _ := st.Load("aaa"); tk.Title != "Alpha" || !strings.Contains(tk.Body, "meanwhile") {
		t.Errorf("other change lost: %q %q", tk.Title, tk.Body)
	}
}

func TestStartMissingTicket(t *testing.T) {
	tr, st := setup(t)
	if err := st.Delete("aaa"); err != nil {
		t.Fatal(err)
	}
	if _, err := Start(tr, "aaa", nil); err == nil || !strings.Contains(err.Error(), "no longer exists") {
		t.Errorf("err %v", err)
	}
}
