package cmd

import (
	"fmt"

	"github.com/h2oai/tk/internal/ticket"
	"github.com/spf13/cobra"
)

var chainCmd = &cobra.Command{
	Use:   "chain <epic-id> <ticket-id>...",
	Short: "Sequence tickets as children of an epic",
	Long: `Chain existing tickets under an epic in sequential order.

Each listed ticket is made a child of the epic via its parent field, and the
tickets are wired into a strict dependency chain where ticket[i] depends on
ticket[i-1]. As a result, ` + "`tk ready <epic-id>`" + ` yields exactly one
runnable ticket at a time.

Re-running the same chain is a no-op: existing parent links and dependencies
are left untouched.`,
	Args: cobra.MinimumNArgs(2),
	RunE: runChain,
}

func init() {
	rootCmd.AddCommand(chainCmd)
}

func runChain(cmd *cobra.Command, args []string) error {
	epic, err := store.Get(args[0])
	if err != nil {
		return err
	}

	if epic.Type != ticket.TypeEpic {
		return fmt.Errorf("ticket %s is not an epic (type=%s)", epic.ID, epic.Type)
	}

	// Resolve every ticket argument up front so partial IDs are expanded to
	// full IDs exactly once, in the order supplied.
	resolved := make([]string, 0, len(args)-1)
	for _, arg := range args[1:] {
		t, err := store.Get(arg)
		if err != nil {
			return err
		}
		if t.ID == epic.ID {
			return fmt.Errorf("ticket %s is the epic itself and cannot be chained to it", t.ID)
		}
		resolved = append(resolved, t.ID)
	}

	changed := false

	// Make every ticket a child of the epic.
	for _, id := range resolved {
		t, err := store.Get(id)
		if err != nil {
			return err
		}
		if t.Parent == epic.ID {
			continue
		}
		if _, err := store.UpdateField(id, "parent", epic.ID); err != nil {
			return err
		}
		changed = true
	}

	// Wire the sequential dependency chain.
	var edges []string
	for i := 1; i < len(resolved); i++ {
		dependent := resolved[i]
		dependency := resolved[i-1]

		t, err := store.Get(dependent)
		if err != nil {
			return err
		}
		if containsString(t.Deps, dependency) {
			continue
		}

		newDeps := append(append([]string{}, t.Deps...), dependency)
		if _, err := store.UpdateField(dependent, "deps", formatDepsArray(newDeps)); err != nil {
			return err
		}
		edges = append(edges, fmt.Sprintf("%s depends on %s", dependent, dependency))
		changed = true
	}

	if !changed {
		fmt.Println("Chain already up to date")
		return nil
	}

	fmt.Printf("Chained %d tickets under epic %s\n", len(resolved), epic.ID)
	for _, edge := range edges {
		fmt.Println(edge)
	}

	return nil
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
