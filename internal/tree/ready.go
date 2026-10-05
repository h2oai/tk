package tree

import (
	"fmt"

	"github.com/h2oai/tk/internal/store"
)

// Outcome says how a Ready query ended.
type Outcome int

const (
	// Found means Result.Ticket is ready to work on.
	Found Outcome = iota
	// Blocked means the top leaf is blocked; Result.Block describes why.
	Blocked
	// NothingLeft means no unclosed work remains.
	NothingLeft
)

// Block describes why the top leaf cannot be worked on.
type Block struct {
	Holder     *store.Ticket // ticket that declares the blocker (Ticket or an ancestor)
	Blocker    *store.Ticket
	BlockerPos string
}

// Result is the answer to Ready. Ticket is set for Found and Blocked.
type Result struct {
	Outcome Outcome
	Ticket  *store.Ticket
	Block   *Block
}

// Ready returns the highest leaf, optionally scoped to the subtree of scope
// (a full id). A blocked top leaf stops the search; it never skips ahead.
func (t *Tree) Ready(scope string) (Result, error) {
	ids := t.order
	if scope != "" {
		if _, err := t.mustExist(scope); err != nil {
			return Result{}, err
		}
		ids = t.Subtree(scope)
	}
	var top string
	for _, id := range ids {
		if !t.actionable(id) {
			continue
		}
		if t.tickets[id].Status == store.StatusInProgress {
			top = id
			break
		}
		if top == "" {
			top = id
		}
	}
	if top == "" {
		return Result{Outcome: NothingLeft}, nil
	}
	tk := t.tickets[top]
	if b := t.blockedBy(top); b != nil {
		return Result{Outcome: Blocked, Ticket: tk, Block: b}, nil
	}
	return Result{Outcome: Found, Ticket: tk}, nil
}

// actionable reports whether id is unclosed and either a leaf or a parent
// whose descendants are all closed.
func (t *Tree) actionable(id string) bool {
	return t.tickets[id].Status != store.StatusClosed && !t.hasUnclosed(t.Descendants(id))
}

// blockedBy returns the first unclosed blocker of id or its ancestors.
func (t *Tree) blockedBy(id string) *Block {
	for _, h := range append([]string{id}, t.Ancestors(id)...) {
		for _, b := range t.tickets[h].BlockedBy {
			if bt := t.tickets[b]; bt != nil && bt.Status != store.StatusClosed {
				return &Block{Holder: t.tickets[h], Blocker: bt, BlockerPos: t.Position(b)}
			}
		}
	}
	return nil
}

// Describe formats a block as `fanir7 "Add parser" waits on lovet2 "Pick schema" (2.4.1, open)`.
func (b *Block) Describe(blocked *store.Ticket) string {
	return fmt.Sprintf("%s %q waits on %s %q (%s, %s)", blocked.ID, blocked.Title,
		b.Blocker.ID, b.Blocker.Title, b.BlockerPos, b.Blocker.Status)
}
