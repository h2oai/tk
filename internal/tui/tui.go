// Package tui is a Bubble Tea interface for reordering tickets. It holds no
// ordering rules: every change goes through tree.Tree, and the view is rebuilt
// from the tree afterwards. It never edits ticket contents.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/h2oai/tk/internal/tree"
)

const help = "j/k move  J/K reorder  H/L outdent/indent  g/G top/bottom  q quit"

// row is one line of the outline.
type row struct {
	id    string
	depth int
}

// Model is the Bubble Tea model.
type Model struct {
	t      *tree.Tree
	rows   []row
	cursor int
	top    int // first visible row
	height int // terminal height, 0 until known
	status string
}

// New returns a model showing t.
func New(t *tree.Tree) *Model {
	m := &Model{t: t}
	m.rebuild()
	return m
}

// Run starts the program on the terminal and blocks until it quits.
func Run(t *tree.Tree) error {
	_, err := tea.NewProgram(New(t), tea.WithAltScreen()).Run()
	return err
}

// rebuild recomputes the rows from the tree, keeping the cursor in range.
func (m *Model) rebuild() {
	m.rows = m.rows[:0]
	for _, id := range m.t.Order() {
		m.rows = append(m.rows, row{id, len(m.t.Ancestors(id))})
	}
	m.cursor = max(0, min(m.cursor, len(m.rows)-1))
}

func (m *Model) selected() string {
	if m.cursor < len(m.rows) {
		return m.rows[m.cursor].id
	}
	return ""
}

func (m *Model) follow(id string) {
	for i, r := range m.rows {
		if r.id == id {
			m.cursor = i
			return
		}
	}
}

// mutate runs op on the selected ticket. Errors from the tree go to the
// status line.
func (m *Model) mutate(op func(*tree.Tree, string) error) {
	id := m.selected()
	if id == "" {
		return
	}
	if err := op(m.t, id); err != nil {
		m.status = err.Error()
		return
	}
	m.status = ""
	m.rebuild()
	m.follow(id)
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			m.cursor = min(m.cursor+1, len(m.rows)-1)
		case "k", "up":
			m.cursor = max(m.cursor-1, 0)
		case "J":
			m.mutate((*tree.Tree).Down)
		case "K":
			m.mutate((*tree.Tree).Up)
		case "H":
			m.mutate((*tree.Tree).Outdent)
		case "L":
			m.mutate((*tree.Tree).Indent)
		case "g":
			m.mutate((*tree.Tree).Top)
		case "G":
			m.mutate((*tree.Tree).Bottom)
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m *Model) View() string {
	var b strings.Builder
	n := len(m.rows)
	if m.height > 3 {
		n = m.height - 2 // leave room for the status and help lines
	}
	if m.cursor < m.top {
		m.top = m.cursor
	} else if m.cursor >= m.top+n {
		m.top = m.cursor - n + 1
	}
	for i := m.top; i < min(m.top+n, len(m.rows)); i++ {
		r := m.rows[i]
		tk := m.t.Get(r.id)
		cur := "  "
		if i == m.cursor {
			cur = "> "
		}
		fmt.Fprintf(&b, "%s%s%s %s %s  [%s] %s\n", cur, strings.Repeat("  ", r.depth), m.t.Position(r.id), tk.Status.Marker(), r.id, tk.Type, tk.Title)
	}
	if len(m.rows) == 0 {
		b.WriteString("no tickets\n")
	}
	b.WriteString(m.status + "\n" + help)
	return b.String()
}
