package tui

import (
	"regexp"
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
		case "left":
			msg = tea.KeyMsg{Type: tea.KeyLeft}
		case "right":
			msg = tea.KeyMsg{Type: tea.KeyRight}
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
	m.lock = func(bool) (func(), error) {
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

func key(m *Model, t tea.KeyType) { m.Update(tea.KeyMsg{Type: t}) }

func TestEnterOpensDetailAndEscReturns(t *testing.T) {
	m, _ := newModel(t, "aaa - ", "bbb - ")
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	press(m, "j")
	key(m, tea.KeyEnter)
	if m.detailID != "bbb" {
		t.Fatalf("detailID = %q, want bbb", m.detailID)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "T bbb") || !strings.Contains(v, "esc back") {
		t.Errorf("detail view missing title/help:\n%s", v)
	}
	press(m, "H", "L") // reorder keys are ignored in the detail view
	if order(m) != "aaa bbb" {
		t.Errorf("order changed in detail view: %s", order(m))
	}
	key(m, tea.KeyEsc)
	if m.detailID != "" || m.cursor != 1 {
		t.Errorf("after esc: detailID=%q cursor=%d", m.detailID, m.cursor)
	}
	key(m, tea.KeyEnter)
	press(m, "q")
	if m.detailID != "" {
		t.Error("q should close the detail view")
	}
}

func TestDetailStepsBetweenTickets(t *testing.T) {
	m, _ := newModel(t, "aaa - ", "bbb aaa", "ccc - ")
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	key(m, tea.KeyEnter)
	press(m, "K")
	if m.detailID != "aaa" || m.status != "first ticket" {
		t.Errorf("K at top: detailID=%q status=%q", m.detailID, m.status)
	}
	press(m, "J")
	if m.detailID != "bbb" || m.status != "" || m.scroll != 0 {
		t.Errorf("J: detailID=%q status=%q", m.detailID, m.status)
	}
	press(m, "right")
	if m.detailID != "ccc" {
		t.Errorf("right: detailID=%q", m.detailID)
	}
	press(m, "J")
	if m.detailID != "ccc" || m.status != "last ticket" {
		t.Errorf("J at bottom: detailID=%q status=%q", m.detailID, m.status)
	}
	press(m, "left")
	if m.detailID != "bbb" {
		t.Errorf("left: detailID=%q", m.detailID)
	}
	if order(m) != "aaa -bbb ccc" {
		t.Errorf("order changed: %s", order(m))
	}
	key(m, tea.KeyEsc)
	if m.detailID != "" || m.cursor != 1 {
		t.Errorf("after esc: detailID=%q cursor=%d", m.detailID, m.cursor)
	}
}

func TestDetailStepWhenTicketVanished(t *testing.T) {
	m, st := newModel(t, "aaa - ", "bbb - ")
	m.lock = func(bool) (func(), error) { return func() {}, nil }
	key(m, tea.KeyEnter)
	if err := st.Delete("aaa"); err != nil {
		t.Fatal(err)
	}
	press(m, "J")
	if m.detailID != "" || m.status == "" {
		t.Errorf("vanished ticket: detailID=%q status=%q", m.detailID, m.status)
	}
}

func TestDetailScrollAndQuit(t *testing.T) {
	m, _ := newModel(t, "aaa - ")
	m.t.Get("aaa").Body = strings.Repeat("line\n\n", 40)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	key(m, tea.KeyEnter)
	press(m, "j")
	if m.scroll != 1 {
		t.Errorf("scroll = %d, want 1", m.scroll)
	}
	press(m, "G")
	if m.scroll != len(m.detail)-m.pageSize() {
		t.Errorf("G scroll = %d", m.scroll)
	}
	press(m, "g")
	if m.scroll != 0 {
		t.Errorf("g scroll = %d", m.scroll)
	}
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC}); cmd == nil {
		t.Error("ctrl+c should quit from the detail view")
	}
}

func TestEnterOnEmptyListAndVanishedTicket(t *testing.T) {
	m, _ := newModel(t)
	key(m, tea.KeyEnter)
	if m.detailID != "" {
		t.Error("enter on empty list opened a detail view")
	}
	m, st := newModel(t, "aaa - ")
	m.lock = func(bool) (func(), error) { return func() {}, nil }
	if err := st.Delete("aaa"); err != nil {
		t.Fatal(err)
	}
	key(m, tea.KeyEnter)
	if m.detailID != "" || m.status == "" {
		t.Errorf("vanished ticket: detailID=%q status=%q", m.detailID, m.status)
	}
}

var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansi.ReplaceAllString(s, "") }
