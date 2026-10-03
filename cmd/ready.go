package cmd

import (
	"fmt"

	"github.com/h2oai/tk/internal/ticket"
	"github.com/spf13/cobra"
)

var readyCmd = &cobra.Command{
	Use:   "ready [ticket-id]",
	Short: "List ready tickets",
	Long: `List open/in-progress tickets with all dependencies resolved.

With no arguments, lists every ready ticket. With a ticket id, lists only
ready tickets whose parent is that ticket, which is useful for finding the
next available work inside an epic.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runReady,
}

var readySort string

func init() {
	rootCmd.AddCommand(readyCmd)
	readyCmd.Flags().StringVar(&readySort, "sort", "priority", "Sort by field (date|priority|status)")
}

func runReady(cmd *cobra.Command, args []string) error {
	tickets, err := store.List()
	if err != nil {
		return err
	}

	// Build the status map from all tickets so dependency resolution stays
	// global even when the listing is scoped to one ticket's children.
	statusMap := make(map[string]ticket.Status)
	for _, t := range tickets {
		statusMap[t.ID] = t.Status
	}

	// Scope to children of the given ticket when an id is supplied.
	if len(args) == 1 {
		target, err := store.Get(args[0])
		if err != nil {
			return err
		}

		children := make([]*ticket.Ticket, 0, len(tickets))
		for _, t := range tickets {
			if t.Parent == target.ID {
				children = append(children, t)
			}
		}
		tickets = children
	}

	if err := ticket.SortBy(tickets, readySort); err != nil {
		return err
	}

	// Filter ready tickets (preserves sort order)
	var ready []*ticket.Ticket
	for _, t := range tickets {
		// Must be open or in_progress
		if t.Status != ticket.StatusOpen && t.Status != ticket.StatusInProgress {
			continue
		}

		// All deps must be closed
		allDepsResolved := true
		for _, dep := range t.Deps {
			if statusMap[dep] != ticket.StatusClosed {
				allDepsResolved = false
				break
			}
		}

		if allDepsResolved {
			ready = append(ready, t)
		}
	}

	// Print
	for _, t := range ready {
		fmt.Printf("%-8s [P%d][%s] - %s\n", t.ID, t.Priority, t.Status, t.Title)
	}

	return nil
}
