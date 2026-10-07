package tree

import (
	"fmt"
	"slices"
	"strings"

	"github.com/h2oai/tk/internal/store"
)

// Skip is a closed ticket that ArchiveClosed left in the live tree because a
// blocked-by edge links it with a ticket that stays live.
type Skip struct {
	ID      string
	WaitsOn []string // live tickets it lists in blocked-by
	Blocks  []string // live tickets that list it in blocked-by
}

func (s Skip) String() string {
	var parts []string
	if len(s.WaitsOn) > 0 {
		parts = append(parts, "waits on "+strings.Join(s.WaitsOn, ", "))
	}
	if len(s.Blocks) > 0 {
		parts = append(parts, "blocks "+strings.Join(s.Blocks, ", "))
	}
	return strings.Join(parts, ", ")
}

// Archive moves the subtree rooted at id out of the live tree into the
// store's archive directory, files unchanged. Every ticket in the subtree must
// be closed (ErrNotClosed) unless force, which closes them first. It is
// refused, force or not, while a blocked-by edge links a subtree member with a
// live ticket outside it (ErrCrossDep). The live tree keeps no reference to the
// archived tickets. It returns the archived tickets in DFS preorder. With
// dryRun it runs the same checks and returns the same tickets without changing
// anything.
//
// The move is best-effort: lists are rewritten first, then files are renamed
// one by one, so a crash leaves orphaned files that fsck reports.
func (t *Tree) Archive(id string, force, dryRun bool) ([]*store.Ticket, error) {
	if _, err := t.mustExist(id); err != nil {
		return nil, err
	}
	sub := t.Subtree(id)
	inSub := func(s string) bool { return slices.Contains(sub, s) }
	var crossing []string
	for _, h := range t.sortedIDs() {
		for _, b := range t.tickets[h].BlockedBy {
			if inSub(h) != inSub(b) && t.tickets[b] != nil {
				crossing = append(crossing, fmt.Sprintf("%s waits on %s", h, b))
			}
		}
	}
	if len(crossing) > 0 {
		return nil, fmt.Errorf("archive %s: %w (%s)", id, ErrCrossDep, strings.Join(crossing, ", "))
	}
	if !force && t.hasUnclosed(sub) {
		return nil, fmt.Errorf("archive %s: subtree %w (use force to close them first)", id, ErrNotClosed)
	}
	return t.archiveSet(sub, dryRun)
}

// ArchiveClosed archives every maximal subtree whose tickets are all closed,
// as one batch. A blocked-by edge counts as crossing only when its other end
// stays live, so edges between archived tickets move with them. A ticket with
// a crossing edge stays live together with its ancestors, which can make
// further tickets cross; selection repeats until nothing changes. Closed
// tickets elsewhere under a kept ancestor are still archived. It returns the
// archived tickets in DFS preorder and the skipped tickets that cross, in DFS
// preorder. With dryRun nothing is changed.
func (t *Tree) ArchiveClosed(dryRun bool) ([]*store.Ticket, []Skip, error) {
	// Candidates: reachable tickets whose whole subtree is closed.
	cand := map[string]bool{}
	for i := len(t.order) - 1; i >= 0; i-- {
		id := t.order[i]
		ok := t.tickets[id].Status == store.StatusClosed
		for _, c := range t.Children(id) {
			ok = ok && cand[c]
		}
		cand[id] = ok
	}
	heldBy := map[string][]string{} // blocker -> tickets listing it, sorted
	for _, h := range t.sortedIDs() {
		for _, b := range t.tickets[h].BlockedBy {
			heldBy[b] = append(heldBy[b], h)
		}
	}
	live := func(id string) bool { return t.tickets[id] != nil && !cand[id] }
	crossing := func(id string) (s Skip, ok bool) {
		s.ID = id
		for _, b := range t.tickets[id].BlockedBy {
			if live(b) {
				s.WaitsOn = append(s.WaitsOn, b)
			}
		}
		for _, h := range heldBy[id] {
			if live(h) {
				s.Blocks = append(s.Blocks, h)
			}
		}
		return s, s.WaitsOn != nil || s.Blocks != nil
	}
	skipped := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, id := range t.order {
			if _, ok := crossing(id); cand[id] && ok {
				skipped[id] = true
				for _, k := range append([]string{id}, t.Ancestors(id)...) {
					cand[k] = false
				}
				changed = true
			}
		}
	}
	var set []string
	var skips []Skip
	for _, id := range t.order {
		if cand[id] {
			set = append(set, id)
		} else if skipped[id] {
			s, _ := crossing(id)
			skips = append(skips, s)
		}
	}
	moved, err := t.archiveSet(set, dryRun)
	if err != nil {
		return nil, nil, err
	}
	return moved, skips, nil
}

// archiveSet closes and archives set, a list of tickets in DFS preorder whose
// descendants are all in it, detaching each from every list outside the set.
func (t *Tree) archiveSet(set []string, dryRun bool) ([]*store.Ticket, error) {
	if err := t.writable(); err != nil {
		return nil, err
	}
	moved := make([]*store.Ticket, len(set))
	for i, s := range set {
		moved[i] = t.tickets[s]
	}
	if dryRun || len(set) == 0 {
		return moved, nil
	}
	inSet := func(s string) bool { return slices.Contains(set, s) }
	err := t.apply(func() error {
		var changed []string
		for _, s := range set {
			t.setStatus(s, store.StatusClosed, &changed)
			for _, owner := range t.owners[s] {
				if owner == "" || !inSet(owner) {
					t.setSiblings(owner, without(t.siblings(owner), s))
				}
			}
		}
		t.archived = set
		return nil
	})
	if err != nil {
		return nil, err
	}
	return moved, nil
}
