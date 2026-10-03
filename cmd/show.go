package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/h2oai/tk/internal/ticket"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Display a ticket",
	Long:  "Display a ticket with its metadata, content, and relationships.",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	target, err := store.Get(args[0])
	if err != nil {
		var ambiguous ticket.ErrAmbiguous
		if errors.As(err, &ambiguous) {
			return printAmbiguousMatches(ambiguous)
		}
		return err
	}

	// Get all tickets to compute relationships
	allTickets, err := store.List()
	if err != nil {
		return err
	}

	// Build lookup maps
	ticketMap := make(map[string]*ticket.Ticket)
	for _, t := range allTickets {
		ticketMap[t.ID] = t
	}

	// Find relationships
	var blockers []*ticket.Ticket // Unclosed deps of this ticket
	var blocking []*ticket.Ticket // Tickets that have this as a dep (not closed)
	var children []*ticket.Ticket // Tickets with this as parent
	var linked []*ticket.Ticket   // Tickets in links array

	// Blockers: unclosed deps
	for _, depID := range target.Deps {
		if dep, ok := ticketMap[depID]; ok {
			if dep.Status != ticket.StatusClosed {
				blockers = append(blockers, dep)
			}
		}
	}

	// Blocking: tickets that depend on this one (and are not closed)
	for _, t := range allTickets {
		if t.ID == target.ID {
			continue
		}
		for _, depID := range t.Deps {
			if depID == target.ID && t.Status != ticket.StatusClosed {
				blocking = append(blocking, t)
				break
			}
		}
	}

	// Children: tickets with this as parent
	for _, t := range allTickets {
		if t.Parent == target.ID {
			children = append(children, t)
		}
	}

	// Linked: tickets in links array
	for _, linkID := range target.Links {
		if linked_t, ok := ticketMap[linkID]; ok {
			linked = append(linked, linked_t)
		}
	}

	// Output the ticket
	printTicket(target, ticketMap)

	// Print relationship sections
	if len(blockers) > 0 {
		fmt.Println()
		fmt.Println("## Blockers")
		fmt.Println()
		for _, b := range blockers {
			fmt.Printf("- %s [%s] %s\n", b.ID, b.Status, b.Title)
		}
	}

	if len(blocking) > 0 {
		fmt.Println()
		fmt.Println("## Blocking")
		fmt.Println()
		for _, b := range blocking {
			fmt.Printf("- %s [%s] %s\n", b.ID, b.Status, b.Title)
		}
	}

	if len(children) > 0 {
		fmt.Println()
		fmt.Println("## Children")
		fmt.Println()
		for _, c := range children {
			fmt.Printf("- %s [%s] %s\n", c.ID, c.Status, c.Title)
		}
	}

	if len(linked) > 0 {
		fmt.Println()
		fmt.Println("## Linked")
		fmt.Println()
		for _, l := range linked {
			fmt.Printf("- %s [%s] %s\n", l.ID, l.Status, l.Title)
		}
	}

	return nil
}

func printAmbiguousMatches(ambiguous ticket.ErrAmbiguous) error {
	matches := append([]string(nil), ambiguous.Matches...)
	sort.Strings(matches)

	fmt.Printf("Multiple tickets found matching '%s':\n\n", ambiguous.ID)
	for _, id := range matches {
		t, err := store.Get(id)
		if err != nil {
			return err
		}
		fmt.Printf("- %s [%s] %s\n", t.ID, t.Status, t.Title)
	}
	return nil
}

func printTicket(t *ticket.Ticket, ticketMap map[string]*ticket.Ticket) {
	fmt.Println("---")
	fmt.Printf("id: %s\n", t.ID)
	fmt.Printf("status: %s\n", t.Status)
	fmt.Printf("deps: %s\n", formatArray(t.Deps))
	fmt.Printf("links: %s\n", formatArray(t.Links))
	fmt.Printf("created: %s\n", t.Created.UTC().Format("2006-01-02T15:04:05Z"))
	fmt.Printf("type: %s\n", t.Type)
	fmt.Printf("priority: %d\n", t.Priority)
	if t.Assignee != "" {
		fmt.Printf("assignee: %s\n", t.Assignee)
	}
	if t.ExternalRef != "" {
		fmt.Printf("external-ref: %s\n", t.ExternalRef)
	}
	if t.Parent != "" {
		if parent, ok := ticketMap[t.Parent]; ok {
			fmt.Printf("parent: %s  # %s\n", t.Parent, parent.Title)
		} else {
			fmt.Printf("parent: %s\n", t.Parent)
		}
	}
	fmt.Println("---")
	fmt.Printf("# %s\n", t.Title)

	if t.Body != "" {
		fmt.Println()
		// Output body to stdout
		os.Stdout.WriteString(t.Body)
		if !strings.HasSuffix(t.Body, "\n") {
			fmt.Println()
		}
	}
}

func formatArray(arr []string) string {
	if len(arr) == 0 {
		return "[]"
	}
	return "[" + strings.Join(arr, ", ") + "]"
}
