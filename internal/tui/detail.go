package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/h2oai/tk/internal/render"
)

const detailHelp = "j/k scroll  pgup/pgdn page  g/G top/bottom  J/K or ←/→ next/prev  e edit  esc back"

// renderDetail renders the ticket wrapped to width in the model's style. If
// glamour fails it falls back to the raw markdown.
func (m *Model) renderDetail(id string) []string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	out, err := render.Render(m.t, id, glamour.WithStandardStyle(m.style), glamour.WithWordWrap(width))
	if err != nil {
		out = render.Markdown(m.t, id)
	}
	return strings.Split(strings.Trim(out, "\n"), "\n")
}

// reload refreshes the tree from disk under a shared lock (when there is a
// lock) and puts the cursor on id. It reports false, with the reason in the
// status line, if that failed or id no longer exists or is archived.
func (m *Model) reload(id string) bool {
	if m.lock != nil {
		unlock, err := m.lock(false)
		if err != nil {
			m.status = err.Error()
			return false
		}
		defer unlock()
		if err := m.t.Reload(); err != nil {
			m.status = err.Error()
			return false
		}
		m.rebuild()
		m.follow(id)
	}
	if err := m.hidden(id); err != nil {
		m.status = err.Error()
		return false
	}
	return true
}

// show renders id's detail view from the current tree.
func (m *Model) show(id string) {
	m.status = ""
	m.detailID = id
	m.detail = m.renderDetail(id)
	m.scroll = 0
}

// open shows the selected ticket's detail view. With a lock it first reloads
// from disk so the content is current.
func (m *Model) open() {
	id := m.selected()
	if id == "" || !m.reload(id) {
		return
	}
	m.show(id)
}

// step moves the detail view to the next (delta 1) or previous (delta -1)
// ticket in list order, reloading from disk first. At either end it stays on
// the current ticket and says so in the status line. If the current ticket has
// vanished or been archived it returns to the list.
func (m *Model) step(delta int) {
	id := m.detailID
	if !m.reload(id) {
		m.closeDetail()
		return
	}
	m.follow(id)
	i := m.cursor + delta
	if i < 0 || i >= len(m.rows) {
		m.show(id)
		if delta > 0 {
			m.status = "last ticket"
		} else {
			m.status = "first ticket"
		}
		return
	}
	m.cursor = i
	m.show(m.rows[i].id)
}

func (m *Model) closeDetail() {
	m.detailID = ""
	m.detail = nil
}

func (m *Model) pageSize() int {
	if m.height > 3 {
		return m.height - 2
	}
	return len(m.detail)
}

func (m *Model) scrollBy(n int) {
	maxTop := max(0, len(m.detail)-m.pageSize())
	m.scroll = max(0, min(m.scroll+n, maxTop))
}

func (m *Model) updateDetailKey(key string) tea.Cmd {
	switch key {
	case "ctrl+c":
		return tea.Quit
	case "e":
		return m.startEdit(m.detailID)
	case "esc", "q":
		m.closeDetail()
	case "J", "right":
		m.step(1)
	case "K", "left":
		m.step(-1)
	case "j", "down":
		m.scrollBy(1)
	case "k", "up":
		m.scrollBy(-1)
	case "pgdown", " ":
		m.scrollBy(m.pageSize())
	case "pgup":
		m.scrollBy(-m.pageSize())
	case "g", "home":
		m.scroll = 0
	case "G", "end":
		m.scrollBy(len(m.detail))
	}
	return nil
}

func (m *Model) detailView() string {
	n := m.pageSize()
	end := min(m.scroll+n, len(m.detail))
	return strings.Join(m.detail[m.scroll:end], "\n") + "\n" + m.status + "\n" + detailHelp
}
