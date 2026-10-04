// Package readytree renders ready tickets as a tree following parent links.
package readytree

import (
	"fmt"
	"io"

	"github.com/h2oai/tk/internal/ticket"
)

const (
	ansiReset   = "\x1b[0m"
	ansiDim     = "\x1b[2m"
	ansiRed     = "\x1b[31m"
	ansiYellow  = "\x1b[33m"
	ansiGreen   = "\x1b[32m"
	ansiMagenta = "\x1b[35m"
	ansiCyan    = "\x1b[36m"
)

// Render writes ready tickets as a tree of parent links. Ancestors of ready
// tickets that are not themselves ready appear as context nodes marked with
// their status. Ready tickets that are already in progress are prefixed with a
// "▶" marker so active work stands out from merely available work. Siblings are
// ordered by sortField. With color set, lines are coloured by priority and
// context nodes are dimmed.
func Render(w io.Writer, all, ready []*ticket.Ticket, sortField string, color bool) error {
	byID := make(map[string]*ticket.Ticket, len(all))
	for _, t := range all {
		byID[t.ID] = t
	}
	isReady := make(map[string]bool, len(ready))
	for _, t := range ready {
		isReady[t.ID] = true
	}

	// parentOf holds only edges to existing tickets. Cycles are broken so every
	// included node is reachable from a root.
	parentOf := make(map[string]string)
	included := make(map[string]*ticket.Ticket)
	for _, t := range ready {
		chain := map[string]bool{}
		for cur := t; cur != nil; {
			if _, ok := included[cur.ID]; ok && cur != t {
				break
			}
			included[cur.ID] = cur
			chain[cur.ID] = true
			p, ok := byID[cur.Parent]
			if !ok || chain[p.ID] {
				break
			}
			parentOf[cur.ID] = p.ID
			cur = p
		}
	}

	children := make(map[string][]*ticket.Ticket)
	var roots []*ticket.Ticket
	for id, t := range included {
		if p, ok := parentOf[id]; ok {
			children[p] = append(children[p], t)
		} else {
			roots = append(roots, t)
		}
	}
	if err := ticket.SortBy(roots, sortField); err != nil {
		return err
	}
	for _, c := range children {
		if err := ticket.SortBy(c, sortField); err != nil {
			return err
		}
	}

	var walk func(t *ticket.Ticket, prefix, connector, childPrefix string)
	walk = func(t *ticket.Ticket, prefix, connector, childPrefix string) {
		fmt.Fprintln(w, prefix+connector+line(t, isReady[t.ID], color))
		kids := children[t.ID]
		for i, k := range kids {
			if i == len(kids)-1 {
				walk(k, childPrefix, "└── ", childPrefix+"    ")
			} else {
				walk(k, childPrefix, "├── ", childPrefix+"│   ")
			}
		}
	}
	for _, r := range roots {
		walk(r, "", "", "")
	}
	return nil
}

func line(t *ticket.Ticket, ready, color bool) string {
	inProgress := ready && t.Status == ticket.StatusInProgress

	suffix := ""
	if !ready {
		suffix = fmt.Sprintf(" (%s)", t.Status)
	}
	if !color {
		marker := ""
		if inProgress {
			marker = "▶ "
		}
		return fmt.Sprintf("%s%s P%d %s %s%s", marker, t.ID, t.Priority, t.Type, t.Title, suffix)
	}
	if !ready {
		return ansiDim + fmt.Sprintf("%s P%d %s %s%s", t.ID, t.Priority, t.Type, t.Title, suffix) + ansiReset
	}
	id := t.ID
	if inProgress {
		id = paint(ansiCyan, "▶") + " " + t.ID
	}
	return fmt.Sprintf("%s %s %s %s", id,
		paint(priorityColor(t.Priority), fmt.Sprintf("P%d", t.Priority)),
		paint(typeColor(t.Type), string(t.Type)),
		t.Title)
}

func priorityColor(p int) string {
	switch {
	case p == 0:
		return ansiRed
	case p == 1:
		return ansiYellow
	case p >= 3:
		return ansiDim
	}
	return ""
}

func typeColor(t ticket.Type) string {
	switch t {
	case ticket.TypeBug:
		return ansiRed
	case ticket.TypeFeature:
		return ansiGreen
	case ticket.TypeEpic:
		return ansiMagenta
	}
	return ""
}

func paint(code, s string) string {
	if code == "" {
		return s
	}
	return code + s + ansiReset
}
