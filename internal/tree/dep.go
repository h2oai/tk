package tree

import (
	"fmt"
	"slices"
)

// waits lists what x effectively waits for: its own blockers, its children
// (a parent finishes after them) and the blockers inherited from ancestors.
func (t *Tree) waits(x string) []string {
	var out []string
	add := func(ids []string) {
		for _, id := range ids {
			if t.tickets[id] != nil {
				out = append(out, id)
			}
		}
	}
	add(t.tickets[x].BlockedBy)
	add(t.tickets[x].Children)
	for _, a := range t.Ancestors(x) {
		add(t.tickets[a].BlockedBy)
	}
	return out
}

// reaches reports whether any target is reachable from start (inclusive).
func (t *Tree) reaches(start string, targets map[string]bool) bool {
	seen := map[string]bool{}
	stack := []string{start}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[n] || t.tickets[n] == nil {
			continue
		}
		if seen[n] = true; targets[n] {
			return true
		}
		stack = append(stack, t.waits(n)...)
	}
	return false
}

// checkDep validates adding "id waits on blocker".
func (t *Tree) checkDep(id, blocker string) error {
	switch {
	case id == blocker:
		return fmt.Errorf("%s: %w", id, ErrSelfDep)
	case t.isAncestor(blocker, id), t.isAncestor(id, blocker):
		return fmt.Errorf("%s and %s: %w", id, blocker, ErrRelatedDep)
	}
	targets := map[string]bool{}
	for _, s := range t.Subtree(id) {
		targets[s] = true
	}
	if t.reaches(blocker, targets) {
		return fmt.Errorf("%s waits on %s: %w", id, blocker, ErrDepCycle)
	}
	return nil
}

func (t *Tree) addDep(id, blocker string) error {
	tk := t.tickets[id]
	if slices.Contains(tk.BlockedBy, blocker) {
		return nil
	}
	if _, err := t.mustExist(blocker); err != nil {
		return err
	}
	if err := t.checkDep(id, blocker); err != nil {
		return err
	}
	tk.BlockedBy = append(slices.Clone(tk.BlockedBy), blocker)
	t.touch(id)
	return nil
}

// AddDep makes id wait on blocker. Adding an existing dependency is a no-op.
// It fails with ErrSelfDep, ErrRelatedDep or ErrDepCycle.
func (t *Tree) AddDep(id, blocker string) error {
	if _, err := t.mustExist(id); err != nil {
		return err
	}
	return t.apply(func() error { return t.addDep(id, blocker) })
}

// RemoveDep removes blocker from id's blocked-by. Absent entries are a no-op;
// the blocker need not exist, so dangling ids can be cleaned up.
func (t *Tree) RemoveDep(id, blocker string) error {
	tk, err := t.mustExist(id)
	if err != nil {
		return err
	}
	if !slices.Contains(tk.BlockedBy, blocker) {
		return nil
	}
	return t.apply(func() error {
		tk.BlockedBy = slices.DeleteFunc(slices.Clone(tk.BlockedBy), func(b string) bool { return b == blocker })
		t.touch(id)
		return nil
	})
}
