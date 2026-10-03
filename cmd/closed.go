package cmd

import (
	"fmt"

	"github.com/h2oai/tk/internal/ticket"
	"github.com/spf13/cobra"
)

var closedCmd = &cobra.Command{
	Use:   "closed [--limit=N]",
	Short: "List recently closed tickets",
	Long: `List recently closed tickets, sorted by modification time (most recent first).
Default limit is 20.`,
	RunE: runClosed,
}

var closedLimit int

func init() {
	rootCmd.AddCommand(closedCmd)
	closedCmd.Flags().IntVar(&closedLimit, "limit", 20, "Maximum number of tickets to show")
}

func runClosed(cmd *cobra.Command, args []string) error {
	// Get tickets sorted by modification time
	tickets, err := store.ListByModTime(100) // Get more than limit to filter
	if err != nil {
		return err
	}

	// Filter for closed/done status and apply limit
	count := 0
	for _, t := range tickets {
		if count >= closedLimit {
			break
		}
		if t.Status == ticket.StatusClosed || t.Status == "done" {
			fmt.Printf("%-8s [%s] - %s\n", t.ID, t.Status, t.Title)
			count++
		}
	}

	return nil
}
