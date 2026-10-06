package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/h2oai/tk/internal/tree"
)

const detailHelp = "j/k scroll  pgup/pgdn page  g/G top/bottom  esc back"

// markdown builds the document shown in the detail view: heading, metadata,
// children, then the ticket body.
func markdown(t *tree.Tree, id string) string {
	tk := t.Get(id)
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", tk.Title)
	fmt.Fprintf(&b, "- **id:** %s\n", id)
	fmt.Fprintf(&b, "- **status:** %s\n", tk.Status)
	fmt.Fprintf(&b, "- **type:** %s\n", tk.Type)
	fmt.Fprintf(&b, "- **position:** %s\n", t.Position(id))
	if p := t.Parent(id); p != "" {
		fmt.Fprintf(&b, "- **parent:** %s %q\n", p, t.Get(p).Title)
	}
	fmt.Fprintf(&b, "- **created:** %s\n", tk.Created.Format("2006-01-02T15:04:05Z"))
	if len(tk.BlockedBy) > 0 {
		b.WriteString("- **blocked-by:**\n")
		for _, d := range tk.BlockedBy {
			if dt := t.Get(d); dt != nil {
				fmt.Fprintf(&b, "  - %s %q (%s, %s)\n", d, dt.Title, t.Position(d), dt.Status)
			} else {
				fmt.Fprintf(&b, "  - %s (missing)\n", d)
			}
		}
	}
	if kids := t.Children(id); len(kids) > 0 {
		b.WriteString("\n## Children\n\n")
		for _, c := range kids {
			fmt.Fprintf(&b, "- %s %s %s  %s\n", t.Position(c), t.Get(c).Status.Marker(), c, t.Get(c).Title)
		}
	}
	if body := strings.TrimSpace(tk.Body); body != "" {
		fmt.Fprintf(&b, "\n---\n\n%s\n", body)
	}
	return b.String()
}

// renderDetail renders the ticket's markdown wrapped to width. If glamour
// fails it falls back to the raw markdown.
func renderDetail(t *tree.Tree, id string, width int) []string {
	md := markdown(t, id)
	if width <= 0 {
		width = 80
	}
	out := md
	if r, err := glamour.NewTermRenderer(glamour.WithStandardStyle("dark"), glamour.WithWordWrap(width)); err == nil {
		if s, err := r.Render(md); err == nil {
			out = s
		}
	}
	return strings.Split(strings.Trim(out, "\n"), "\n")
}

// open shows the selected ticket's detail view. With a lock it first reloads
// from disk so the content is current.
func (m *Model) open() {
	id := m.selected()
	if id == "" {
		return
	}
	if m.lock != nil {
		unlock, err := m.lock(false)
		if err != nil {
			m.status = err.Error()
			return
		}
		defer unlock()
		if err := m.t.Reload(); err != nil {
			m.status = err.Error()
			return
		}
		m.rebuild()
		m.follow(id)
	}
	if m.t.Get(id) == nil {
		m.status = id + " no longer exists"
		return
	}
	m.status = ""
	m.detailID = id
	m.detail = renderDetail(m.t, id, m.width)
	m.scroll = 0
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

func (m *Model) updateDetailKey(key string) bool {
	switch key {
	case "ctrl+c":
		return true
	case "esc", "q":
		m.closeDetail()
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
	return false
}

func (m *Model) detailView() string {
	n := m.pageSize()
	end := min(m.scroll+n, len(m.detail))
	return strings.Join(m.detail[m.scroll:end], "\n") + "\n" + m.status + "\n" + detailHelp
}
