package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var linkAllPairs bool

var linkCmd = &cobra.Command{
	Use:   "link <id> <id> [id...]",
	Short: "Link tickets together",
	Long: `Link tickets together (symmetric relationship).
If A links to B, then B also links to A.

By default links form a star: the first ticket is linked to each of the
remaining tickets, which are not linked to each other. Pass --all-pairs to
instead link every pair of the supplied tickets (a full mesh).`,
	Args: cobra.MinimumNArgs(2),
	RunE: runLink,
}

var unlinkCmd = &cobra.Command{
	Use:   "unlink <id> <target-id>",
	Short: "Remove link between tickets",
	Args:  cobra.ExactArgs(2),
	RunE:  runUnlink,
}

func init() {
	rootCmd.AddCommand(linkCmd)
	rootCmd.AddCommand(unlinkCmd)
	linkCmd.Flags().BoolVar(&linkAllPairs, "all-pairs", false, "Link every pair of supplied tickets (full mesh) instead of a star")
}

func runLink(cmd *cobra.Command, args []string) error {
	// Resolve all ticket IDs first
	var ids []string
	for _, arg := range args {
		t, err := store.Get(arg)
		if err != nil {
			return err
		}
		ids = append(ids, t.ID)
	}

	// Determine which peers each ticket should be linked to.
	peers := make([][]string, len(ids))
	if linkAllPairs {
		// Full mesh: every ticket links to every other ticket.
		for i := range ids {
			for j := range ids {
				if i != j {
					peers[i] = append(peers[i], ids[j])
				}
			}
		}
	} else {
		// Star: the first ticket is the hub, linked to each of the rest.
		for j := 1; j < len(ids); j++ {
			peers[0] = append(peers[0], ids[j])
			peers[j] = append(peers[j], ids[0])
		}
	}

	// Add links to each ticket.
	addedCount := 0
	for i, id := range ids {
		t, err := store.Get(id)
		if err != nil {
			return err
		}

		// Build set of existing links.
		existingLinks := make(map[string]bool)
		for _, l := range t.Links {
			existingLinks[l] = true
		}

		newLinks := t.Links
		for _, otherID := range peers[i] {
			if !existingLinks[otherID] {
				newLinks = append(newLinks, otherID)
				existingLinks[otherID] = true
				addedCount++
			}
		}

		// Update if changed
		if len(newLinks) != len(t.Links) {
			linksStr := formatLinksArray(newLinks)
			_, err := store.UpdateField(id, "links", linksStr)
			if err != nil {
				return err
			}
		}
	}

	if addedCount == 0 {
		fmt.Println("All links already exist")
	} else {
		fmt.Printf("Added %d link(s) between %d tickets\n", addedCount, len(ids))
	}

	return nil
}

func runUnlink(cmd *cobra.Command, args []string) error {
	sourceID := args[0]
	targetID := args[1]

	// Get both tickets
	source, err := store.Get(sourceID)
	if err != nil {
		return err
	}

	target, err := store.Get(targetID)
	if err != nil {
		return err
	}

	// Check if link exists
	found := false
	for _, l := range source.Links {
		if l == target.ID {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("link not found")
	}

	// Remove from source
	var newSourceLinks []string
	for _, l := range source.Links {
		if l != target.ID {
			newSourceLinks = append(newSourceLinks, l)
		}
	}
	if newSourceLinks == nil {
		newSourceLinks = []string{}
	}
	_, err = store.UpdateField(source.ID, "links", formatLinksArray(newSourceLinks))
	if err != nil {
		return err
	}

	// Remove from target
	var newTargetLinks []string
	for _, l := range target.Links {
		if l != source.ID {
			newTargetLinks = append(newTargetLinks, l)
		}
	}
	if newTargetLinks == nil {
		newTargetLinks = []string{}
	}
	_, err = store.UpdateField(target.ID, "links", formatLinksArray(newTargetLinks))
	if err != nil {
		return err
	}

	fmt.Printf("Removed link: %s <-> %s\n", source.ID, target.ID)
	return nil
}

func formatLinksArray(links []string) string {
	if len(links) == 0 {
		return "[]"
	}
	return "[" + strings.Join(links, ", ") + "]"
}
