package cmd

import (
	"fmt"

	"github.com/lo5/tk/internal/query"
	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query [jq-filter]",
	Short: "Output tickets as JSON",
	Long: `Output tickets as JSON, one object per line.
Optionally apply a jq-style filter.

Examples:
  tk query                          # All tickets as JSON
  tk query '.priority == "0"'       # High priority tickets
  tk query '.status == "open"'      # Open tickets
  tk query -F filter.jq             # Filter read from a file
  tk query - <<'EOF'                # Filter read from stdin, no quoting
  .status == "open" and .priority == "0"
  EOF`,
	Args: cobra.MaximumNArgs(1),
	RunE: runQuery,
}

var queryFile string

func init() {
	rootCmd.AddCommand(queryCmd)

	queryCmd.Flags().StringVarP(&queryFile, "file", "F", "", "Read filter from file (- reads stdin)")
}

func runQuery(cmd *cobra.Command, args []string) error {
	var inline string
	if len(args) > 0 {
		inline = args[0]
	}
	filter, err := resolveText(cmd, inline, queryFile, "a filter argument")
	if err != nil {
		return err
	}

	tickets, err := store.List()
	if err != nil {
		return err
	}

	// Convert all tickets to JSON
	var jsonLines []string
	for _, t := range tickets {
		line, err := query.ToJSON(t)
		if err != nil {
			continue
		}
		jsonLines = append(jsonLines, line)
	}

	// Apply filter if provided
	if filter != "" {
		filtered, err := query.Filter(jsonLines, filter)
		if err != nil {
			return err
		}
		jsonLines = filtered
	}

	// Print results
	for _, line := range jsonLines {
		fmt.Println(line)
	}

	return nil
}
