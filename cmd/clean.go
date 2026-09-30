package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
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

Links are governed by --links:

  --links=ignore  (default)  Links are informational; a link never blocks
                             deletion, even to a ticket that survives.
  --links=block              A link to a ticket that is not itself being
                             deleted blocks deletion (the historical behavior).

Dependants and children are always hard blockers, unaffected by --links. A
link to an ID that does not exist (a dangling link) follows the active policy:
ignored under --links=ignore and blocking under --links=block. clean never
rewrites dangling references; use "tk prune" to remove them.

Deletability is transitive: closed tickets that are only referenced (via
dependants or children, or via links under --links=block) by other closed
tickets being deleted in the same run are removed together, so one run clears
an entire chain of closed dependencies.

In dry-run mode, blocked tickets are grouped under the surviving non-candidate
anchor (an open, in-progress, or missing ticket) responsible for the block, so
a large cascade can be traced to the few references keeping it alive.

With --json the result is emitted as JSON and nothing else. The dry-run schema
is:

  {
    "closed":    <int>,   // total closed tickets considered
    "deletable": <int>,   // closed tickets safe to delete
    "blocked":   <int>,   // closed tickets kept in place
    "direct":    <int>,   // blocked tickets whose own edge points at an anchor
    "transitive":<int>,   // blocked only via a referenced ticket that was itself blocked
    "anchors": [          // surviving non-candidates holding tickets back
      {"id": <string>, "status": <string>, "blocked_count": <int>}
    ],
    "tickets": [          // one entry per closed ticket
      {
        "id": <string>,
        "status": <string>,
        "deletable": <bool>,
        "reason": <string>,     // "dependant", "child", or "link"; empty if deletable
                                // "link" appears only under --links=block
        "blocked_by": <string>, // ID of the direct blocking reference; empty if deletable
        "anchor": <string>      // root non-candidate ID; empty if deletable
      }
    ]
  }

With --json --fix the deletion results are emitted instead:

  {
    "deleted": <int>,   // tickets actually removed
    "skipped": <int>,   // blocked tickets left in place
    "errors":  <int>,   // deletions that failed
    "results": [
      {"id": <string>, "deleted": <bool>, "error": <string>} // error omitted when empty
    ]
  }

Anchors are ordered by blocked_count (descending), then id. Entries are ordered
by ticket id.

With --components the ticket graph is shown grouped by connected component
instead of the deletion plan. Components are connected by links (treated as
symmetric); tickets with no links are isolated singletons. Each component is
listed by size descending with its open and closed counts and whether it is
anchored (contains a non-closed ticket) or deletable. --components takes
precedence over --fix: it reports and never deletes. Combined with --json it
emits {"components": [...]}.`, Args: cobra.NoArgs,
	RunE: runClean,
}

var cleanFix bool
var cleanVerbose bool
var cleanJSON bool
var cleanLinks string
var cleanComponents bool

// cleanBlockedCap is the maximum number of blocked tickets listed in the
// human dry-run output before truncating. --verbose lifts the cap.
const cleanBlockedCap = 20

func init() {
	rootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().BoolVar(&cleanFix, "fix", false,
		"Actually delete closed tickets (default is dry-run)")
	cleanCmd.Flags().BoolVarP(&cleanVerbose, "verbose", "v", false,
		"Show the full blocked ticket list instead of the first 20")
	cleanCmd.Flags().BoolVar(&cleanJSON, "json", false,
		"Emit machine-readable JSON instead of human-readable text")
	cleanCmd.Flags().StringVar(&cleanLinks, "links", "ignore",
		"Whether links block deletion: ignore or block")
	cleanCmd.Flags().BoolVar(&cleanComponents, "components", false,
		"Group tickets by connected component instead of showing the deletion plan")
}

// linkPolicy records whether `tk clean` treats links as deletion blockers.
type linkPolicy int

const (
	// linkPolicyIgnore is the default: links are informational and never block
	// deletion.
	linkPolicyIgnore linkPolicy = iota
	// linkPolicyBlock treats a link to a ticket that is not itself being
	// deleted as a blocker, the historical behavior of `tk clean`.
	linkPolicyBlock
)

// parseLinkPolicy maps the --links flag value to a linkPolicy.
func parseLinkPolicy(value string) (linkPolicy, error) {
	switch value {
	case "ignore":
		return linkPolicyIgnore, nil
	case "block":
		return linkPolicyBlock, nil
	default:
		return linkPolicyIgnore, fmt.Errorf("invalid --links value %q: must be \"ignore\" or \"block\"", value)
	}
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
	// blockedBy is the ID of the ticket whose direct reference demoted this
	// ticket. Empty for deletable tickets.
	blockedBy string
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
// and shrink that set to a fixed point. Links participate in this fixed point
// only under linkPolicyBlock.
func deletionPlan(allTickets []*ticket.Ticket, policy linkPolicy) []cleanableTicket {
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

	// Every closed ticket starts as a candidate; dependants and children (and
	// links under linkPolicyBlock) can all demote it during the fixed-point
	// loop below.
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
			if edge, blocked := blockingReason(t, dependants, children, byID, deletable, policy); blocked {
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
			ct.blockedBy = blockedBy[t.ID].blockerID
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
// run. Dependants and children are always treated transitively: a reference
// only blocks deletion if the referencing ticket is not itself deletable in
// this run. Links are treated the same way only under linkPolicyBlock; under
// linkPolicyIgnore they are skipped entirely.
func blockingReason(t *ticket.Ticket, dependants, children map[string][]*ticket.Ticket, byID map[string]*ticket.Ticket, deletable map[string]bool, policy linkPolicy) (blockEdge, bool) {
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

	if policy == linkPolicyBlock {
		for _, linkID := range t.Links {
			linked, ok := byID[linkID]
			if !ok {
				return blockEdge{blockerID: linkID, relation: relationLinkMissing}, true
			}
			if !deletable[linked.ID] {
				return blockEdge{blockerID: linked.ID, relation: relationLink}, true
			}
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

// cleanComponent summarizes one connected component of the ticket link graph.
type cleanComponent struct {
	// members is every ticket ID in the component, sorted ascending.
	members []string
	// open is the number of non-closed tickets (open or in_progress).
	open int
	// closed is the number of closed tickets.
	closed int
	// openIDs lists the non-closed ticket IDs, sorted ascending, so an
	// anchored component can name what is holding it in place.
	openIDs []string
}

// size is the number of tickets in the component.
func (c cleanComponent) size() int { return len(c.members) }

// anchored reports whether the component contains a non-closed ticket. An
// anchored component's closed tickets share the graph with that survivor.
func (c cleanComponent) anchored() bool { return c.open > 0 }

// status labels the component for the listing: "n/a" when there is nothing to
// clean, "anchored" when a non-closed ticket is present, else "deletable".
func (c cleanComponent) status() string {
	switch {
	case c.closed == 0:
		return "n/a"
	case c.anchored():
		return "anchored"
	default:
		return "deletable"
	}
}

// linkComponents builds the connected components of the symmetric link graph.
// Links are undirected, so an edge exists between two tickets whether it is
// recorded on one side or both. Links to IDs that do not exist (dangling
// links) are skipped and self-links are no-ops; neither can panic. A ticket
// with no links is a singleton component.
func linkComponents(allTickets []*ticket.Ticket) []cleanComponent {
	byID := make(map[string]*ticket.Ticket, len(allTickets))
	parent := make(map[string]string, len(allTickets))
	for _, t := range allTickets {
		byID[t.ID] = t
		if _, ok := parent[t.ID]; !ok {
			parent[t.ID] = t.ID
		}
	}

	var find func(string) string
	find = func(id string) string {
		for parent[id] != id {
			parent[id] = parent[parent[id]] // path halving
			id = parent[id]
		}
		return id
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}

	for _, t := range allTickets {
		for _, linkID := range t.Links {
			if _, ok := byID[linkID]; !ok {
				continue
			}
			union(t.ID, linkID)
		}
	}

	groups := make(map[string][]string)
	for id := range parent {
		root := find(id)
		groups[root] = append(groups[root], id)
	}

	components := make([]cleanComponent, 0, len(groups))
	for _, members := range groups {
		sort.Strings(members)
		c := cleanComponent{members: members}
		for _, id := range members {
			if byID[id].Status == ticket.StatusClosed {
				c.closed++
			} else {
				c.open++
				c.openIDs = append(c.openIDs, id)
			}
		}
		components = append(components, c)
	}

	// Largest components first; ties break on the smallest member ID so the
	// listing is deterministic.
	sort.Slice(components, func(i, j int) bool {
		if components[i].size() != components[j].size() {
			return components[i].size() > components[j].size()
		}
		return components[i].members[0] < components[j].members[0]
	})
	return components
}

// printComponents renders the connected-component view of the ticket graph.
func printComponents(components []cleanComponent) {
	if len(components) == 0 {
		fmt.Println("No tickets found.")
		return
	}

	idxWidth := len(strconv.Itoa(len(components)))
	sizeWidth, openWidth, closedWidth := len("size"), len("open"), len("closed")
	for _, c := range components {
		if w := len(strconv.Itoa(c.size())); w > sizeWidth {
			sizeWidth = w
		}
		if w := len(strconv.Itoa(c.open)); w > openWidth {
			openWidth = w
		}
		if w := len(strconv.Itoa(c.closed)); w > closedWidth {
			closedWidth = w
		}
	}

	fmt.Println("Components:")
	fmt.Printf("%-*s  %-*s  %-*s  %-*s  %s\n",
		idxWidth, "#", sizeWidth, "size", openWidth, "open", closedWidth, "closed", "status")
	for i, c := range components {
		status := c.status()
		if c.anchored() && c.closed > 0 {
			status += " (open: " + strings.Join(c.openIDs, ", ") + ")"
		}
		fmt.Printf("%-*d  %-*d  %-*d  %-*d  %s\n",
			idxWidth, i+1, sizeWidth, c.size(), openWidth, c.open, closedWidth, c.closed, status)
	}
}

// cleanComponentJSON is one connected component in `tk clean --components
// --json`. OpenIDs lists the component's non-closed tickets.
type cleanComponentJSON struct {
	Size    int      `json:"size"`
	Open    int      `json:"open"`
	Closed  int      `json:"closed"`
	Status  string   `json:"status"`
	OpenIDs []string `json:"open_ids"`
}

// cleanComponentsJSON is the documented top-level schema of `tk clean
// --components --json`.
type cleanComponentsJSON struct {
	Components []cleanComponentJSON `json:"components"`
}

// printComponentsJSON emits the component view as JSON and nothing else.
func printComponentsJSON(components []cleanComponent) error {
	out := cleanComponentsJSON{Components: make([]cleanComponentJSON, 0, len(components))}
	for _, c := range components {
		out.Components = append(out.Components, cleanComponentJSON{
			Size:    c.size(),
			Open:    c.open,
			Closed:  c.closed,
			Status:  c.status(),
			OpenIDs: append([]string{}, c.openIDs...),
		})
	}
	return writeCleanJSON(out)
}

// cleanPlanJSON is the documented top-level schema of `tk clean --json` in
// dry-run mode. Counts describe the closed-ticket population; Anchors and
// Tickets are emitted in deterministic order.
type cleanPlanJSON struct {
	Closed     int               `json:"closed"`
	Deletable  int               `json:"deletable"`
	Blocked    int               `json:"blocked"`
	Direct     int               `json:"direct"`
	Transitive int               `json:"transitive"`
	Anchors    []cleanAnchorJSON `json:"anchors"`
	Tickets    []cleanTicketJSON `json:"tickets"`
}

// cleanAnchorJSON is one surviving non-candidate and the number of blocked
// tickets attributed to it. Status is the ticket status or "missing".
type cleanAnchorJSON struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	BlockedCount int    `json:"blocked_count"`
}

// cleanTicketJSON is one closed ticket. Reason, BlockedBy, and Anchor are
// populated only for blocked tickets; deletable entries leave them empty.
type cleanTicketJSON struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Deletable bool   `json:"deletable"`
	Reason    string `json:"reason"`
	BlockedBy string `json:"blocked_by"`
	Anchor    string `json:"anchor"`
}

// cleanFixJSON is the documented top-level schema of `tk clean --json --fix`.
type cleanFixJSON struct {
	Deleted int               `json:"deleted"`
	Skipped int               `json:"skipped"`
	Errors  int               `json:"errors"`
	Results []cleanResultJSON `json:"results"`
}

// cleanResultJSON is one attempted deletion. Error is omitted when the
// deletion succeeded.
type cleanResultJSON struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
	Error   string `json:"error,omitempty"`
}

// writeCleanJSON pretty-prints v as the sole output of a --json invocation.
func writeCleanJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling clean JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// printCleanPlanJSON emits the dry-run plan. It writes no prose.
func printCleanPlanJSON(cleanable, blocked []cleanableTicket, anchors []cleanAnchor) error {
	plan := cleanPlanJSON{
		Closed:    len(cleanable),
		Deletable: len(cleanable) - len(blocked),
		Blocked:   len(blocked),
		Anchors:   make([]cleanAnchorJSON, 0, len(anchors)),
		Tickets:   make([]cleanTicketJSON, 0, len(cleanable)),
	}
	for _, a := range anchors {
		plan.Anchors = append(plan.Anchors, cleanAnchorJSON{
			ID:           a.id,
			Status:       a.status,
			BlockedCount: a.count,
		})
	}
	for _, ct := range cleanable {
		entry := cleanTicketJSON{
			ID:        ct.ticket.ID,
			Status:    string(ct.ticket.Status),
			Deletable: !ct.blocked,
		}
		if ct.blocked {
			entry.Reason = normalizeRelation(ct.relation)
			entry.BlockedBy = ct.blockedBy
			entry.Anchor = ct.anchor
			if ct.direct {
				plan.Direct++
			}
		}
		plan.Tickets = append(plan.Tickets, entry)
	}
	plan.Transitive = plan.Blocked - plan.Direct
	return writeCleanJSON(plan)
}

// printCleanFixJSON deletes the deletable tickets and emits the per-ticket
// results instead of the plan. It writes no prose.
func printCleanFixJSON(deletable, blocked []cleanableTicket) error {
	result := cleanFixJSON{
		Skipped: len(blocked),
		Results: make([]cleanResultJSON, 0, len(deletable)),
	}
	for _, ct := range deletable {
		entry := cleanResultJSON{ID: ct.ticket.ID}
		if err := store.Delete(ct.ticket.ID); err != nil {
			entry.Error = err.Error()
			result.Errors++
		} else {
			entry.Deleted = true
			result.Deleted++
		}
		result.Results = append(result.Results, entry)
	}
	return writeCleanJSON(result)
}

func runClean(cmd *cobra.Command, args []string) error {
	// 1. Load all tickets
	allTickets, err := store.List()
	if err != nil {
		return err
	}

	// 2. Resolve the link policy, then classify closed tickets as deletable or
	// blocked
	policy, err := parseLinkPolicy(cleanLinks)
	if err != nil {
		return err
	}
	cleanable := deletionPlan(allTickets, policy)

	// 3. Component view is a report only: it takes precedence over the
	// deletion plan and --fix.
	if cleanComponents {
		components := linkComponents(allTickets)
		if cleanJSON {
			return printComponentsJSON(components)
		}
		printComponents(components)
		return nil
	}

	// 4. Separate into deletable and blocked lists
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

	// 5. Handle dry-run (default)
	if !cleanFix {
		if cleanJSON {
			return printCleanPlanJSON(cleanable, blocked, anchors)
		}

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

	// 6. Handle --fix mode (actual deletion)
	if cleanJSON {
		return printCleanFixJSON(deletable, blocked)
	}

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
