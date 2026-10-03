package cmd

import (
	"fmt"
	"strings"

	"github.com/h2oai/tk/internal/ticket"
	"github.com/spf13/cobra"
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove dangling references from tickets",
	Long: `Scan all tickets and remove references to non-existent tickets.

By default, performs a dry-run showing what would be changed.
Use --fix to actually remove the dangling references.

Checks three types of references:
  - deps: Dependencies that point to deleted tickets
  - links: Bidirectional links to deleted tickets
  - parent: Parent references to deleted tickets`,
	Args: cobra.NoArgs,
	RunE: runPrune,
}

var pruneFix bool

func init() {
	rootCmd.AddCommand(pruneCmd)
	pruneCmd.Flags().BoolVar(&pruneFix, "fix", false,
		"Actually remove dangling references (default is dry-run)")
}

type danglingRefs struct {
	ticket *ticket.Ticket
	deps   []string // dangling dep IDs
	links  []string // dangling link IDs
	parent string   // dangling parent ID (empty if none)
}

func runPrune(cmd *cobra.Command, args []string) error {
	// 1. Load all tickets
	allTickets, err := store.List()
	if err != nil {
		return err
	}

	if len(allTickets) == 0 {
		fmt.Println("No tickets found.")
		return nil
	}

	// 2. Build valid ID set
	validIDs := buildValidIDSet(allTickets)

	// 3. Find dangling references
	dangling := findDanglingRefs(allTickets, validIDs)

	// 4. Display or fix
	if len(dangling) == 0 {
		fmt.Println("No dangling references found.")
		return nil
	}

	if !pruneFix {
		displayDryRun(dangling)
		return nil
	}

	// 5. Fix and report
	return fixDanglingRefs(cmd, dangling)
}

func buildValidIDSet(tickets []*ticket.Ticket) map[string]bool {
	validIDs := make(map[string]bool)
	for _, t := range tickets {
		validIDs[t.ID] = true
	}
	return validIDs
}

func findDanglingRefs(tickets []*ticket.Ticket, validIDs map[string]bool) []danglingRefs {
	var result []danglingRefs

	for _, t := range tickets {
		var dr danglingRefs
		dr.ticket = t

		// Check deps
		for _, depID := range t.Deps {
			if !validIDs[depID] {
				dr.deps = append(dr.deps, depID)
			}
		}

		// Check links
		for _, linkID := range t.Links {
			if !validIDs[linkID] {
				dr.links = append(dr.links, linkID)
			}
		}

		// Check parent
		if t.Parent != "" && !validIDs[t.Parent] {
			dr.parent = t.Parent
		}

		// Add to result if any dangling refs found
		if len(dr.deps) > 0 || len(dr.links) > 0 || dr.parent != "" {
			result = append(result, dr)
		}
	}

	return result
}

func displayDryRun(dangling []danglingRefs) {
	totalDeps := 0
	totalLinks := 0
	totalParents := 0

	fmt.Printf("Scanning tickets for dangling references...\n\n")
	fmt.Printf("Found dangling references in %d ticket(s):\n\n", len(dangling))

	for _, dr := range dangling {
		fmt.Printf("%s [%s] %s\n", dr.ticket.ID, dr.ticket.Status, dr.ticket.Title)

		if len(dr.deps) > 0 {
			fmt.Printf("  deps: %s (do not exist)\n", strings.Join(dr.deps, ", "))
			totalDeps += len(dr.deps)
		}

		if len(dr.links) > 0 {
			fmt.Printf("  links: %s (do not exist)\n", strings.Join(dr.links, ", "))
			totalLinks += len(dr.links)
		}

		if dr.parent != "" {
			fmt.Printf("  parent: %s (does not exist)\n", dr.parent)
			totalParents++
		}

		fmt.Println()
	}

	fmt.Println("Summary:")
	if totalDeps > 0 {
		fmt.Printf("  %d dangling deps\n", totalDeps)
	}
	if totalLinks > 0 {
		fmt.Printf("  %d dangling links\n", totalLinks)
	}
	if totalParents > 0 {
		fmt.Printf("  %d dangling parent(s)\n", totalParents)
	}
	total := totalDeps + totalLinks + totalParents
	fmt.Printf("  %d total dangling references\n", total)
	fmt.Println("\nRun with --fix to remove these references.")
}

func fixDanglingRefs(cmd *cobra.Command, dangling []danglingRefs) error {
	totalFixed := 0
	totalTickets := 0

	fmt.Println("Pruning dangling references...")
	fmt.Println()

	for _, dr := range dangling {
		ticketFixed := false

		// Fix deps
		if len(dr.deps) > 0 {
			validDeps := filterOutDangling(dr.ticket.Deps, dr.deps)
			depsStr := formatDepsArray(validDeps)
			_, err := store.UpdateField(dr.ticket.ID, "deps", depsStr)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "Warning: failed to update deps for %s: %v\n", dr.ticket.ID, err)
			} else {
				fmt.Printf("%s: Removed deps: %s\n", dr.ticket.ID, strings.Join(dr.deps, ", "))
				totalFixed += len(dr.deps)
				ticketFixed = true
			}
		}

		// Fix links
		if len(dr.links) > 0 {
			validLinks := filterOutDangling(dr.ticket.Links, dr.links)
			linksStr := formatLinksArray(validLinks)
			_, err := store.UpdateField(dr.ticket.ID, "links", linksStr)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "Warning: failed to update links for %s: %v\n", dr.ticket.ID, err)
			} else {
				fmt.Printf("%s: Removed links: %s\n", dr.ticket.ID, strings.Join(dr.links, ", "))
				totalFixed += len(dr.links)
				ticketFixed = true
			}
		}

		// Fix parent
		if dr.parent != "" {
			_, err := store.UpdateField(dr.ticket.ID, "parent", "")
			if err != nil {
				fmt.Fprintf(cmd.OutOrStderr(), "Warning: failed to update parent for %s: %v\n", dr.ticket.ID, err)
			} else {
				fmt.Printf("%s: Removed parent: %s\n", dr.ticket.ID, dr.parent)
				totalFixed++
				ticketFixed = true
			}
		}

		if ticketFixed {
			totalTickets++
		}
	}

	fmt.Println()
	fmt.Printf("Pruned %d dangling reference(s) from %d ticket(s).\n", totalFixed, totalTickets)
	return nil
}

// filterOutDangling returns a new slice with dangling IDs removed
func filterOutDangling(original []string, dangling []string) []string {
	danglingSet := make(map[string]bool)
	for _, d := range dangling {
		danglingSet[d] = true
	}

	var result []string
	for _, id := range original {
		if !danglingSet[id] {
			result = append(result, id)
		}
	}

	if result == nil {
		result = []string{}
	}

	return result
}
