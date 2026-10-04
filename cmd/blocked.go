package cmd

import (
	"fmt"
	"strings"

	"github.com/h2oai/tk/internal/ticket"
	"github.com/spf13/cobra"
)

var blockedCmd = &cobra.Command{
	Use:   "blocked",
	Short: "List blocked tickets",
	Long: `List open/in-progress tickets with unresolved dependencies.
Shows only the unclosed blockers for each ticket.`,
	RunE: runBlocked,
}

var blockedSort string

func init() {
	rootCmd.AddCommand(blockedCmd)
	blockedCmd.Flags().StringVar(&blockedSort, "sort", "date", "Sort by field (date|priority|status)")
}

type blockedTicket struct {
	ticket   *ticket.Ticket
	blockers []string
}

func runBlocked(cmd *cobra.Command, args []string) error {
	tickets, err := store.List()
	if err != nil {
		return err
	}

	if err := ticket.SortBy(tickets, blockedSort); err != nil {
		return err
	}

	// Build status map
	statusMap := make(map[string]ticket.Status)
	for _, t := range tickets {
		statusMap[t.ID] = t.Status
	}

	// Filter blocked tickets (preserves sort order)
	var blocked []blockedTicket
	for _, t := range tickets {
		// Must be open or in_progress
		if t.Status != ticket.StatusOpen && t.Status != ticket.StatusInProgress {
			continue
		}

		// Must have deps
		if len(t.Deps) == 0 {
			continue
		}

		// Find unclosed blockers
		var blockers []string
		for _, dep := range t.Deps {
			if statusMap[dep] != ticket.StatusClosed {
				blockers = append(blockers, dep)
			}
		}

		if len(blockers) > 0 {
			blocked = append(blocked, blockedTicket{ticket: t, blockers: blockers})
		}
	}

	// Print
	for _, b := range blocked {
		blockersStr := "[" + strings.Join(b.blockers, ", ") + "]"
		fmt.Printf("%s - %s <- %s\n", formatTicketPrefix(b.ticket), b.ticket.Title, blockersStr)
	}

	return nil
}
