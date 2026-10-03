package cmd

import (
	"fmt"

	"github.com/h2oai/tk/internal/ticket"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm <id>",
	Short: "Delete a ticket",
	Long: `Delete a ticket after verifying it is safe to remove.

Refuses deletion if:
  - Other tickets depend on it (in their deps field)
  - Other tickets have it as a parent (in their parent field)
  - Ticket has links (bidirectional links field) without --force

Use --force to remove links automatically (still refuses if dependants/children exist).`,
	Args: cobra.ExactArgs(1),
	RunE: runRm,
}

var rmForce bool

func init() {
	rootCmd.AddCommand(rmCmd)
	rmCmd.Flags().BoolVarP(&rmForce, "force", "f", false,
		"Force deletion by removing links (still refuses if dependants/children exist)")
}

func runRm(cmd *cobra.Command, args []string) error {
	// 1. Get target ticket
	target, err := store.Get(args[0])
	if err != nil {
		return err
	}

	// 2. Get all tickets for relationship checking
	allTickets, err := store.List()
	if err != nil {
		return err
	}

	// 3. Find blocking relationships
	dependants := findDependants(allTickets, target.ID)
	children := findChildren(allTickets, target.ID)

	// 4. Check for hard blockers (dependants and children)
	if len(dependants) > 0 {
		return fmt.Errorf("cannot delete %s: ticket has dependants\n\nBlocking tickets (dependants):\n%s",
			target.ID, formatBlockingTickets(dependants))
	}

	if len(children) > 0 {
		return fmt.Errorf("cannot delete %s: ticket has children\n\nBlocking tickets (children):\n%s",
			target.ID, formatBlockingTickets(children))
	}

	// 5. Check for links (soft blocker, can be forced)
	if len(target.Links) > 0 && !rmForce {
		// Build ticket map for getting linked ticket details
		ticketMap := make(map[string]*ticket.Ticket)
		for _, t := range allTickets {
			ticketMap[t.ID] = t
		}

		var linkedTickets []*ticket.Ticket
		for _, linkID := range target.Links {
			if linked, ok := ticketMap[linkID]; ok {
				linkedTickets = append(linkedTickets, linked)
			}
		}

		return fmt.Errorf("cannot delete %s: ticket has links\n\nLinked tickets:\n%s\n\nUse --force to remove links and delete",
			target.ID, formatBlockingTickets(linkedTickets))
	}

	// 6. Remove links if --force is used and links exist
	linksRemoved := 0
	if rmForce && len(target.Links) > 0 {
		for _, linkedID := range target.Links {
			linkedTicket, err := store.Get(linkedID)
			if err != nil {
				// Skip if linked ticket doesn't exist (orphaned link)
				continue
			}

			// Remove target.ID from linked ticket's Links array
			var newLinks []string
			for _, l := range linkedTicket.Links {
				if l != target.ID {
					newLinks = append(newLinks, l)
				}
			}
			if newLinks == nil {
				newLinks = []string{}
			}

			_, err = store.UpdateField(linkedTicket.ID, "links", formatLinksArray(newLinks))
			if err != nil {
				return fmt.Errorf("failed to unlink %s: %w", linkedTicket.ID, err)
			}
			linksRemoved++
		}
	}

	// 7. Delete the ticket
	if err := store.Delete(target.ID); err != nil {
		return err
	}

	// 8. Print success message
	if linksRemoved > 0 {
		fmt.Printf("Removed %d link(s) and deleted ticket: %s\n", linksRemoved, target.ID)
	} else {
		fmt.Printf("Deleted ticket: %s\n", target.ID)
	}

	return nil
}
