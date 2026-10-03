package cmd

import (
	"os"

	"github.com/h2oai/tk/internal/ticket"
	"github.com/spf13/cobra"
)

var (
	ticketsDir string
	store      *ticket.FileStore
)

var rootCmd = &cobra.Command{
	Use:   "tk",
	Short: "Minimal ticket system with dependency tracking",
	Long: `tk - minimal ticket system with dependency tracking

Tickets are stored as markdown files with YAML frontmatter in .tickets/
Supports partial ID matching (e.g., 'tk show 5c4' matches 'nw-5c46')`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		store = ticket.NewFileStore(ticketsDir)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&ticketsDir, "dir", ticket.DefaultTicketsDir, "tickets directory")
}
