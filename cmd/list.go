package cmd

import (
	"fmt"
	"strings"

	"github.com/h2oai/tk/internal/ticket"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "ls [--status=X]",
	Aliases: []string{"list"},
	Short:   "List tickets",
	Long:    "List all tickets, optionally filtered by status.",
	RunE:    runList,
}

var listStatus string
var listSort string

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status (open|in_progress|closed)")
	listCmd.Flags().StringVar(&listSort, "sort", "date", "Sort by field (date|priority|status)")
}

func runList(cmd *cobra.Command, args []string) error {
	tickets, err := store.List()
	if err != nil {
		return err
	}

	// Filter by status if specified
	if listStatus != "" {
		status := ticket.Status(listStatus)
		var filtered []*ticket.Ticket
		for _, t := range tickets {
			if t.Status == status {
				filtered = append(filtered, t)
			}
		}
		tickets = filtered
	}

	if err := ticket.SortBy(tickets, listSort); err != nil {
		return err
	}

	for _, t := range tickets {
		depStr := ""
		if len(t.Deps) > 0 {
			depStr = " <- [" + strings.Join(t.Deps, ", ") + "]"
		}
		fmt.Printf("%s - %s%s\n", formatTicketPrefix(t), t.Title, depStr)
	}

	return nil
}
