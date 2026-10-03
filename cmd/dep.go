package cmd

import (
	"fmt"
	"strings"

	"github.com/h2oai/tk/internal/deptree"
	"github.com/h2oai/tk/internal/ticket"
	"github.com/spf13/cobra"
)

var depCmd = &cobra.Command{
	Use:   "dep <id> <dependency-id>",
	Short: "Add a dependency",
	Long: `Add a dependency to a ticket.
The first ticket will depend on the second ticket.

Also supports: dep tree [--full] <id> - show dependency tree`,
	RunE: runDep,
}

var undepCmd = &cobra.Command{
	Use:   "undep <id> <dependency-id>",
	Short: "Remove a dependency",
	Args:  cobra.ExactArgs(2),
	RunE:  runUndep,
}

var depTreeCmd = &cobra.Command{
	Use:   "tree [--full] <id>",
	Short: "Show dependency tree",
	Long: `Show the dependency tree for a ticket.
Use --full to show all occurrences (disable deduplication).`,
	Args: cobra.ExactArgs(1),
	RunE: runDepTree,
}

var depTreeFull bool

func init() {
	rootCmd.AddCommand(depCmd)
	rootCmd.AddCommand(undepCmd)

	depCmd.AddCommand(depTreeCmd)
	depTreeCmd.Flags().BoolVar(&depTreeFull, "full", false, "Show all occurrences (disable deduplication)")
}

func runDep(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: tk dep <id> <dependency-id>")
	}

	ticketID := args[0]
	depID := args[1]

	// Get the ticket
	t, err := store.Get(ticketID)
	if err != nil {
		return err
	}

	// Verify dependency exists
	dep, err := store.Get(depID)
	if err != nil {
		return err
	}

	// Check if dep already exists
	for _, d := range t.Deps {
		if d == dep.ID {
			fmt.Println("Dependency already exists")
			return nil
		}
	}

	// Add dependency
	t.Deps = append(t.Deps, dep.ID)

	// Update the field directly to preserve formatting
	newDeps := formatDepsArray(t.Deps)
	id, err := store.UpdateField(ticketID, "deps", newDeps)
	if err != nil {
		return err
	}

	fmt.Printf("Added dependency: %s -> %s\n", id, dep.ID)
	return nil
}

func runUndep(cmd *cobra.Command, args []string) error {
	ticketID := args[0]
	depID := args[1]

	// Get the ticket
	t, err := store.Get(ticketID)
	if err != nil {
		return err
	}

	// Find and remove the dependency
	found := false
	var newDeps []string
	for _, d := range t.Deps {
		if d == depID || strings.Contains(d, depID) {
			found = true
			continue
		}
		newDeps = append(newDeps, d)
	}

	if !found {
		return fmt.Errorf("dependency not found")
	}

	// Update the field
	if newDeps == nil {
		newDeps = []string{}
	}
	depsStr := formatDepsArray(newDeps)
	id, err := store.UpdateField(ticketID, "deps", depsStr)
	if err != nil {
		return err
	}

	fmt.Printf("Removed dependency: %s -/-> %s\n", id, depID)
	return nil
}

func runDepTree(cmd *cobra.Command, args []string) error {
	rootID := args[0]

	// Get all tickets
	tickets, err := store.List()
	if err != nil {
		return err
	}

	if len(tickets) == 0 {
		return fmt.Errorf("no tickets found")
	}

	// Build ticket map
	ticketMap := make(map[string]*ticket.Ticket)
	for _, t := range tickets {
		ticketMap[t.ID] = t
	}

	// Resolve root ID
	resolvedID, err := ticket.ResolveID(store.Dir(), rootID)
	if err != nil {
		return err
	}

	// Build and render tree
	tree := deptree.Build(ticketMap, resolvedID, depTreeFull)
	tree.Render()

	return nil
}

func formatDepsArray(deps []string) string {
	if len(deps) == 0 {
		return "[]"
	}
	return "[" + strings.Join(deps, ", ") + "]"
}
