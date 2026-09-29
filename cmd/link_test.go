package cmd

import (
	"strings"
	"testing"

	"github.com/lo5/tk/internal/ticket"
)

// TestLinkCommand tests the link command (symmetric linking)
func TestLinkCommand(t *testing.T) {
	t.Run("symmetric linking two tickets", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create two tickets
		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		// Link A and B
		output, err := ctx.exec("link", idA, idB)
		if err != nil {
			t.Fatalf("link command error: %v", err)
		}

		// Verify output message
		if !strings.Contains(output, "Added") && !strings.Contains(output, "link") {
			t.Errorf("output should contain success message, got: %s", output)
		}

		// Verify A has B in links
		ticketA, _ := ctx.store().Get(idA)
		if len(ticketA.Links) != 1 {
			t.Errorf("ticket A: expected 1 link, got %d", len(ticketA.Links))
		}
		if ticketA.Links[0] != idB {
			t.Errorf("ticket A: link = %v, want %v", ticketA.Links[0], idB)
		}

		// Verify B has A in links (symmetric)
		ticketB, _ := ctx.store().Get(idB)
		if len(ticketB.Links) != 1 {
			t.Errorf("ticket B: expected 1 link, got %d", len(ticketB.Links))
		}
		if ticketB.Links[0] != idA {
			t.Errorf("ticket B: link = %v, want %v", ticketB.Links[0], idA)
		}
	})

	t.Run("already linked is idempotent", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		// Link first time
		_, err := ctx.exec("link", idA, idB)
		if err != nil {
			t.Fatalf("first link error: %v", err)
		}

		// Link again
		output, err := ctx.exec("link", idA, idB)
		if err != nil {
			t.Fatalf("second link error: %v", err)
		}

		// Should output "All links already exist"
		if !strings.Contains(output, "All links already exist") {
			t.Errorf("expected 'All links already exist' message, got: %s", output)
		}

		// Verify still only one link each
		ticketA, _ := ctx.store().Get(idA)
		if len(ticketA.Links) != 1 {
			t.Errorf("ticket A: expected 1 link after duplicate, got %d", len(ticketA.Links))
		}

		ticketB, _ := ctx.store().Get(idB)
		if len(ticketB.Links) != 1 {
			t.Errorf("ticket B: expected 1 link after duplicate, got %d", len(ticketB.Links))
		}
	})

	t.Run("partial IDs resolved", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		// Use partial IDs
		partialA := idA[strings.Index(idA, "-")+1:]
		partialB := idB[strings.Index(idB, "-")+1:]

		// Link with partial IDs
		output, err := ctx.exec("link", partialA, partialB)
		if err != nil {
			t.Fatalf("link with partial IDs error: %v", err)
		}

		if !strings.Contains(output, "Added") {
			t.Errorf("expected success message, got: %s", output)
		}

		// Verify links were created with full IDs
		ticketA, _ := ctx.store().Get(idA)
		if len(ticketA.Links) != 1 || ticketA.Links[0] != idB {
			t.Errorf("link not created correctly with partial IDs")
		}
	})
}

// TestLinkCommandMultiWay tests multi-way linking
func TestLinkCommandMultiWay(t *testing.T) {
	t.Run("link three tickets as a star", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create three tickets
		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)
		idC, _ := ctx.exec("new", "Ticket C")
		idC = strings.TrimSpace(idC)

		// Link the hub (A) to B and C. B and C must NOT be linked to each other.
		output, err := ctx.exec("link", idA, idB, idC)
		if err != nil {
			t.Fatalf("link three tickets error: %v", err)
		}

		// Output should mention count and number of tickets
		if !strings.Contains(output, "3 tickets") {
			t.Errorf("output should mention '3 tickets', got: %s", output)
		}
		if !strings.Contains(output, "link") {
			t.Errorf("output should contain 'link', got: %s", output)
		}

		// Verify A is linked to B and C
		ticketA, _ := ctx.store().Get(idA)
		if len(ticketA.Links) != 2 {
			t.Errorf("ticket A: expected 2 links, got %d", len(ticketA.Links))
		}
		hasB := false
		hasC := false
		for _, link := range ticketA.Links {
			if link == idB {
				hasB = true
			}
			if link == idC {
				hasC = true
			}
		}
		if !hasB || !hasC {
			t.Errorf("ticket A should be linked to both B and C")
		}

		// Verify B is linked only to A (not to C)
		ticketB, _ := ctx.store().Get(idB)
		if len(ticketB.Links) != 1 || ticketB.Links[0] != idA {
			t.Errorf("ticket B: expected only link to A, got %v", ticketB.Links)
		}

		// Verify C is linked only to A (not to B)
		ticketC, _ := ctx.store().Get(idC)
		if len(ticketC.Links) != 1 || ticketC.Links[0] != idA {
			t.Errorf("ticket C: expected only link to A, got %v", ticketC.Links)
		}
	})

	t.Run("all-pairs links every pair", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)
		idC, _ := ctx.exec("new", "Ticket C")
		idC = strings.TrimSpace(idC)

		// With --all-pairs every ticket links to every other ticket.
		if _, err := ctx.exec("link", "--all-pairs", idA, idB, idC); err != nil {
			t.Fatalf("link --all-pairs error: %v", err)
		}

		for _, tc := range []struct {
			name string
			id   string
			want []string
		}{
			{"A", idA, []string{idB, idC}},
			{"B", idB, []string{idA, idC}},
			{"C", idC, []string{idA, idB}},
		} {
			ticket, _ := ctx.store().Get(tc.id)
			got := make(map[string]bool, len(ticket.Links))
			for _, l := range ticket.Links {
				got[l] = true
			}
			if len(ticket.Links) != len(tc.want) {
				t.Errorf("ticket %s: expected %d links, got %v", tc.name, len(tc.want), ticket.Links)
			}
			for _, w := range tc.want {
				if !got[w] {
					t.Errorf("ticket %s: missing link to %s (got %v)", tc.name, w, ticket.Links)
				}
			}
		}
	})

	t.Run("count reflects new links added", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)
		idC, _ := ctx.exec("new", "Ticket C")
		idC = strings.TrimSpace(idC)

		// First, link A and B
		_, err := ctx.exec("link", idA, idB)
		if err != nil {
			t.Fatalf("first link error: %v", err)
		}

		// Now link all three - should only add new links (A-C, B-C)
		output, err := ctx.exec("link", idA, idB, idC)
		if err != nil {
			t.Fatalf("second link error: %v", err)
		}

		// Output should reflect only the NEW links added
		// New links: A-C, C-A, B-C, C-B = 4 new links
		if !strings.Contains(output, "Added") {
			t.Errorf("output should mention 'Added', got: %s", output)
		}
	})

	t.Run("idempotent multi-way linking", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)
		idC, _ := ctx.exec("new", "Ticket C")
		idC = strings.TrimSpace(idC)

		// Link all three
		_, err := ctx.exec("link", idA, idB, idC)
		if err != nil {
			t.Fatalf("first link error: %v", err)
		}

		// Link again
		output, err := ctx.exec("link", idA, idB, idC)
		if err != nil {
			t.Fatalf("second link error: %v", err)
		}

		// Should say all links already exist
		if !strings.Contains(output, "All links already exist") {
			t.Errorf("expected 'All links already exist', got: %s", output)
		}
	})
}

// TestLinkArrayFormat tests the formatting of link arrays
func TestLinkArrayFormat(t *testing.T) {
	t.Run("empty links to single link", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		// Link
		ctx.exec("link", idA, idB)

		// Check format
		ticketA, _ := ctx.store().Get(idA)
		if len(ticketA.Links) != 1 {
			t.Fatalf("expected 1 link, got %d", len(ticketA.Links))
		}
		if ticketA.Links[0] != idB {
			t.Errorf("links[0] = %v, want %v", ticketA.Links[0], idB)
		}
	})

	t.Run("existing link plus new link", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)
		idC, _ := ctx.exec("new", "Ticket C")
		idC = strings.TrimSpace(idC)

		// Link A-B
		ctx.exec("link", idA, idB)

		// Link A-C
		ctx.exec("link", idA, idC)

		// Verify A has both links
		ticketA, _ := ctx.store().Get(idA)
		if len(ticketA.Links) != 2 {
			t.Fatalf("expected 2 links, got %d", len(ticketA.Links))
		}

		// Should contain both IDs
		hasB := false
		hasC := false
		for _, link := range ticketA.Links {
			if link == idB {
				hasB = true
			}
			if link == idC {
				hasC = true
			}
		}
		if !hasB || !hasC {
			t.Errorf("ticket A should have links to both B and C")
		}
	})
}

// TestUnlinkCommand tests the unlink command
func TestUnlinkCommand(t *testing.T) {
	t.Run("remove link between two tickets", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		// Link
		ctx.exec("link", idA, idB)

		// Unlink
		output, err := ctx.exec("unlink", idA, idB)
		if err != nil {
			t.Fatalf("unlink error: %v", err)
		}

		// Verify output message
		if !strings.Contains(output, "Removed link:") {
			t.Errorf("output should contain 'Removed link:', got: %s", output)
		}
		if !strings.Contains(output, "<->") {
			t.Errorf("output should contain '<->', got: %s", output)
		}

		// Verify A has no links
		ticketA, _ := ctx.store().Get(idA)
		if len(ticketA.Links) != 0 {
			t.Errorf("ticket A: expected 0 links after unlink, got %d", len(ticketA.Links))
		}

		// Verify B has no links (symmetric removal)
		ticketB, _ := ctx.store().Get(idB)
		if len(ticketB.Links) != 0 {
			t.Errorf("ticket B: expected 0 links after unlink, got %d", len(ticketB.Links))
		}
	})

	t.Run("unlink non-existent link", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		// Try to unlink without linking first
		_, err := ctx.exec("unlink", idA, idB)
		if err == nil {
			t.Error("expected error for non-existent link")
		}
		if !strings.Contains(err.Error(), "link not found") {
			t.Errorf("error should mention 'link not found', got: %v", err)
		}
	})

	t.Run("partial IDs resolved", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		// Link
		ctx.exec("link", idA, idB)

		// Use partial IDs to unlink
		partialA := idA[strings.Index(idA, "-")+1:]
		partialB := idB[strings.Index(idB, "-")+1:]

		output, err := ctx.exec("unlink", partialA, partialB)
		if err != nil {
			t.Fatalf("unlink with partial IDs error: %v", err)
		}

		if !strings.Contains(output, "Removed link:") {
			t.Errorf("expected success message, got: %s", output)
		}

		// Verify link removed
		ticketA, _ := ctx.store().Get(idA)
		if len(ticketA.Links) != 0 {
			t.Errorf("expected 0 links after unlink, got %d", len(ticketA.Links))
		}
	})
}

// TestMultipleLinks tests handling of multiple links
func TestMultipleLinks(t *testing.T) {
	t.Run("link A to B, C, D", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)
		idC, _ := ctx.exec("new", "Ticket C")
		idC = strings.TrimSpace(idC)
		idD, _ := ctx.exec("new", "Ticket D")
		idD = strings.TrimSpace(idD)

		// Link A-B
		ctx.exec("link", idA, idB)
		// Link A-C
		ctx.exec("link", idA, idC)
		// Link A-D
		ctx.exec("link", idA, idD)

		// Verify A has all three links
		ticketA, _ := ctx.store().Get(idA)
		if len(ticketA.Links) != 3 {
			t.Errorf("ticket A: expected 3 links, got %d", len(ticketA.Links))
		}

		// Verify all IDs present
		linkSet := make(map[string]bool)
		for _, link := range ticketA.Links {
			linkSet[link] = true
		}
		if !linkSet[idB] || !linkSet[idC] || !linkSet[idD] {
			t.Errorf("ticket A should be linked to B, C, and D")
		}
	})

	t.Run("unlink one pair, others remain", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)
		idC, _ := ctx.exec("new", "Ticket C")
		idC = strings.TrimSpace(idC)

		// Link A-B and A-C
		ctx.exec("link", idA, idB)
		ctx.exec("link", idA, idC)

		// Unlink A-B
		_, err := ctx.exec("unlink", idA, idB)
		if err != nil {
			t.Fatalf("unlink error: %v", err)
		}

		// Verify A still has link to C
		ticketA, _ := ctx.store().Get(idA)
		if len(ticketA.Links) != 1 {
			t.Errorf("ticket A: expected 1 link remaining, got %d", len(ticketA.Links))
		}
		if ticketA.Links[0] != idC {
			t.Errorf("remaining link should be to C, got %v", ticketA.Links[0])
		}

		// Verify B has no links
		ticketB, _ := ctx.store().Get(idB)
		if len(ticketB.Links) != 0 {
			t.Errorf("ticket B: expected 0 links, got %d", len(ticketB.Links))
		}

		// Verify C still has link to A
		ticketC, _ := ctx.store().Get(idC)
		if len(ticketC.Links) != 1 {
			t.Errorf("ticket C: expected 1 link, got %d", len(ticketC.Links))
		}
		if ticketC.Links[0] != idA {
			t.Errorf("ticket C link should be to A, got %v", ticketC.Links[0])
		}
	})
}

// TestLinkPersistence tests that link changes are persisted
func TestLinkPersistence(t *testing.T) {
	t.Run("links written to disk", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		// Link
		ctx.exec("link", idA, idB)

		// Create new store to verify persistence
		newStore := ticket.NewFileStore(ctx.ticketsDir)
		reloadedA, err := newStore.Get(idA)
		if err != nil {
			t.Fatalf("failed to reload ticket A: %v", err)
		}

		if len(reloadedA.Links) != 1 || reloadedA.Links[0] != idB {
			t.Errorf("link not persisted correctly for ticket A")
		}

		reloadedB, err := newStore.Get(idB)
		if err != nil {
			t.Fatalf("failed to reload ticket B: %v", err)
		}

		if len(reloadedB.Links) != 1 || reloadedB.Links[0] != idA {
			t.Errorf("link not persisted correctly for ticket B")
		}
	})

	t.Run("unlink persisted", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		// Link and unlink
		ctx.exec("link", idA, idB)
		ctx.exec("unlink", idA, idB)

		// Verify persistence
		newStore := ticket.NewFileStore(ctx.ticketsDir)
		reloadedA, _ := newStore.Get(idA)
		reloadedB, _ := newStore.Get(idB)

		if len(reloadedA.Links) != 0 {
			t.Errorf("ticket A: expected 0 links after unlink, got %d", len(reloadedA.Links))
		}
		if len(reloadedB.Links) != 0 {
			t.Errorf("ticket B: expected 0 links after unlink, got %d", len(reloadedB.Links))
		}
	})
}

// TestLinkCommandOutput tests the output format of link commands
func TestLinkCommandOutput(t *testing.T) {
	t.Run("link output format", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		output, _ := ctx.exec("link", idA, idB)

		// Should mention links and tickets
		if !strings.Contains(output, "link") {
			t.Errorf("output should mention 'link', got: %s", output)
		}
		if !strings.Contains(output, "2 tickets") {
			t.Errorf("output should mention '2 tickets', got: %s", output)
		}
	})

	t.Run("unlink output format", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		ctx.exec("link", idA, idB)
		output, _ := ctx.exec("unlink", idA, idB)

		// Format should be: "Removed link: {id} <-> {target-id}"
		if !strings.Contains(output, "Removed link:") {
			t.Errorf("missing 'Removed link:' in output")
		}
		if !strings.Contains(output, "<->") {
			t.Errorf("missing '<->' in output")
		}
		if !strings.Contains(output, idA) || !strings.Contains(output, idB) {
			t.Errorf("output should contain both IDs")
		}
	})
}

// TestLinkTicketExistence tests that both tickets must exist for linking
func TestLinkTicketExistence(t *testing.T) {
	t.Run("both tickets must exist", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)

		// Try to link with non-existent ticket
		_, err := ctx.exec("link", idA, "nonexistent")
		if err == nil {
			t.Error("expected error for non-existent ticket")
		}
	})

	t.Run("first ticket must exist", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		// Try to link from non-existent ticket
		_, err := ctx.exec("link", "nonexistent", idB)
		if err == nil {
			t.Error("expected error for non-existent ticket")
		}
	})
}
