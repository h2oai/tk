package tui

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// fakeEditor makes the model's editor apply change to the file it is given,
// in place of running $EDITOR, and returns the count of runs.
func fakeEditor(t *testing.T, m *Model, change func(string) string) *int {
	t.Helper()
	t.Setenv("EDITOR", "unused")
	runs := 0
	m.exec = func(c *exec.Cmd, done tea.ExecCallback) tea.Cmd {
		runs++
		path := c.Args[len(c.Args)-1]
		data, err := os.ReadFile(path)
		if err == nil {
			err = os.WriteFile(path, []byte(change(string(data))), 0o644)
		}
		return func() tea.Msg { return done(err) }
	}
	return &runs
}

// pressEdit presses e and delivers the editor's result, as the program would.
func pressEdit(m *Model) {
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if cmd != nil {
		m.Update(cmd())
	}
}

func retitle(s string) string { return strings.Replace(s, "# T aaa", "# Renamed", 1) }

func TestEditFromList(t *testing.T) {
	m, st := newModel(t, "aaa -", "bbb -")
	locks := 0
	m.lock = func(bool) (func(), error) { locks++; return func() {}, nil }
	runs := fakeEditor(t, m, retitle)
	pressEdit(m)
	if *runs != 1 || m.status != "edited aaa" || m.cursor != 0 {
		t.Fatalf("runs=%d status=%q cursor=%d", *runs, m.status, m.cursor)
	}
	if tk, _ := st.Load("aaa"); tk.Title != "Renamed" {
		t.Errorf("disk title %q", tk.Title)
	}
	if !strings.Contains(m.View(), "Renamed") {
		t.Errorf("list not refreshed:\n%s", m.View())
	}
	if locks != 2 {
		t.Errorf("locks %d, want one to read and one to save", locks)
	}
}

func TestEditFromDetailRerendersAtTop(t *testing.T) {
	m, _ := newModel(t, "aaa -")
	m.t.Get("aaa").Body = strings.Repeat("line\n\n", 40)
	if err := m.t.Replace(m.t.Get("aaa")); err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	key(m, tea.KeyEnter)
	press(m, "G")
	fakeEditor(t, m, retitle)
	pressEdit(m)
	if m.detailID != "aaa" || m.scroll != 0 || m.status != "edited aaa" {
		t.Fatalf("detailID=%q scroll=%d status=%q", m.detailID, m.scroll, m.status)
	}
	if v := stripANSI(m.View()); !strings.Contains(v, "Renamed") || !strings.Contains(v, "edited aaa") {
		t.Errorf("detail not re-rendered:\n%s", v)
	}
}

func TestEditUnchanged(t *testing.T) {
	m, _ := newModel(t, "aaa -")
	fakeEditor(t, m, func(s string) string { return s })
	pressEdit(m)
	if m.status != "unchanged aaa" {
		t.Errorf("status %q", m.status)
	}
}

func TestEditFailuresSaveNothing(t *testing.T) {
	m, st := newModel(t, "aaa -")
	t.Setenv("EDITOR", "")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if cmd != nil || m.status != "$EDITOR is not set" {
		t.Errorf("no $EDITOR: cmd=%v status=%q", cmd != nil, m.status)
	}

	fakeEditor(t, m, func(string) string { return "garbage" })
	pressEdit(m)
	if !strings.Contains(m.status, "invalid, nothing saved") {
		t.Errorf("bad edit: status %q", m.status)
	}

	fakeEditor(t, m, retitle)
	write := m.exec
	m.exec = func(c *exec.Cmd, done tea.ExecCallback) tea.Cmd {
		write(c, done)()
		return func() tea.Msg { return done(errors.New("exit status 1")) }
	}
	pressEdit(m)
	if !strings.Contains(m.status, "editor: exit status 1") {
		t.Errorf("editor failed: status %q", m.status)
	}
	if tk, _ := st.Load("aaa"); tk.Title != "T aaa" {
		t.Errorf("disk changed: %q", tk.Title)
	}
}

func TestEditVanishedTicketClosesDetail(t *testing.T) {
	m, st := newModel(t, "aaa -", "bbb -")
	key(m, tea.KeyEnter)
	if err := st.Delete("aaa"); err != nil {
		t.Fatal(err)
	}
	fakeEditor(t, m, retitle)
	pressEdit(m)
	if m.detailID != "" || !strings.Contains(m.status, "no longer exists") {
		t.Errorf("detailID=%q status=%q", m.detailID, m.status)
	}
}
