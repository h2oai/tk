package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lo5/tk/internal/ticket"
)

// TestRmCommand - Basic deletion tests
func TestRmCommand(t *testing.T) {
	t.Run("delete ticket with no relationships", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create a ticket
		id, _ := ctx.exec("new", "Test ticket")
		id = strings.TrimSpace(id)

		// Delete it
		output, err := ctx.exec("rm", id)
		if err != nil {
			t.Fatalf("rm command error: %v", err)
		}

		// Verify output message
		if !strings.Contains(output, "Deleted ticket:") {
			t.Errorf("output should contain 'Deleted ticket:', got: %s", output)
		}
		if !strings.Contains(output, id) {
			t.Errorf("output should contain ticket ID %s, got: %s", id, output)
		}

		// Verify ticket is gone from store
		_, err = ctx.store().Get(id)
		if err == nil {
			t.Error("ticket should not exist after deletion")
		}

		// Verify ticket not in List()
		allTickets, _ := ctx.store().List()
		for _, tkt := range allTickets {
			if tkt.ID == id {
				t.Error("deleted ticket should not appear in list")
			}
		}

		// Verify file doesn't exist
		path := filepath.Join(ctx.ticketsDir, id+".md")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("ticket file should be deleted")
		}
	})

	t.Run("partial ID resolution", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id, _ := ctx.exec("new", "Test ticket")
		id = strings.TrimSpace(id)

		// Use partial ID (the verb-noun portion)
		partial := id[strings.Index(id, "-")+1:]

		output, err := ctx.exec("rm", partial)
		if err != nil {
			t.Fatalf("rm with partial ID error: %v", err)
		}

		if !strings.Contains(output, "Deleted ticket:") {
			t.Errorf("expected success message, got: %s", output)
		}

		// Verify ticket is deleted
		_, err = ctx.store().Get(id)
		if err == nil {
			t.Error("ticket should be deleted")
		}
	})

	t.Run("non-existent ticket error", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		_, err := ctx.exec("rm", "nonexistent")
		if err == nil {
			t.Error("expected error for non-existent ticket")
		}
	})
}

// TestRmRefuseWithDependants - Blocking by dependants
func TestRmRefuseWithDependants(t *testing.T) {
	t.Run("refuse deletion with single dependant", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create A and B, B depends on A
		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		ctx.exec("dep", idB, idA)

		// Try to delete A
		_, err := ctx.exec("rm", idA)
		if err == nil {
			t.Error("expected error when deleting ticket with dependants")
		}

		// Verify error message
		if !strings.Contains(err.Error(), "dependants") {
			t.Errorf("error should mention 'dependants', got: %v", err)
		}
		if !strings.Contains(err.Error(), idB) {
			t.Errorf("error should list blocking ticket %s, got: %v", idB, err)
		}
		if !strings.Contains(err.Error(), "Ticket B") {
			t.Errorf("error should show ticket title, got: %v", err)
		}

		// Verify A still exists
		ticketA, err := ctx.store().Get(idA)
		if err != nil {
			t.Error("ticket A should still exist after refused deletion")
		}
		if ticketA == nil {
			t.Error("ticket A should not be nil")
		}
	})

	t.Run("refuse deletion with multiple dependants", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create A, B, C; both B and C depend on A
		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)
		idC, _ := ctx.exec("new", "Ticket C")
		idC = strings.TrimSpace(idC)

		ctx.exec("dep", idB, idA)
		ctx.exec("dep", idC, idA)

		// Try to delete A
		_, err := ctx.exec("rm", idA)
		if err == nil {
			t.Error("expected error when deleting ticket with dependants")
		}

		// Verify error lists both B and C
		if !strings.Contains(err.Error(), idB) {
			t.Errorf("error should list %s", idB)
		}
		if !strings.Contains(err.Error(), idC) {
			t.Errorf("error should list %s", idC)
		}
	})

	t.Run("force flag still refuses with dependants", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		ctx.exec("dep", idB, idA)

		// Try to delete A with --force
		_, err := ctx.exec("rm", "--force", idA)
		if err == nil {
			t.Error("--force should still refuse deletion with dependants")
		}

		if !strings.Contains(err.Error(), "dependants") {
			t.Errorf("error should mention 'dependants', got: %v", err)
		}
	})

	t.Run("can delete after removing dependencies", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		// B depends on A
		ctx.exec("dep", idB, idA)

		// Remove dependency
		ctx.exec("undep", idB, idA)

		// Now should be able to delete A
		output, err := ctx.exec("rm", idA)
		if err != nil {
			t.Fatalf("should be able to delete after removing dependency: %v", err)
		}

		if !strings.Contains(output, "Deleted ticket:") {
			t.Errorf("expected success message, got: %s", output)
		}
	})
}

// TestRmRefuseWithChildren - Blocking by children
func TestRmRefuseWithChildren(t *testing.T) {
	t.Run("refuse deletion with single child", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create parent P
		idP, _ := ctx.exec("new", "Parent ticket")
		idP = strings.TrimSpace(idP)

		// Create child C with P as parent
		idC, _ := ctx.exec("new", "Child ticket", "--parent", idP)
		idC = strings.TrimSpace(idC)

		// Try to delete P
		_, err := ctx.exec("rm", idP)
		if err == nil {
			t.Error("expected error when deleting ticket with children")
		}

		// Verify error message
		if !strings.Contains(err.Error(), "children") {
			t.Errorf("error should mention 'children', got: %v", err)
		}
		if !strings.Contains(err.Error(), idC) {
			t.Errorf("error should list blocking child %s, got: %v", idC, err)
		}
		if !strings.Contains(err.Error(), "Child ticket") {
			t.Errorf("error should show ticket title, got: %v", err)
		}

		// Verify P still exists
		_, err = ctx.store().Get(idP)
		if err != nil {
			t.Error("parent ticket should still exist after refused deletion")
		}
	})

	t.Run("refuse deletion with multiple children", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idP, _ := ctx.exec("new", "Parent ticket")
		idP = strings.TrimSpace(idP)
		idC1, _ := ctx.exec("new", "Child 1", "--parent", idP)
		idC1 = strings.TrimSpace(idC1)
		idC2, _ := ctx.exec("new", "Child 2", "--parent", idP)
		idC2 = strings.TrimSpace(idC2)

		// Try to delete P
		_, err := ctx.exec("rm", idP)
		if err == nil {
			t.Error("expected error when deleting ticket with children")
		}

		// Verify error lists both children
		if !strings.Contains(err.Error(), idC1) {
			t.Errorf("error should list %s", idC1)
		}
		if !strings.Contains(err.Error(), idC2) {
			t.Errorf("error should list %s", idC2)
		}
	})

	t.Run("force flag still refuses with children", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idP, _ := ctx.exec("new", "Parent ticket")
		idP = strings.TrimSpace(idP)
		idC, _ := ctx.exec("new", "Child ticket", "--parent", idP)
		idC = strings.TrimSpace(idC)

		// Try to delete P with --force
		_, err := ctx.exec("rm", "--force", idP)
		if err == nil {
			t.Error("--force should still refuse deletion with children")
		}

		if !strings.Contains(err.Error(), "children") {
			t.Errorf("error should mention 'children', got: %v", err)
		}
	})
}

// TestRmRefuseWithLinks - Blocking by links (without force)
func TestRmRefuseWithLinks(t *testing.T) {
	t.Run("refuse deletion with single link", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		_, linkErr := ctx.exec("link", idA, idB)
		if linkErr != nil {
			t.Fatalf("link command failed: %v", linkErr)
		}

		// Verify link was created
		ticketA, _ := ctx.store().Get(idA)
		if len(ticketA.Links) == 0 {
			t.Fatalf("ticket A should have links after link command, got: %v", ticketA.Links)
		}

		// Try to delete A without --force
		_, err := ctx.exec("rm", idA)
		if err == nil {
			t.Error("expected error when deleting ticket with links")
		}

		// Verify error message
		if !strings.Contains(err.Error(), "links") {
			t.Errorf("error should mention 'links', got: %v", err)
		}
		if !strings.Contains(err.Error(), "--force") {
			t.Errorf("error should mention '--force' flag, got: %v", err)
		}
		if !strings.Contains(err.Error(), idB) {
			t.Errorf("error should list linked ticket %s, got: %v", idB, err)
		}

		// Verify A still exists
		_, err = ctx.store().Get(idA)
		if err != nil {
			t.Error("ticket should still exist after refused deletion")
		}
	})

	t.Run("refuse deletion with multiple links", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)
		idC, _ := ctx.exec("new", "Ticket C")
		idC = strings.TrimSpace(idC)

		ctx.exec("link", idA, idB, idC)

		// Try to delete A
		_, err := ctx.exec("rm", idA)
		if err == nil {
			t.Error("expected error when deleting ticket with links")
		}

		// Verify error lists both B and C
		if !strings.Contains(err.Error(), idB) {
			t.Errorf("error should list %s", idB)
		}
		if !strings.Contains(err.Error(), idC) {
			t.Errorf("error should list %s", idC)
		}
	})
}

// TestRmForceWithLinks - Force deletion cleanup
func TestRmForceWithLinks(t *testing.T) {
	t.Run("force deletes and removes symmetric links", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		ctx.exec("link", idA, idB)

		// Delete A with --force
		output, err := ctx.exec("rm", "--force", idA)
		if err != nil {
			t.Fatalf("rm --force should succeed: %v", err)
		}

		// Verify success message mentions link removal
		if !strings.Contains(output, "Removed") && !strings.Contains(output, "link") {
			t.Errorf("output should mention link removal, got: %s", output)
		}
		if !strings.Contains(output, "1 link(s)") {
			t.Errorf("output should mention '1 link(s)', got: %s", output)
		}

		// Verify A is deleted
		_, err = ctx.store().Get(idA)
		if err == nil {
			t.Error("ticket A should be deleted")
		}

		// Verify B.Links is empty (symmetric cleanup)
		ticketB, err := ctx.store().Get(idB)
		if err != nil {
			t.Fatalf("failed to get ticket B: %v", err)
		}
		if len(ticketB.Links) != 0 {
			t.Errorf("ticket B should have 0 links, got %d: %v", len(ticketB.Links), ticketB.Links)
		}
	})

	t.Run("force with multiple links removes all", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create A, B, C and link all three
		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)
		idC, _ := ctx.exec("new", "Ticket C")
		idC = strings.TrimSpace(idC)

		ctx.exec("link", idA, idB, idC)

		// Delete A with --force
		output, err := ctx.exec("rm", "--force", idA)
		if err != nil {
			t.Fatalf("rm --force should succeed: %v", err)
		}

		if !strings.Contains(output, "2 link(s)") {
			t.Errorf("should mention removing 2 links, got: %s", output)
		}

		// Verify A is deleted
		_, err = ctx.store().Get(idA)
		if err == nil {
			t.Error("ticket A should be deleted")
		}

		// Verify B.Links = [C] (A removed, still linked to C)
		ticketB, _ := ctx.store().Get(idB)
		if len(ticketB.Links) != 1 {
			t.Errorf("ticket B should have 1 link, got %d: %v", len(ticketB.Links), ticketB.Links)
		}
		if len(ticketB.Links) > 0 && ticketB.Links[0] != idC {
			t.Errorf("ticket B should be linked to C, got: %v", ticketB.Links)
		}

		// Verify C.Links = [B] (symmetric)
		ticketC, _ := ctx.store().Get(idC)
		if len(ticketC.Links) != 1 {
			t.Errorf("ticket C should have 1 link, got %d: %v", len(ticketC.Links), ticketC.Links)
		}
		if len(ticketC.Links) > 0 && ticketC.Links[0] != idB {
			t.Errorf("ticket C should be linked to B, got: %v", ticketC.Links)
		}
	})

	t.Run("force with orphaned link ID", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		ctx.exec("link", idA, idB)

		// Manually delete B's file to create orphaned link
		path := filepath.Join(ctx.ticketsDir, idB+".md")
		os.Remove(path)

		// Delete A with --force should succeed (skip orphaned link)
		output, err := ctx.exec("rm", "--force", idA)
		if err != nil {
			t.Fatalf("rm --force should succeed even with orphaned link: %v", err)
		}

		if !strings.Contains(output, "Deleted ticket:") {
			t.Errorf("should show success message, got: %s", output)
		}

		// Verify A is deleted
		_, err = ctx.store().Get(idA)
		if err == nil {
			t.Error("ticket A should be deleted")
		}
	})
}

// TestRmNoDanglingReferences - Critical: no dangling pointers
func TestRmNoDanglingReferences(t *testing.T) {
	t.Run("force removes all link references", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create A, B, C; link A-B and A-C
		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)
		idC, _ := ctx.exec("new", "Ticket C")
		idC = strings.TrimSpace(idC)

		ctx.exec("link", idA, idB)
		ctx.exec("link", idA, idC)

		// Delete A with --force
		_, err := ctx.exec("rm", "--force", idA)
		if err != nil {
			t.Fatalf("rm --force failed: %v", err)
		}

		// Verify B doesn't contain A
		ticketB, _ := ctx.store().Get(idB)
		for _, linkID := range ticketB.Links {
			if linkID == idA {
				t.Error("ticket B should not have link to deleted ticket A")
			}
		}

		// Verify C doesn't contain A
		ticketC, _ := ctx.store().Get(idC)
		for _, linkID := range ticketC.Links {
			if linkID == idA {
				t.Error("ticket C should not have link to deleted ticket A")
			}
		}

		// List all tickets and verify no references to A
		allTickets, _ := ctx.store().List()
		for _, tkt := range allTickets {
			// Check deps
			for _, depID := range tkt.Deps {
				if depID == idA {
					t.Errorf("ticket %s still has deleted ticket %s in deps", tkt.ID, idA)
				}
			}
			// Check links
			for _, linkID := range tkt.Links {
				if linkID == idA {
					t.Errorf("ticket %s still has deleted ticket %s in links", tkt.ID, idA)
				}
			}
		}
	})

	t.Run("links remain symmetric after deletion", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create A, B, C and link all three
		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)
		idC, _ := ctx.exec("new", "Ticket C")
		idC = strings.TrimSpace(idC)

		ctx.exec("link", idA, idB, idC)

		// Delete A with --force
		_, err := ctx.exec("rm", "--force", idA)
		if err != nil {
			t.Fatalf("rm --force failed: %v", err)
		}

		// Verify B.Links = [C] and C.Links = [B] (symmetric)
		ticketB, _ := ctx.store().Get(idB)
		ticketC, _ := ctx.store().Get(idC)

		if len(ticketB.Links) != 1 || ticketB.Links[0] != idC {
			t.Errorf("ticket B should be linked only to C, got: %v", ticketB.Links)
		}

		if len(ticketC.Links) != 1 || ticketC.Links[0] != idB {
			t.Errorf("ticket C should be linked only to B, got: %v", ticketC.Links)
		}
	})

	t.Run("no dangling deps after deletion", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create A and B, B depends on A
		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		ctx.exec("dep", idB, idA)

		// Try to delete A (should refuse)
		_, err := ctx.exec("rm", idA)
		if err == nil {
			t.Error("should refuse to delete ticket with dependants")
		}

		// Verify B.Deps still contains A (not cleaned up)
		ticketB, _ := ctx.store().Get(idB)
		found := false
		for _, depID := range ticketB.Deps {
			if depID == idA {
				found = true
				break
			}
		}
		if !found {
			t.Error("ticket B should still have A in deps after refused deletion")
		}
	})
}

// TestRmPersistence - Changes written to disk
func TestRmPersistence(t *testing.T) {
	t.Run("deletion persisted", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)

		// Delete it
		ctx.exec("rm", idA)

		// Create new FileStore and verify A is not in List()
		newStore := ticket.NewFileStore(ctx.ticketsDir)
		allTickets, err := newStore.List()
		if err != nil {
			t.Fatalf("failed to list tickets: %v", err)
		}

		for _, tkt := range allTickets {
			if tkt.ID == idA {
				t.Error("deleted ticket should not appear in new store")
			}
		}

		// Verify file doesn't exist on filesystem
		path := filepath.Join(ctx.ticketsDir, idA+".md")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Error("ticket file should not exist on filesystem")
		}
	})

	t.Run("link cleanup persisted", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		ctx.exec("link", idA, idB)
		ctx.exec("rm", "--force", idA)

		// Create new FileStore and load B
		newStore := ticket.NewFileStore(ctx.ticketsDir)
		reloadedB, err := newStore.Get(idB)
		if err != nil {
			t.Fatalf("failed to reload ticket B: %v", err)
		}

		// Verify B.Links is empty
		if len(reloadedB.Links) != 0 {
			t.Errorf("ticket B should have 0 links after persistence reload, got %d: %v",
				len(reloadedB.Links), reloadedB.Links)
		}
	})
}

// TestRmBlockerPrecedence - Error priority
func TestRmBlockerPrecedence(t *testing.T) {
	t.Run("dependants checked before links", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create A, B; B depends on A; also link them
		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		ctx.exec("dep", idB, idA)
		ctx.exec("link", idA, idB)

		// Try to delete A with --force
		_, err := ctx.exec("rm", "--force", idA)
		if err == nil {
			t.Error("should refuse deletion due to dependants")
		}

		// Error should mention dependants, not links
		if !strings.Contains(err.Error(), "dependants") {
			t.Errorf("error should mention 'dependants' (priority over links), got: %v", err)
		}
		if strings.Contains(err.Error(), "links") && !strings.Contains(err.Error(), "dependants") {
			t.Errorf("error should prioritize dependants over links, got: %v", err)
		}
	})

	t.Run("children checked before links", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create parent P, child C; also link them
		idP, _ := ctx.exec("new", "Parent ticket")
		idP = strings.TrimSpace(idP)
		idC, _ := ctx.exec("new", "Child ticket", "--parent", idP)
		idC = strings.TrimSpace(idC)

		ctx.exec("link", idP, idC)

		// Try to delete P with --force
		_, err := ctx.exec("rm", "--force", idP)
		if err == nil {
			t.Error("should refuse deletion due to children")
		}

		// Error should mention children, not links
		if !strings.Contains(err.Error(), "children") {
			t.Errorf("error should mention 'children' (priority over links), got: %v", err)
		}
	})
}

// TestRmOutput - Success messages
func TestRmOutput(t *testing.T) {
	t.Run("basic deletion message", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)

		output, err := ctx.exec("rm", idA)
		if err != nil {
			t.Fatalf("rm failed: %v", err)
		}

		// Format: "Deleted ticket: {id}"
		if !strings.Contains(output, "Deleted ticket:") {
			t.Errorf("output should contain 'Deleted ticket:', got: %s", output)
		}
		if !strings.Contains(output, idA) {
			t.Errorf("output should contain ticket ID, got: %s", output)
		}
	})

	t.Run("force deletion with links message", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		ctx.exec("link", idA, idB)

		output, err := ctx.exec("rm", "--force", idA)
		if err != nil {
			t.Fatalf("rm --force failed: %v", err)
		}

		// Format: "Removed 1 link(s) and deleted ticket: {id}"
		if !strings.Contains(output, "Removed") {
			t.Errorf("output should contain 'Removed', got: %s", output)
		}
		if !strings.Contains(output, "1 link(s)") {
			t.Errorf("output should contain '1 link(s)', got: %s", output)
		}
		if !strings.Contains(output, "deleted ticket:") {
			t.Errorf("output should contain 'deleted ticket:', got: %s", output)
		}
		if !strings.Contains(output, idA) {
			t.Errorf("output should contain ticket ID, got: %s", output)
		}
	})

	t.Run("error messages list blocking tickets", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create A and B, B depends on A
		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		ctx.exec("dep", idB, idA)

		// Try to delete A
		_, err := ctx.exec("rm", idA)
		if err == nil {
			t.Fatal("expected error")
		}

		// Format: "- {id} [{status}] {title}"
		errMsg := err.Error()
		if !strings.Contains(errMsg, "- "+idB) {
			t.Errorf("error should list ticket with '- {id}' format, got: %v", err)
		}
		if !strings.Contains(errMsg, "[open]") {
			t.Errorf("error should show status in brackets, got: %v", err)
		}
		if !strings.Contains(errMsg, "Ticket B") {
			t.Errorf("error should show ticket title, got: %v", err)
		}
	})
}
