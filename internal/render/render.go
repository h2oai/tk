// Package render turns a ticket into markdown and renders it for a terminal
// with glamour. It is shared by `tk show --pretty` and the TUI detail view, so
// both show the same document.
package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/h2oai/tk/internal/tree"
	"github.com/muesli/termenv"
)

// Markdown builds the document for id: heading, metadata, children, then the
// ticket body.
func Markdown(t *tree.Tree, id string) string {
	tk := t.Get(id)
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", tk.Title)
	fmt.Fprintf(&b, "- **id:** %s\n", id)
	fmt.Fprintf(&b, "- **status:** %s\n", tk.Status)
	fmt.Fprintf(&b, "- **type:** %s\n", tk.Type)
	fmt.Fprintf(&b, "- **effort:** %s\n", tk.Effort)
	if tk.Archived {
		b.WriteString("- **archived:** true\n")
	}
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
			pos := t.Position(c)
			if pos == "-" {
				pos = `\-` // an archived child's "- -" would open a nested list
			}
			fmt.Fprintf(&b, "- %s %s %s  %s\n", pos, t.Get(c).Marker(), c, t.Get(c).Title)
		}
	}
	if body := strings.TrimSpace(tk.Body); body != "" {
		fmt.Fprintf(&b, "\n---\n\n%s\n", body)
	}
	return b.String()
}

// Render renders id's markdown with glamour and the given options.
func Render(t *tree.Tree, id string, opts ...glamour.TermRendererOption) (string, error) {
	r, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return "", err
	}
	return r.Render(Markdown(t, id))
}

// TerminalStyle reports the glamour style matching the terminal background,
// dark or light. It queries the terminal, so call it before a Bubble Tea
// program starts reading input.
func TerminalStyle() string {
	if termenv.HasDarkBackground() {
		return styles.DarkStyle
	}
	return styles.LightStyle
}
