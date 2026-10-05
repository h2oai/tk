package tree

import (
	"fmt"

	"github.com/h2oai/tk/internal/store"
)

// setStatus changes a status and marks the ticket dirty, reporting a change.
func (t *Tree) setStatus(id string, s store.Status, changed *[]string) {
	if tk := t.tickets[id]; tk.Status != s {
		tk.Status = s
		t.touch(id)
		*changed = append(*changed, id)
	}
}

// reopenAncestors puts closed ancestors back to open.
func (t *Tree) reopenAncestors(id string, changed *[]string) {
	for _, a := range t.Ancestors(id) {
		if t.tickets[a].Status == store.StatusClosed {
			t.setStatus(a, store.StatusOpen, changed)
		}
	}
}

// Start sets in_progress. It fails with ErrNotLeaf while descendants are
// unclosed. Closed ancestors are reopened. It returns the changed ids.
func (t *Tree) Start(id string) ([]string, error) {
	if _, err := t.mustExist(id); err != nil {
		return nil, err
	}
	if t.hasUnclosed(t.Descendants(id)) {
		return nil, fmt.Errorf("%s: %w", id, ErrNotLeaf)
	}
	var changed []string
	err := t.apply(func() error {
		t.setStatus(id, store.StatusInProgress, &changed)
		t.reopenAncestors(id, &changed)
		return nil
	})
	return changed, err
}

// Close closes id. While descendants are unclosed it fails with
// ErrOpenDescendants unless force, which closes them too. It returns the
// changed ids, descendants first.
func (t *Tree) Close(id string, force bool) ([]string, error) {
	if _, err := t.mustExist(id); err != nil {
		return nil, err
	}
	desc := t.Descendants(id)
	if !force && t.hasUnclosed(desc) {
		return nil, fmt.Errorf("%s: %w (use force to close them too)", id, ErrOpenDescendants)
	}
	var changed []string
	err := t.apply(func() error {
		for i := len(desc) - 1; i >= 0; i-- {
			t.setStatus(desc[i], store.StatusClosed, &changed)
		}
		t.setStatus(id, store.StatusClosed, &changed)
		return nil
	})
	return changed, err
}

// Reopen sets open and reopens closed ancestors. It returns the changed ids.
func (t *Tree) Reopen(id string) ([]string, error) {
	if _, err := t.mustExist(id); err != nil {
		return nil, err
	}
	var changed []string
	err := t.apply(func() error {
		t.setStatus(id, store.StatusOpen, &changed)
		t.reopenAncestors(id, &changed)
		return nil
	})
	return changed, err
}
