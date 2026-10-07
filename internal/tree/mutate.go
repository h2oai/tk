package tree

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/h2oai/tk/internal/store"
)

// Place says where to insert among siblings. At most one field may be set;
// none means append. Index is 1-based and clamped to the end; 0 and -1
// append. Before and After name a sibling under the target parent.
type Place struct {
	Index         int
	Before, After string
}

// insert returns list (which must not contain id) with id placed per p.
func (p Place) insert(list []string, id string) ([]string, error) {
	n := 0
	for _, set := range []bool{p.Index != 0, p.Before != "", p.After != ""} {
		if set {
			n++
		}
	}
	i := len(list)
	switch {
	case n > 1:
		return nil, fmt.Errorf("%w: use only one of index, before, after", ErrBadPlace)
	case p.Index < -1:
		return nil, fmt.Errorf("%w: index %d", ErrBadPlace, p.Index)
	case p.Index > 0:
		i = min(p.Index-1, len(list))
	case p.Before != "" || p.After != "":
		anchor := p.Before + p.After
		if anchor == id {
			return nil, fmt.Errorf("%w: cannot anchor %s to itself", ErrBadPlace, id)
		}
		i = slices.Index(list, anchor)
		if i < 0 {
			return nil, fmt.Errorf("%w: %s is not a sibling at the target parent", ErrBadPlace, anchor)
		}
		if p.After != "" {
			i++
		}
	}
	return slices.Insert(slices.Clone(list), i, id), nil
}

func without(list []string, id string) []string {
	return slices.DeleteFunc(slices.Clone(list), func(s string) bool { return s == id })
}

// checkParent validates that parent is "" or an existing ticket.
func (t *Tree) checkParent(parent string) error {
	if parent == "" {
		return nil
	}
	_, err := t.mustExist(parent)
	return err
}

// adopt resets a closed or in_progress parent to open when it gains a child.
// Closed ancestors are reopened too if the newcomer has unclosed work.
func (t *Tree) adopt(parent string, unclosed bool) {
	if parent == "" {
		return
	}
	var changed []string
	switch p := t.tickets[parent]; {
	case p.Status == store.StatusInProgress, p.Status == store.StatusClosed && unclosed:
		t.setStatus(parent, store.StatusOpen, &changed)
	}
	if unclosed {
		t.reopenAncestors(parent, &changed)
	}
}

// Add stores a new childless ticket under parent ("" for a root).
func (t *Tree) Add(tk *store.Ticket, parent string, p Place) error {
	if err := tk.Validate(); err != nil {
		return err
	}
	if len(tk.Children) > 0 {
		return fmt.Errorf("new ticket %s must not have children", tk.ID)
	}
	if t.tickets[tk.ID] != nil {
		return fmt.Errorf("%s: %w", tk.ID, ErrExists)
	}
	if err := t.checkParent(parent); err != nil {
		return err
	}
	list, err := p.insert(t.siblings(parent), tk.ID)
	if err != nil {
		return err
	}
	return t.apply(func() error {
		t.tickets[tk.ID] = tk
		t.touch(tk.ID)
		t.adopt(parent, tk.Status != store.StatusClosed)
		t.setSiblings(parent, list)
		deps := tk.BlockedBy
		tk.BlockedBy = nil
		t.reindex()
		for _, b := range deps {
			if err := t.addDep(tk.ID, b); err != nil {
				return err
			}
			t.reindex()
		}
		return nil
	})
}

// Move reparents and/or repositions id. parent "" means root. It fails with
// ErrCycle when parent is id or one of its descendants, and rejects moves that
// introduce a dependency problem.
func (t *Tree) Move(id, parent string, p Place) error {
	if _, err := t.mustExist(id); err != nil {
		return err
	}
	if err := t.checkParent(parent); err != nil {
		return err
	}
	if parent == id || slices.Contains(t.Descendants(id), parent) {
		return fmt.Errorf("move %s under %s: %w", id, parent, ErrCycle)
	}
	old := t.parent[id]
	list, err := p.insert(without(t.siblings(parent), id), id)
	if err != nil {
		return err
	}
	before := depKeys(t.depProblems())
	return t.apply(func() error {
		if old != parent {
			t.adopt(parent, t.hasUnclosed(t.Subtree(id)))
		}
		t.setSiblings(parent, list)
		if old != parent {
			t.setSiblings(old, without(t.siblings(old), id))
		}
		t.reindex()
		for _, pr := range t.depProblems() {
			if !before[pr.key()] {
				return fmt.Errorf("move %s: %s: %w", id, pr.Msg, pr.Err)
			}
		}
		return nil
	})
}

// Up moves id one place earlier among its siblings; at the edge it is a no-op.
func (t *Tree) Up(id string) error { return t.shift(id, func(i, n int) int { return max(i-1, 0) }) }

// Down moves id one place later among its siblings.
func (t *Tree) Down(id string) error {
	return t.shift(id, func(i, n int) int { return min(i+1, n-1) })
}

// Top moves id to the first place among its siblings.
func (t *Tree) Top(id string) error { return t.shift(id, func(i, n int) int { return 0 }) }

// Bottom moves id to the last place among its siblings.
func (t *Tree) Bottom(id string) error { return t.shift(id, func(i, n int) int { return n - 1 }) }

func (t *Tree) shift(id string, target func(i, n int) int) error {
	if _, err := t.mustExist(id); err != nil {
		return err
	}
	parent := t.parent[id]
	list := t.siblings(parent)
	i := slices.Index(list, id)
	if i < 0 {
		return fmt.Errorf("%s is not listed under its parent; run fsck", id)
	}
	to := target(i, len(list))
	if to == i {
		return nil
	}
	return t.Move(id, parent, Place{Index: to + 1})
}

// Indent makes id the last child of its previous sibling. It is a no-op when
// id is the first of its siblings.
func (t *Tree) Indent(id string) error {
	if _, err := t.mustExist(id); err != nil {
		return err
	}
	list := t.siblings(t.parent[id])
	i := slices.Index(list, id)
	for i--; i >= 0; i-- {
		if t.tickets[list[i]] != nil {
			return t.Move(id, list[i], Place{})
		}
	}
	return nil
}

// Outdent makes id the next sibling of its parent. It is a no-op for a root.
func (t *Tree) Outdent(id string) error {
	if _, err := t.mustExist(id); err != nil {
		return err
	}
	parent := t.parent[id]
	if parent == "" {
		return nil
	}
	return t.Move(id, t.parent[parent], Place{After: parent})
}

// Remove deletes id. Without force it refuses when id has children or blocks
// another ticket (ErrHasChildren, ErrIsBlocker). With force it deletes the
// whole subtree and detaches it from other tickets' blocked-by.
func (t *Tree) Remove(id string, force bool) ([]string, error) {
	tk, err := t.mustExist(id)
	if err != nil {
		return nil, err
	}
	if !force && len(tk.Children) > 0 {
		return nil, fmt.Errorf("%s: %w (use force to delete the subtree)", id, ErrHasChildren)
	}
	gone := []string{id}
	if force {
		gone = t.Subtree(id)
	}
	var holders []string
	for _, h := range t.sortedIDs() {
		if slices.Contains(gone, h) {
			continue
		}
		for _, b := range t.tickets[h].BlockedBy {
			if slices.Contains(gone, b) {
				holders = append(holders, h)
				break
			}
		}
	}
	if !force && len(holders) > 0 {
		return nil, fmt.Errorf("%s: %w (%s)", id, ErrIsBlocker, strings.Join(holders, ", "))
	}
	err = t.apply(func() error {
		parent := t.parent[id]
		if t.isRoot(id) {
			parent = ""
		}
		if t.isListed(parent, id) {
			t.setSiblings(parent, without(t.siblings(parent), id))
		}
		for _, h := range holders {
			ht := t.tickets[h]
			ht.BlockedBy = slices.DeleteFunc(slices.Clone(ht.BlockedBy), func(b string) bool { return slices.Contains(gone, b) })
			t.touch(h)
		}
		t.gone = gone
		return nil
	})
	if err != nil {
		return nil, err
	}
	return gone, nil
}

func (t *Tree) isRoot(id string) bool { return slices.Contains(t.roots, id) }

func (t *Tree) isListed(parent, id string) bool {
	return (parent == "" || t.tickets[parent] != nil) && slices.Contains(t.siblings(parent), id)
}

// Replace overwrites the stored ticket with tk (same id) after an edit. It
// refuses to change status, children or blocked-by: those go through the
// commands that apply the tree rules.
func (t *Tree) Replace(tk *store.Ticket) error {
	cur, err := t.mustExist(tk.ID)
	if err != nil {
		return err
	}
	switch {
	case tk.Status != cur.Status:
		return fmt.Errorf("%s: status cannot be edited, use tk start, close or reopen", tk.ID)
	case !slices.Equal(tk.Children, cur.Children):
		return fmt.Errorf("%s: children cannot be edited, use tk mv", tk.ID)
	case !slices.Equal(tk.BlockedBy, cur.BlockedBy):
		return fmt.Errorf("%s: blocked-by cannot be edited, use tk dep or undep", tk.ID)
	}
	return t.apply(func() error {
		t.tickets[tk.ID] = tk
		t.touch(tk.ID)
		return nil
	})
}

// AppendNote appends a timestamped note section to the ticket's body.
func (t *Tree) AppendNote(id, text string, at time.Time) error {
	tk, err := t.mustExist(id)
	if err != nil {
		return err
	}
	return t.apply(func() error {
		tk.Body = strings.TrimRight(tk.Body, "\n")
		if tk.Body != "" {
			tk.Body += "\n\n"
		}
		tk.Body += fmt.Sprintf("## Note %s\n\n%s", at.UTC().Format(time.RFC3339), text)
		t.touch(id)
		return nil
	})
}

// SetType sets the ticket type. It is pure metadata: no status or ancestor
// effects, and setting the current type is a no-op. It reports whether
// anything changed.
func (t *Tree) SetType(id string, ty store.Type) (bool, error) {
	if !ty.Valid() {
		_, err := store.ParseType(string(ty))
		return false, err
	}
	tk, err := t.mustExist(id)
	if err != nil {
		return false, err
	}
	if tk.Type == ty {
		return false, nil
	}
	err = t.apply(func() error {
		tk.Type = ty
		t.touch(id)
		return nil
	})
	return err == nil, err
}
