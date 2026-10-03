package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var noteCmd = &cobra.Command{
	Use:   "note <id> [note text]",
	Short: "Append timestamped note to ticket",
	Long: `Append a timestamped note to a ticket.
Note text is passed inline, from stdin with -, or from a file with -F <path>:

  tk note ab12c - <<'EOF'
  Root cause is ` + "`parse()`" + ` -- see $HOME handling.
  EOF`,
	Args: cobra.MinimumNArgs(1),
	RunE: runNote,
}

var noteFile string

func init() {
	rootCmd.AddCommand(noteCmd)

	noteCmd.Flags().StringVarP(&noteFile, "file", "F", "", "Read note from file (- reads stdin)")
}

func runNote(cmd *cobra.Command, args []string) error {
	ticketID := args[0]

	note, err := resolveText(cmd, strings.Join(args[1:], " "), noteFile, "note text")
	if err != nil {
		return err
	}
	if note == "" {
		return fmt.Errorf("no note provided")
	}

	// Check if Notes section exists
	contains, id, err := store.FileContains(ticketID, "## Notes")
	if err != nil {
		return err
	}

	// Build content to append
	timestamp := time.Now().UTC().Format(time.RFC3339)
	var content strings.Builder

	if !contains {
		content.WriteString("\n## Notes\n")
	}
	content.WriteString(fmt.Sprintf("\n**%s**\n\n%s\n", timestamp, note))

	// Append to file
	_, err = store.AppendToFile(id, content.String())
	if err != nil {
		return err
	}

	fmt.Printf("Note added to %s\n", id)
	return nil
}
