package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/h2oai/tk/internal/store"
	"github.com/h2oai/tk/internal/tree"
)

// newModel builds a tree on disk from "id parent" lines (parent "-" for a
// root) and returns a model over it plus the store for reloading.
func newModel(t *testing.T, lines ...string) (*Model, *store.Store) {
	t.Helper()
	st := store.New(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	tr, err := tree.Load(st)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lines {
		f := strings.Fields(l)
		parent := f[1]
		if parent == "-" {
			parent = ""
		}
		tk := &store.Ticket{ID: f[0], Status: store.StatusOpen, Type: store.TypeTask, Created: time.Now().UTC(), Title: "T " + f[0]}
		if err := tr.Add(tk, parent, tree.Place{}); err != nil {
			t.Fatal(err)
		}
	}
	return New(tr, nil), st
}

func press(m *Model, keys ...string) {
	for _, k := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		switch k {
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		}
		m.Update(msg)
	}
}

// order lists ids in outline order as "id" prefixed by one dash per depth.
func order(m *Model) string {
	var parts []string
	for _, r := range m.rows {
		parts = append(parts, strings.Repeat("-", r.depth)+r.id)
	}
	return strings.Join(parts, " ")
}

func reload(t *testing.T, st *store.Store) *Model {
	t.Helper()
	tr, err := tree.Load(st)
	if err != nil {
		t.Fatal(err)
	}
	return New(tr, nil)
}

func TestCursorMovement(t *testing.T) {
	m, _ := newModel(t, "a -", "b -", "c b")
	if m.selected() != "a" {
		t.Fatalf("start on %s", m.selected())
	}
	press(m, "j", "j", "j", "j")
	if m.selected() != "c" {
		t.Errorf("clamped at %s", m.selected())
	}
	press(m, "k")
	if m.selected() != "b" {
		t.Errorf("k: %s", m.selected())
	}
	press(m, "up", "up", "up")
	if m.selected() != "a" {
		t.Errorf("up: %s", m.selected())
	}
	press(m, "down")
	if m.selected() != "b" {
		t.Errorf("down: %s", m.selected())
	}
}

func TestShiftKeys(t *testing.T) {
	m, st := newModel(t, "a -", "b -", "c -")
	press(m, "j", "j", "K") // cursor on c, move up
	if got := order(m); got != "a c b" || m.selected() != "c" {
		t.Errorf("K: %s on %s", got, m.selected())
	}
	press(m, "K", "K") // edge is a benign no-op
	if got := order(m); got != "c a b" || m.status != "" {
		t.Errorf("K at top: %s status %q", got, m.status)
	}
	press(m, "J")
	if got := order(m); got != "a c b" || m.selected() != "c" {
		t.Errorf("J: %s", got)
	}
	if got := order(reload(t, st)); got != "a c b" {
		t.Errorf("disk: %s", got)
	}
}

func TestIndentOutdentKeys(t *testing.T) {
	m, st := newModel(t, "a -", "b -")
	press(m, "j", "L")
	if got := order(m); got != "a -b" || m.selected() != "b" {
		t.Errorf("L: %s on %s", got, m.selected())
	}
	if got := order(reload(t, st)); got != "a -b" {
		t.Errorf("disk: %s", got)
	}
	press(m, "H")
	if got := order(m); got != "a b" || m.selected() != "b" {
		t.Errorf("H: %s", got)
	}
	press(m, "H") // root outdent is a no-op
	if got := order(m); got != "a b" || m.status != "" {
		t.Errorf("H at root: %s status %q", got, m.status)
	}
}

func TestTopBottomKeys(t *testing.T) {
	m, _ := newModel(t, "a -", "b -", "c -")
	press(m, "G")
	if got := order(m); got != "b c a" || m.selected() != "a" {
		t.Errorf("G: %s on %s", got, m.selected())
	}
	press(m, "g")
	if got := order(m); got != "a b c" || m.selected() != "a" {
		t.Errorf("g: %s", got)
	}
}

func TestErrorInStatusLine(t *testing.T) {
	st := store.New(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	tr, err := tree.Load(st)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b"} {
		tk := &store.Ticket{ID: id, Status: store.StatusOpen, Type: store.TypeTask, Created: time.Now().UTC(), Title: id}
		if err := tr.Add(tk, "", tree.Place{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tr.AddDep("b", "a"); err != nil {
		t.Fatal(err)
	}
	m := New(tr, nil)
	press(m, "j", "L") // b under its blocker a breaks the dependency rule
	if m.status == "" || !strings.Contains(m.View(), m.status) {
		t.Errorf("status %q not shown", m.status)
	}
	if got := order(m); got != "a b" {
		t.Errorf("failed move changed view: %s", got)
	}
	press(m, "j") // a successful key clears nothing, but a mutation does
	press(m, "K")
	if m.status != "" {
		t.Errorf("status not cleared: %q", m.status)
	}
}

func TestQuit(t *testing.T) {
	m, _ := newModel(t, "a -")
	for _, k := range []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune("q")}, {Type: tea.KeyCtrlC}} {
		if _, cmd := m.Update(k); cmd == nil {
			t.Errorf("%v did not quit", k)
		}
	}
}

func TestReorderReloadsAndLocks(t *testing.T) {
	m, st := newModel(t, "a -", "b -", "c -")
	locks, unlocks := 0, 0
	m.lock = func() (func(), error) {
		locks++
		return func() { unlocks++ }, nil
	}
	// Another process appends d and removes nothing; the model has not seen it.
	other, err := tree.Load(st)
	if err != nil {
		t.Fatal(err)
	}
	d := &store.Ticket{ID: "d", Status: store.StatusOpen, Type: store.TypeTask, Created: time.Now().UTC(), Title: "T d"}
	if err := other.Add(d, "", tree.Place{}); err != nil {
		t.Fatal(err)
	}
	press(m, "J") // a down
	if got := order(m); got != "b a c d" {
		t.Errorf("after stale reorder: %s", got)
	}
	if got := order(reload(t, st)); got != "b a c d" {
		t.Errorf("disk lost the other process's ticket: %s", got)
	}
	if locks != 1 || unlocks != 1 {
		t.Errorf("locks %d unlocks %d", locks, unlocks)
	}
	// The selected ticket is deleted elsewhere: report it, change nothing.
	other, _ = tree.Load(st)
	if _, err := other.Remove("a", false); err != nil {
		t.Fatal(err)
	}
	press(m, "J")
	if !strings.Contains(m.status, "no longer exists") {
		t.Errorf("status %q", m.status)
	}
}
