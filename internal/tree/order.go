package tree

import (
	"fmt"
	"slices"

	"github.com/h2oai/tk/internal/store"
)

// Misorder is a ticket that sits above an unclosed blocker it waits on, so
// ready will stop at it once it is the top leaf. Holder is the ticket that
// declares the blocker; descendants inherit it, so they are not listed.
type Misorder struct {
	Holder     *store.Ticket
	Blocker    *store.Ticket
	HolderPos  string
	BlockerPos string
}

func (m Misorder) key() string { return m.Holder.ID + "|" + m.Blocker.ID }

// String formats a misorder as a one-line warning body.
func (m Misorder) String() string {
	return fmt.Sprintf("%s %q (%s) is above its blocker %s %q (%s, %s)",
		m.Holder.ID, m.Holder.Title, m.HolderPos,
		m.Blocker.ID, m.Blocker.Title, m.BlockerPos, m.Blocker.Status)
}

// Misorders lists every unclosed ticket that precedes one of its unclosed
// blockers in DFS order, in tree order.
func (t *Tree) Misorders() []Misorder {
	idx := make(map[string]int, len(t.order))
	for i, id := range t.order {
		idx[id] = i
	}
	var out []Misorder
	for i, id := range t.order {
		tk := t.tickets[id]
		if tk.Status == store.StatusClosed {
			continue
		}
		for _, b := range tk.BlockedBy {
			bt := t.tickets[b]
			j, ok := idx[b]
			if bt == nil || bt.Status == store.StatusClosed || !ok || j < i {
				continue
			}
			out = append(out, Misorder{tk, bt, t.Position(id), t.Position(b)})
		}
	}
	return out
}

// MisordersSince returns the misorders now present that were not in before
// and involve id or its descendants, as holder or blocker.
func (t *Tree) MisordersSince(before []Misorder, id string) []Misorder {
	old := make(map[string]bool, len(before))
	for _, m := range before {
		old[m.key()] = true
	}
	sub := t.Subtree(id)
	var out []Misorder
	for _, m := range t.Misorders() {
		if !old[m.key()] && (slices.Contains(sub, m.Holder.ID) || slices.Contains(sub, m.Blocker.ID)) {
			out = append(out, m)
		}
	}
	return out
}
