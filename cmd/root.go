package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// DefaultDir is the tickets directory used when --dir is not given.
const DefaultDir = ".tickets"

var ticketsDir string

var rootCmd = &cobra.Command{
	Use:           "tk",
	Short:         "Minimal ordered-tree ticket tracker",
	SilenceUsage:  true,
	SilenceErrors: false,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&ticketsDir, "dir", DefaultDir, "tickets directory")
}
