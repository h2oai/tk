package cmd

import (
	"fmt"
	"strings"

	"github.com/h2oai/tk/internal/ticket"
)

// formatBlockingTickets formats a list of tickets for error messages
func formatBlockingTickets(tickets []*ticket.Ticket) string {
	var lines []string
	for _, t := range tickets {
		lines = append(lines, fmt.Sprintf("  - %s [%s] %s", t.ID, t.Status, t.Title))
	}
	return strings.Join(lines, "\n")
}

// findDependants returns tickets that depend on targetID
func findDependants(allTickets []*ticket.Ticket, targetID string) []*ticket.Ticket {
	var dependants []*ticket.Ticket
	for _, t := range allTickets {
		if t.ID == targetID {
			continue
		}
		for _, depID := range t.Deps {
			if depID == targetID {
				dependants = append(dependants, t)
				break
			}
		}
	}
	return dependants
}

// findChildren returns tickets that have targetID as parent
func findChildren(allTickets []*ticket.Ticket, targetID string) []*ticket.Ticket {
	var children []*ticket.Ticket
	for _, t := range allTickets {
		if t.Parent == targetID {
			children = append(children, t)
		}
	}
	return children
}
