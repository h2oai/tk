// Package tree holds the domain logic for tk: the ordered ticket tree, ready
// selection, status transitions, mutations, dependencies and fsck.
package tree

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/h2oai/tk/internal/store"
)

// Sentinel errors that callers can test with errors.Is.
var (
	ErrExists          = errors.New("ticket already exists")
	ErrOpenDescendants = errors.New("has descendants that are not closed")
	ErrNotLeaf         = errors.New("has descendants that are not closed; start one of them")
	ErrHasChildren     = errors.New("has children")
	ErrIsBlocker       = errors.New("blocks other tickets")
	ErrCycle           = errors.New("would make a ticket its own ancestor")
	ErrBadPlace        = errors.New("invalid position")
	ErrSelfDep         = errors.New("a ticket cannot depend on itself")
	ErrRelatedDep      = errors.New("a ticket cannot depend on its own ancestor or descendant")
	ErrDepCycle        = errors.New("dependency cycle")
	ErrCorrupt         = errors.New("tickets are unreadable or corrupt")
	ErrNotClosed       = errors.New("has tickets that are not closed")
	ErrCrossDep        = errors.New("blocked-by crosses the subtree boundary")
)

// Tree is an in-memory view of a store: every ticket, the ordered roots and
// the derived parent map. Mutating methods persist through the store.
type Tree struct {
	st      *store.Store
	tickets map[string]*store.Ticket
	roots   []string
	parent  map[string]string   // child -> parent, roots absent
	owners  map[string][]string // child -> every list naming it ("" is ROOT)
	order   []string            // DFS preorder over reachable tickets
	pos     map[string][]int
	loadErr []Problem

	dirty      []string
	rootsDirty bool
	gone       []string
	archived   []string
}

// Load reads every ticket and ROOT.md. Unreadable files and integrity problems
// do not fail the load; Fsck reports them.
func Load(st *store.Store) (*Tree, error) {
	t := &Tree{st: st}
	if err := t.reload(); err != nil {
		return nil, err
	}
	return t, nil
}

// Reload discards the in-memory state and re-reads everything from disk, for
// callers that hold the store lock and may be looking at stale data.
func (t *Tree) Reload() error { return t.reload() }

func (t *Tree) reload() error {
	ids, err := t.st.IDs()
	if err != nil {
		return err
	}
	t.tickets = make(map[string]*store.Ticket, len(ids))
	t.loadErr = nil
	for _, id := range ids {
		tk, err := t.st.Load(id)
		if err != nil {
			t.loadErr = append(t.loadErr, Problem{Kind: KindUnreadable, ID: id, Msg: err.Error()})
			continue
		}
		t.tickets[id] = tk
	}
	t.roots, err = t.st.LoadRoots()
	if err != nil {
		t.roots = nil
		t.loadErr = append(t.loadErr, Problem{Kind: KindUnreadable, ID: "ROOT", Msg: err.Error()})
	}
	t.dirty, t.rootsDirty, t.gone, t.archived = nil, false, nil, nil
	t.reindex()
	return nil
}

// reindex rebuilds the parent map, ownership and DFS order from memory.
func (t *Tree) reindex() {
	t.parent = map[string]string{}
	t.owners = map[string][]string{}
	for _, id := range t.roots {
		t.owners[id] = append(t.owners[id], "")
	}
	for _, id := range t.sortedIDs() {
		for _, c := range t.tickets[id].Children {
			t.owners[c] = append(t.owners[c], id)
			if _, claimed := t.parent[c]; !claimed && !slices.Contains(t.roots, c) {
				t.parent[c] = id
			}
		}
	}
	t.order = nil
	t.pos = map[string][]int{}
	var visit func(ids []string, prefix []int)
	visit = func(ids []string, prefix []int) {
		n := 0
		for _, id := range ids {
			if _, ok := t.tickets[id]; !ok {
				continue
			}
			if _, seen := t.pos[id]; seen {
				continue
			}
			n++
			p := append(slices.Clone(prefix), n)
			t.pos[id] = p
			t.order = append(t.order, id)
			visit(t.tickets[id].Children, p)
		}
	}
	visit(t.roots, nil)
}

func (t *Tree) sortedIDs() []string {
	ids := make([]string, 0, len(t.tickets))
	for id := range t.tickets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Get returns the ticket with the exact id, or nil.
func (t *Tree) Get(id string) *store.Ticket { return t.tickets[id] }

// Resolve maps a full or partial id to a full id.
func (t *Tree) Resolve(partial string) (string, error) {
	return store.ResolveID(t.sortedIDs(), partial)
}

// Roots returns the ordered root ids.
func (t *Tree) Roots() []string { return slices.Clone(t.roots) }

// Orphans returns the ids of tickets not reachable from ROOT, sorted.
func (t *Tree) Orphans() []string {
	var out []string
	for _, id := range t.sortedIDs() {
		if _, ok := t.pos[id]; !ok {
			out = append(out, id)
		}
	}
	return out
}

// Order returns all reachable ticket ids in DFS preorder.
func (t *Tree) Order() []string { return slices.Clone(t.order) }

// Parent returns the parent id, or "" for roots and unknown tickets.
func (t *Tree) Parent(id string) string { return t.parent[id] }

// Children returns the existing children of id in order.
func (t *Tree) Children(id string) []string {
	tk := t.tickets[id]
	if tk == nil {
		return nil
	}
	var out []string
	for _, c := range tk.Children {
		if t.tickets[c] != nil {
			out = append(out, c)
		}
	}
	return out
}

// IsLeaf reports whether id has no children.
func (t *Tree) IsLeaf(id string) bool {
	tk := t.tickets[id]
	return tk != nil && len(tk.Children) == 0
}

// Position formats the position of id as "2.1.3", or "" if unreachable.
func (t *Tree) Position(id string) string {
	p := t.pos[id]
	parts := make([]string, len(p))
	for i, n := range p {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ".")
}

// Ancestors returns the ancestors of id, nearest first.
func (t *Tree) Ancestors(id string) []string {
	var out []string
	seen := map[string]bool{id: true}
	for p := t.parent[id]; p != "" && !seen[p]; p = t.parent[p] {
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// Subtree returns id followed by its descendants in DFS preorder.
func (t *Tree) Subtree(id string) []string {
	if t.tickets[id] == nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	var walk func(string)
	walk = func(n string) {
		if seen[n] || t.tickets[n] == nil {
			return
		}
		seen[n] = true
		out = append(out, n)
		for _, c := range t.tickets[n].Children {
			walk(c)
		}
	}
	walk(id)
	return out
}

// Descendants returns the descendants of id in DFS preorder, excluding id.
func (t *Tree) Descendants(id string) []string {
	s := t.Subtree(id)
	if len(s) == 0 {
		return nil
	}
	return s[1:]
}

func (t *Tree) isAncestor(anc, id string) bool { return slices.Contains(t.Ancestors(id), anc) }

func (t *Tree) hasUnclosed(ids []string) bool {
	for _, id := range ids {
		if t.tickets[id].Status != store.StatusClosed {
			return true
		}
	}
	return false
}

func (t *Tree) mustExist(id string) (*store.Ticket, error) {
	tk := t.tickets[id]
	if tk == nil {
		return nil, fmt.Errorf("%s: %w", id, store.ErrNotFound)
	}
	return tk, nil
}

// siblings returns the raw ordered list that names the children of parent
// ("" is ROOT).
func (t *Tree) siblings(parent string) []string {
	if parent == "" {
		return t.roots
	}
	return t.tickets[parent].Children
}

func (t *Tree) setSiblings(parent string, list []string) {
	if parent == "" {
		t.roots, t.rootsDirty = list, true
		return
	}
	t.tickets[parent].Children = list
	t.touch(parent)
}

func (t *Tree) touch(id string) {
	if !slices.Contains(t.dirty, id) {
		t.dirty = append(t.dirty, id)
	}
}

// apply runs a mutation in memory, persists what it touched and rolls the
// in-memory state back to the store's on any failure. It refuses to run at
// all while any ticket or ROOT.md failed to load, so that an unreadable file
// is never overwritten by a view that treats it as empty.
func (t *Tree) apply(fn func() error) error {
	if len(t.loadErr) > 0 {
		return fmt.Errorf("%w (%s); refusing to modify, run `tk fsck`", ErrCorrupt, t.loadErr[0])
	}
	err := fn()
	if err == nil {
		err = t.commit()
	}
	if err != nil {
		_ = t.reload()
		return err
	}
	for _, id := range append(t.gone, t.archived...) {
		delete(t.tickets, id)
	}
	t.dirty, t.rootsDirty, t.gone, t.archived = nil, false, nil, nil
	t.reindex()
	return nil
}

// commit saves dirty tickets and ROOT.md before deleting or archiving files,
// so a crash midway leaves orphaned files (reported by fsck) rather than
// lost ones.
func (t *Tree) commit() error {
	for _, id := range t.dirty {
		if err := t.st.Save(t.tickets[id]); err != nil {
			return err
		}
	}
	if t.rootsDirty {
		if err := t.st.SaveRoots(t.roots); err != nil {
			return err
		}
	}
	for _, id := range t.gone {
		if err := t.st.Delete(id); err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
	}
	for _, id := range t.archived {
		if _, err := t.st.Archive(id); err != nil {
			return err
		}
	}
	return nil
}
