package tree

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/h2oai/tk/internal/store"
)

// Problem kinds reported by Fsck.
const (
	KindUnreadable = "unreadable"
	KindOrphan     = "orphan"
	KindDuplicate  = "duplicate"
	KindDangling   = "dangling"
	KindCycle      = "cycle"
	KindDep        = "dep"
	KindStatus     = "status"
)

// Problem is one integrity violation.
type Problem struct {
	Kind string
	ID   string // ticket concerned ("ROOT" for ROOT.md)
	Msg  string
	Err  error // sentinel for dep rule violations, else nil
}

func (p Problem) String() string { return fmt.Sprintf("%s: %s: %s", p.Kind, p.ID, p.Msg) }

func (p Problem) key() string { return p.Kind + "|" + p.ID + "|" + p.Msg }

func depKeys(ps []Problem) map[string]bool {
	m := make(map[string]bool, len(ps))
	for _, p := range ps {
		m[p.key()] = true
	}
	return m
}

// Fsck verifies integrity and returns every problem found, in a stable order.
func (t *Tree) Fsck() []Problem {
	out := slices.Clone(t.loadErr)
	ids := t.sortedIDs()
	for _, id := range t.roots {
		if t.tickets[id] == nil {
			out = append(out, Problem{KindDangling, "ROOT", "lists missing ticket " + id, nil})
		}
	}
	for _, id := range ids {
		tk := t.tickets[id]
		for _, c := range tk.Children {
			if t.tickets[c] == nil {
				out = append(out, Problem{KindDangling, id, "children lists missing ticket " + c, nil})
			}
		}
		for _, b := range tk.BlockedBy {
			if t.tickets[b] == nil {
				out = append(out, Problem{KindDangling, id, "blocked-by lists missing ticket " + b, nil})
			}
		}
	}
	for _, id := range ids {
		if o := t.owners[id]; len(o) > 1 {
			names := make([]string, len(o))
			for i, p := range o {
				names[i] = cmpName(p)
			}
			out = append(out, Problem{KindDuplicate, id, fmt.Sprintf("listed %d times (%s)", len(o), strings.Join(names, ", ")), nil})
		}
	}
	for _, id := range ids {
		if _, ok := t.pos[id]; !ok {
			out = append(out, Problem{KindOrphan, id, "not reachable from ROOT", nil})
		}
	}
	out = append(out, t.depProblems()...)
	for _, id := range ids {
		if st := t.tickets[id].Status; (st == store.StatusClosed || st == store.StatusInProgress) && t.hasUnclosed(t.Descendants(id)) {
			out = append(out, Problem{KindStatus, id, string(st) + " but has descendants that are not closed", nil})
		}
	}
	return out
}

func cmpName(owner string) string {
	if owner == "" {
		return "ROOT"
	}
	return owner
}

// depProblems reports dependency rule violations: self and ancestor/descendant
// dependencies, and cycles over blocked-by plus parent links.
func (t *Tree) depProblems() []Problem {
	var out []Problem
	ids := t.sortedIDs()
	for _, id := range ids {
		for _, b := range t.tickets[id].BlockedBy {
			switch {
			case t.tickets[b] == nil:
			case b == id:
				out = append(out, Problem{KindDep, id, "depends on itself", ErrSelfDep})
			case t.isAncestor(b, id):
				out = append(out, Problem{KindDep, id, "depends on its ancestor " + b, ErrRelatedDep})
			case t.isAncestor(id, b):
				out = append(out, Problem{KindDep, id, "depends on its descendant " + b, ErrRelatedDep})
			}
		}
	}
	return append(out, t.cycles(ids)...)
}

func (t *Tree) cycles(ids []string) []Problem {
	const grey, black = 1, 2
	colour := map[string]int{}
	seen := map[string]bool{}
	var out []Problem
	var stack []string
	var visit func(string)
	visit = func(n string) {
		colour[n] = grey
		stack = append(stack, n)
		for _, m := range t.waits(n) {
			switch colour[m] {
			case 0:
				visit(m)
			case grey:
				cyc := slices.Clone(stack[slices.Index(stack, m):])
				min := slices.Index(cyc, slices.Min(cyc))
				cyc = append(cyc[min:], cyc[:min]...)
				key := slices.Clone(cyc)
				sort.Strings(key)
				if k := strings.Join(key, ","); !seen[k] {
					seen[k] = true
					out = append(out, Problem{KindCycle, cyc[0], "cycle: " + strings.Join(append(cyc, m), " -> "), ErrDepCycle})
				}
			}
		}
		stack = stack[:len(stack)-1]
		colour[n] = black
	}
	for _, id := range ids {
		if colour[id] == 0 {
			visit(id)
		}
	}
	return out
}
