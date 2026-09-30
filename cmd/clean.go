package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lo5/tk/internal/ticket"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Delete all closed tickets",
	Long: `Delete all closed tickets that are safe to remove.

By default, performs a dry-run showing what would be deleted.
Use --fix to actually delete the tickets.

Refuses deletion if a closed ticket:
  - Has dependants that are not themselves being deleted
  - Has children that are not themselves being deleted
  - Has links to a ticket that is not itself being deleted

Deletability is transitive: closed tickets that are only referenced (via
dependants, children, or links) by other closed tickets being deleted in the
same run are removed together, so one run clears an entire chain of closed
dependencies.

In dry-run mode, blocked tickets are grouped under the surviving non-candidate
anchor (an open, in-progress, or missing ticket) responsible for the block, so
a large cascade can be traced to the few references keeping it alive.`,
	Args: cobra.NoArgs,
	RunE: runClean,
}

var cleanFix bool
var cleanVerbose bool

// cleanBlockedCap is the maximum number of blocked tickets listed in the
// human dry-run output before truncating. --verbose lifts the cap.
const cleanBlockedCap = 20

func init() {
	rootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().BoolVar(&cleanFix, "fix", false,
		"Actually delete closed tickets (default is dry-run)")
	cleanCmd.Flags().BoolVarP(&cleanVerbose, "verbose", "v", false,
		"Show the full blocked ticket list instead of the first 20")
}

type cleanableTicket struct {
	ticket  *ticket.Ticket
	blocked bool
	reason  string
	// anchor is the non-candidate ticket (open, in-progress, or missing) that
	// transitively causes this ticket to be blocked. Empty only in the
	// defensive case of a blocking-edge cycle.
	anchor string
	// relation is the relation of this ticket's own direct blocking edge.
	relation string
	// direct reports whether this ticket's own blocking edge points at a
	// non-candidate (open, in-progress, or missing). A directly anchored
	// ticket is blocked on its own; a transitive one is demoted only because
	// a ticket it references was itself demoted.
	direct bool
}

// blockEdge records the specific reference that demoted a candidate: the
// referencing ticket and the relation through which it points at the blocked
// ticket. A link to an ID that does not exist is recorded as "link(missing)".
type blockEdge struct {
	blockerID string
	relation  string
}

// blockRelations are the relation names used by blockEdge.relation.
const (
	relationDependant   = "dependant"
	relationChild       = "child"
	relationLink        = "link"
	relationLinkMissing = "link(missing)"
)

// deletionPlan classifies every closed ticket as deletable or blocked.
//
// Blocking is transitive: a closed ticket is safe to delete only if everything
// referencing it is deleted in the same run. Checking each ticket once against
// the unmodified ticket set would peel off a single level of a dependency chain
// per invocation, so instead we start with every closed ticket as a candidate
// and shrink that set to a fixed point.
func deletionPlan(allTickets []*ticket.Ticket) []cleanableTicket {
	// Reverse indices: who points at each ticket.
	dependants := make(map[string][]*ticket.Ticket)
	children := make(map[string][]*ticket.Ticket)
	byID := make(map[string]*ticket.Ticket, len(allTickets))
	for _, t := range allTickets {
		byID[t.ID] = t
		for _, depID := range t.Deps {
			if depID != t.ID {
				dependants[depID] = append(dependants[depID], t)
			}
		}
		if t.Parent != "" {
			children[t.Parent] = append(children[t.Parent], t)
		}
	}

	// Every closed ticket starts as a candidate; dependants, children, and
	// links can all demote it during the fixed-point loop below.
	deletable := make(map[string]bool)
	reasons := make(map[string]string)
	var closed []*ticket.Ticket
	for _, t := range allTickets {
		if t.Status != ticket.StatusClosed {
			continue
		}
		closed = append(closed, t)
		deletable[t.ID] = true
	}

	// closedSet is the set of candidates: every closed ticket, regardless of
	// whether it survives the fixed point. Anything outside it (open,
	// in-progress, or missing) is a non-candidate and therefore an anchor.
	closedSet := make(map[string]bool, len(closed))
	for _, t := range closed {
		closedSet[t.ID] = true
	}

	// Drop candidates referenced by a ticket that survives this run, repeating
	// until the set stops shrinking. Candidates that only reference each other
	// (including dependency cycles and mutual links) survive the loop and are
	// deleted together. Each demotion records the edge that caused it so the
	// block can later be traced back to its root cause.
	blockedBy := make(map[string]blockEdge)
	for changed := true; changed; {
		changed = false
		for _, t := range closed {
			if !deletable[t.ID] {
				continue
			}
			if edge, blocked := blockingReason(t, dependants, children, byID, deletable); blocked {
				delete(deletable, t.ID)
				blockedBy[t.ID] = edge
				reasons[t.ID] = reasonFor(edge, byID)
				changed = true
			}
		}
	}

	plan := make([]cleanableTicket, 0, len(closed))
	for _, t := range closed {
		ct := cleanableTicket{ticket: t}
		if !deletable[t.ID] {
			ct.blocked = true
			ct.reason = reasons[t.ID]
			ct.relation = blockedBy[t.ID].relation
			ct.direct = !closedSet[blockedBy[t.ID].blockerID]
			ct.anchor = findAnchor(t.ID, blockedBy, closedSet)
		}
		plan = append(plan, ct)
	}
	return plan
}

// findAnchor walks the recorded blocking edges from a blocked ticket until it
// reaches a non-candidate: a ticket that is open, in-progress, or missing. That
// non-candidate is the surviving root cause of the block. Because candidates
// are removed monotonically, the edge graph is acyclic and every chain ends at
// an anchor; the visited set only guards against corrupt input.
func findAnchor(id string, blockedBy map[string]blockEdge, closedSet map[string]bool) string {
	visited := map[string]bool{id: true}
	cur := id
	for {
		edge, ok := blockedBy[cur]
		if !ok {
			return ""
		}
		blocker := edge.blockerID
		if !closedSet[blocker] {
			return blocker
		}
		if visited[blocker] {
			return ""
		}
		visited[blocker] = true
		cur = blocker
	}
}

// reasonFor renders a human-readable reason for a recorded blocking edge. The
// blocking relation is what matters; the reason string is retained for the
// per-ticket listing.
func reasonFor(edge blockEdge, byID map[string]*ticket.Ticket) string {
	switch edge.relation {
	case relationDependant:
		return "has dependants"
	case relationChild:
		if b, ok := byID[edge.blockerID]; ok && b.Status == ticket.StatusClosed {
			return "has blocked children"
		}
		return "has non-closed children"
	case relationLink, relationLinkMissing:
		return "has links"
	}
	return edge.relation
}

// blockingReason reports the specific edge that prevents a candidate from being
// deleted, given the set of tickets currently expected to be deleted in this
// run. Dependants, children, and links are all treated transitively: a
// reference only blocks deletion if the referencing or linked ticket is not
// itself deletable in this run.
func blockingReason(t *ticket.Ticket, dependants, children map[string][]*ticket.Ticket, byID map[string]*ticket.Ticket, deletable map[string]bool) (blockEdge, bool) {
	for _, d := range dependants[t.ID] {
		if !deletable[d.ID] {
			return blockEdge{blockerID: d.ID, relation: relationDependant}, true
		}
	}

	for _, c := range children[t.ID] {
		if deletable[c.ID] {
			continue
		}
		return blockEdge{blockerID: c.ID, relation: relationChild}, true
	}

	for _, linkID := range t.Links {
		linked, ok := byID[linkID]
		if !ok {
			return blockEdge{blockerID: linkID, relation: relationLinkMissing}, true
		}
		if !deletable[linked.ID] {
			return blockEdge{blockerID: linked.ID, relation: relationLink}, true
		}
	}

	return blockEdge{}, false
}

// cleanAnchor aggregates the blocked tickets attributed to one non-candidate
// root cause.
type cleanAnchor struct {
	id        string
	status    string
	count     int
	relations map[string]bool
	targets   []string
}

// groupAnchors attributes every blocked ticket to its surviving anchor and
// aggregates the result per anchor. Anchors are ordered by the number of
// blocked tickets they cause (descending), then by ID for stable output.
func groupAnchors(blocked []cleanableTicket, allTickets []*ticket.Ticket) []cleanAnchor {
	byID := make(map[string]*ticket.Ticket, len(allTickets))
	for _, t := range allTickets {
		byID[t.ID] = t
	}

	groups := make(map[string]*cleanAnchor)
	for _, ct := range blocked {
		if ct.anchor == "" {
			// Defensive: a blocking-edge cycle with no non-candidate
			// reference. The fixed point cannot produce this, so skip it
			// rather than invent a bogus anchor.
			continue
		}
		g, ok := groups[ct.anchor]
		if !ok {
			status := "missing"
			if t, found := byID[ct.anchor]; found {
				status = string(t.Status)
			}
			g = &cleanAnchor{
				id:        ct.anchor,
				status:    status,
				relations: make(map[string]bool),
			}
			groups[ct.anchor] = g
		}
		g.count++
		if ct.relation != "" {
			g.relations[normalizeRelation(ct.relation)] = true
		}
		if len(g.targets) < 3 {
			g.targets = append(g.targets, ct.ticket.ID)
		}
	}

	anchors := make([]cleanAnchor, 0, len(groups))
	for _, g := range groups {
		anchors = append(anchors, *g)
	}
	sort.Slice(anchors, func(i, j int) bool {
		if anchors[i].count != anchors[j].count {
			return anchors[i].count > anchors[j].count
		}
		return anchors[i].id < anchors[j].id
	})
	return anchors
}

// normalizeRelation collapses the missing-link marker to the plain "link"
// relation for display; missingness is conveyed by the anchor's [missing]
// status instead.
func normalizeRelation(relation string) string {
	if relation == relationLinkMissing {
		return relationLink
	}
	return relation
}

// relationLabel renders the relations observed under an anchor. A lone blocked
// ticket also names the ticket it blocks, matching the compact anchor line.
func (g cleanAnchor) relationLabel() string {
	rels := make([]string, 0, len(g.relations))
	for r := range g.relations {
		rels = append(rels, r)
	}
	sort.Strings(rels)
	if len(rels) == 0 {
		return ""
	}
	label := strings.Join(rels, "/")
	if g.count == 1 && len(g.targets) == 1 {
		label += " -> " + g.targets[0]
	}
	return label
}

// printAnchors prints the anchor section: each surviving non-candidate, its
// status, how many blocked tickets it holds, and the relations involved.
func printAnchors(anchors []cleanAnchor) {
	if len(anchors) == 0 {
		return
	}

	width := 0
	for _, a := range anchors {
		if len(a.id) > width {
			width = len(a.id)
		}
	}

	fmt.Println("\nAnchors:")
	for _, a := range anchors {
		label := a.relationLabel()
		if label == "" {
			fmt.Printf("  %-*s [%s] blocks %d\n", width, a.id, a.status, a.count)
			continue
		}
		fmt.Printf("  %-*s [%s] blocks %d (%s)\n", width, a.id, a.status, a.count, label)
	}
}

func runClean(cmd *cobra.Command, args []string) error {
	// 1. Load all tickets
	allTickets, err := store.List()
	if err != nil {
		return err
	}

	// 2. Classify closed tickets as deletable or blocked
	cleanable := deletionPlan(allTickets)

	// 3. Separate into deletable and blocked lists
	var deletable []cleanableTicket
	var blocked []cleanableTicket
	for _, ct := range cleanable {
		if ct.blocked {
			blocked = append(blocked, ct)
		} else {
			deletable = append(deletable, ct)
		}
	}

	// Attribute every blocked ticket to the non-candidate anchor responsible
	// for its block. Reporting only: this does not affect deletion semantics.
	anchors := groupAnchors(blocked, allTickets)

	// 4. Handle dry-run (default)
	if !cleanFix {
		totalClosed := len(cleanable)
		numDeletable := len(deletable)
		numBlocked := len(blocked)

		if totalClosed == 0 {
			fmt.Println("No closed tickets found.")
			return nil
		}

		fmt.Printf("Found %d closed ticket(s):\n", totalClosed)
		fmt.Printf("  %d deletable\n", numDeletable)
		if numBlocked > 0 {
			numDirect := 0
			for _, ct := range blocked {
				if ct.direct {
					numDirect++
				}
			}
			fmt.Printf("  %d blocked - %d directly anchored, %d transitively blocked\n",
				numBlocked, numDirect, numBlocked-numDirect)
			fmt.Printf("  %d surviving anchor(s)\n", len(anchors))
		} else {
			fmt.Printf("  %d blocked\n", numBlocked)
		}

		if numBlocked > 0 {
			printAnchors(anchors)

			shown := blocked
			if !cleanVerbose && len(blocked) > cleanBlockedCap {
				shown = blocked[:cleanBlockedCap]
			}

			if len(shown) < numBlocked {
				fmt.Printf("\nBlocked tickets (showing %d of %d):\n", len(shown), numBlocked)
			} else {
				fmt.Println("\nBlocked tickets:")
			}
			for _, ct := range shown {
				fmt.Printf("  %s [%s] %s - %s\n", ct.ticket.ID, ct.ticket.Status, ct.ticket.Title, ct.reason)
			}
			if len(shown) < numBlocked {
				fmt.Printf("  ... and %d more; re-run with --verbose for the full list.\n", numBlocked-len(shown))
			}
		}

		if numDeletable > 0 {
			fmt.Printf("\nRun with --fix to delete %d deletable ticket(s).\n", numDeletable)
		}

		return nil
	}

	// 5. Handle --fix mode (actual deletion)
	if len(deletable) == 0 {
		if len(blocked) > 0 {
			fmt.Printf("No deletable tickets. All %d closed ticket(s) are blocked.\n", len(blocked))
		} else {
			fmt.Println("No closed tickets found.")
		}
		return nil
	}

	fmt.Println("Deleting closed tickets...")
	fmt.Println()

	successCount := 0
	errorCount := 0

	for _, ct := range deletable {
		if err := store.Delete(ct.ticket.ID); err != nil {
			fmt.Printf("Warning: failed to delete %s: %v\n", ct.ticket.ID, err)
			errorCount++
			continue
		}
		fmt.Printf("Deleted: %s\n", ct.ticket.ID)
		successCount++
	}

	fmt.Printf("\nDeleted %d ticket(s)", successCount)
	if len(blocked) > 0 {
		fmt.Printf(", skipped %d blocked ticket(s)", len(blocked))
	}
	if errorCount > 0 {
		fmt.Printf(", %d error(s)", errorCount)
	}
	fmt.Println(".")

	return nil
}
