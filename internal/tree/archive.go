package tree

import (
	"fmt"
	"slices"

	"github.com/h2oai/tk/internal/store"
)

// Archive marks every ticket in the subtree rooted at id as archived. The
// tickets keep their files and their place in the tree; ls, the TUI and
// positions skip them. Every ticket in the subtree must be closed
// (ErrNotClosed) unless force, which closes them first. It returns the tickets
// that were not archived yet, in DFS preorder. With dryRun it runs the same
// checks and returns the same tickets without changing anything.
func (t *Tree) Archive(id string, force, dryRun bool) ([]*store.Ticket, error) {
	if _, err := t.mustExist(id); err != nil {
		return nil, err
	}
	sub := t.Subtree(id)
	if !force && t.hasUnclosed(sub) {
		return nil, fmt.Errorf("archive %s: subtree %w (use force to close them first)", id, ErrNotClosed)
	}
	return t.setArchived(sub, true, dryRun)
}

// ArchiveClosed archives every subtree whose tickets are all closed and that
// is not archived yet, as one batch. It returns the newly archived tickets in
// DFS preorder. With dryRun nothing is changed.
func (t *Tree) ArchiveClosed(dryRun bool) ([]*store.Ticket, error) {
	// A ticket qualifies when it and its whole subtree are closed; walking
	// the preorder backwards sees children before their parent.
	ok := map[string]bool{}
	for i := len(t.order) - 1; i >= 0; i-- {
		id := t.order[i]
		ok[id] = t.tickets[id].Status == store.StatusClosed
		for _, c := range t.Children(id) {
			ok[id] = ok[id] && ok[c]
		}
	}
	var set []string
	for _, id := range t.order {
		if ok[id] {
			set = append(set, id)
		}
	}
	return t.setArchived(set, true, dryRun)
}

// Unarchive clears the archived flag on id, its subtree and its archived
// ancestors, so no archived ticket is left above a live one. It returns the
// tickets that changed in DFS preorder. With dryRun nothing is changed.
func (t *Tree) Unarchive(id string, dryRun bool) ([]*store.Ticket, error) {
	if _, err := t.mustExist(id); err != nil {
		return nil, err
	}
	set := t.Ancestors(id)
	slices.Reverse(set)
	return t.setArchived(append(set, t.Subtree(id)...), false, dryRun)
}

// setArchived sets the archived flag to on for every ticket in ids (in DFS
// preorder) whose flag differs, closing them when archiving. It returns the
// tickets that change.
func (t *Tree) setArchived(ids []string, on, dryRun bool) ([]*store.Ticket, error) {
	if err := t.writable(); err != nil {
		return nil, err
	}
	var set []*store.Ticket
	for _, id := range ids {
		if tk := t.tickets[id]; tk.Archived != on {
			set = append(set, tk)
		}
	}
	if dryRun || len(set) == 0 {
		return set, nil
	}
	// Tickets are saved in the order they are touched. Archive bottom-up and
	// unarchive top-down, so a save that fails midway never leaves an
	// archived ticket above a live one.
	saveOrder := slices.Clone(set)
	if on {
		slices.Reverse(saveOrder)
	}
	err := t.apply(func() error {
		var changed []string
		for _, tk := range saveOrder {
			if on {
				t.setStatus(tk.ID, store.StatusClosed, &changed)
			}
			tk.Archived = on
			t.touch(tk.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return set, nil
}
