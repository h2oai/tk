// Package readytree renders ready tickets as a tree following parent links.
package readytree

import (
	"fmt"
	"io"

	"github.com/h2oai/tk/internal/ticket"
)

const (
	ansiReset  = "\x1b[0m"
	ansiDim    = "\x1b[2m"
	ansiRed    = "\x1b[31m"
	ansiYellow = "\x1b[33m"
)

// Render writes ready tickets as a tree of parent links. Ancestors of ready
// tickets that are not themselves ready appear as context nodes marked with
// their status. Siblings are ordered by sortField. With color set, lines are
// coloured by priority and context nodes are dimmed.
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
	s := fmt.Sprintf("%s P%d %s %s", t.ID, t.Priority, t.Type, t.Title)
	if !ready {
		s += fmt.Sprintf(" (%s)", t.Status)
	}
	if !color {
		return s
	}
	switch {
	case !ready:
		return ansiDim + s + ansiReset
	case t.Priority == 0:
		return ansiRed + s + ansiReset
	case t.Priority == 1:
		return ansiYellow + s + ansiReset
	case t.Priority >= 3:
		return ansiDim + s + ansiReset
	}
	return s
}
