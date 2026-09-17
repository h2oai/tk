package cmd

import (
	"fmt"

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
  - Has bidirectional links

Deletability is transitive: closed tickets that are only referenced by other
closed tickets being deleted in the same run are removed together, so one run
clears an entire chain of closed dependencies.`,
	Args: cobra.NoArgs,
	RunE: runClean,
}

var cleanFix bool

func init() {
	rootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().BoolVar(&cleanFix, "fix", false,
		"Actually delete closed tickets (default is dry-run)")
}

type cleanableTicket struct {
	ticket  *ticket.Ticket
	blocked bool
	reason  string
}

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
	for _, t := range allTickets {
		for _, depID := range t.Deps {
			if depID != t.ID {
				dependants[depID] = append(dependants[depID], t)
			}
		}
		if t.Parent != "" {
			children[t.Parent] = append(children[t.Parent], t)
		}
	}

	// Every closed ticket starts as a candidate, except those with links,
	// which always block deletion.
	deletable := make(map[string]bool)
	reasons := make(map[string]string)
	var closed []*ticket.Ticket
	for _, t := range allTickets {
		if t.Status != ticket.StatusClosed {
			continue
		}
		closed = append(closed, t)
		if len(t.Links) > 0 {
			reasons[t.ID] = "has links"
			continue
		}
		deletable[t.ID] = true
	}

	// Drop candidates referenced by a ticket that survives this run, repeating
	// until the set stops shrinking. Candidates that only reference each other
	// (including dependency cycles) survive the loop and are deleted together.
	for changed := true; changed; {
		changed = false
		for _, t := range closed {
			if !deletable[t.ID] {
				continue
			}
			if reason, blocked := blockingReason(t.ID, dependants, children, deletable); blocked {
				delete(deletable, t.ID)
				reasons[t.ID] = reason
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
		}
		plan = append(plan, ct)
	}
	return plan
}

// blockingReason reports why a candidate cannot be deleted, given the set of
// tickets currently expected to be deleted in this run.
func blockingReason(id string, dependants, children map[string][]*ticket.Ticket, deletable map[string]bool) (string, bool) {
	for _, d := range dependants[id] {
		if !deletable[d.ID] {
			return "has dependants", true
		}
	}

	hasBlockedChild := false
	for _, c := range children[id] {
		if deletable[c.ID] {
			continue
		}
		if c.Status != ticket.StatusClosed {
			return "has non-closed children", true
		}
		hasBlockedChild = true
	}
	if hasBlockedChild {
		return "has blocked children", true
	}

	return "", false
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
		fmt.Printf("  %d blocked\n", numBlocked)

		if numBlocked > 0 {
			fmt.Println("\nBlocked tickets:")
			for _, ct := range blocked {
				fmt.Printf("  %s [%s] %s - %s\n", ct.ticket.ID, ct.ticket.Status, ct.ticket.Title, ct.reason)
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
