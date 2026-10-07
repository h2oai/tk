package tree

import (
	"fmt"
	"slices"
	"strings"

	"github.com/h2oai/tk/internal/store"
)

// Archive moves the subtree rooted at id out of the live tree into the
// store's archive directory, files unchanged. Every ticket in the subtree must
// be closed (ErrNotClosed) unless force, which closes them first. It is
// refused, force or not, while a blocked-by edge links a subtree member with a
// live ticket outside it (ErrCrossDep). The live tree keeps no reference to the
// archived tickets. It returns the archived tickets in DFS preorder.
//
// The move is best-effort: lists are rewritten first, then files are renamed
// one by one, so a crash leaves orphaned files that fsck reports.
func (t *Tree) Archive(id string, force bool) ([]*store.Ticket, error) {
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
	moved := make([]*store.Ticket, len(sub))
	for i, s := range sub {
		moved[i] = t.tickets[s]
	}
	err := t.apply(func() error {
		var changed []string
		for _, s := range sub {
			t.setStatus(s, store.StatusClosed, &changed)
			for _, owner := range t.owners[s] {
				if owner == "" || !inSub(owner) {
					t.setSiblings(owner, without(t.siblings(owner), s))
				}
			}
		}
		t.archived = sub
		return nil
	})
	if err != nil {
		return nil, err
	}
	return moved, nil
}
