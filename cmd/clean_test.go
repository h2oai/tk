package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lo5/tk/internal/ticket"
)

// TestCleanNoClosedTickets - No closed tickets found
func TestCleanNoClosedTickets(t *testing.T) {
	t.Run("no tickets at all", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "No closed tickets found") {
			t.Errorf("expected 'No closed tickets found', got: %s", output)
		}
	})

	t.Run("only open tickets", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create some open tickets
		ctx.exec("new", "Open ticket 1")
		ctx.exec("new", "Open ticket 2")

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "No closed tickets found") {
			t.Errorf("expected 'No closed tickets found', got: %s", output)
		}
	})

	t.Run("only in_progress tickets", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create some in_progress tickets
		id1, _ := ctx.exec("new", "In progress 1")
		id1 = strings.TrimSpace(id1)
		ctx.exec("start", id1)

		id2, _ := ctx.exec("new", "In progress 2")
		id2 = strings.TrimSpace(id2)
		ctx.exec("start", id2)

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "No closed tickets found") {
			t.Errorf("expected 'No closed tickets found', got: %s", output)
		}
	})
}

// TestCleanAllClosedDeletable - All closed tickets are safe to delete
func TestCleanAllClosedDeletable(t *testing.T) {
	t.Run("single closed ticket", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create and close a ticket
		id, _ := ctx.exec("new", "Closed ticket")
		id = strings.TrimSpace(id)
		ctx.exec("close", id)

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "Found 1 closed ticket(s)") {
			t.Errorf("expected 'Found 1 closed ticket(s)', got: %s", output)
		}
		if !strings.Contains(output, "1 deletable") {
			t.Errorf("expected '1 deletable', got: %s", output)
		}
		if !strings.Contains(output, "0 blocked") {
			t.Errorf("expected '0 blocked', got: %s", output)
		}
		if !strings.Contains(output, "Run with --fix to delete 1 deletable ticket(s)") {
			t.Errorf("expected fix prompt, got: %s", output)
		}
	})

	t.Run("multiple closed tickets", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create and close multiple tickets
		for i := 1; i <= 3; i++ {
			id, _ := ctx.exec("new", "Closed ticket")
			id = strings.TrimSpace(id)
			ctx.exec("close", id)
		}

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "Found 3 closed ticket(s)") {
			t.Errorf("expected 'Found 3 closed ticket(s)', got: %s", output)
		}
		if !strings.Contains(output, "3 deletable") {
			t.Errorf("expected '3 deletable', got: %s", output)
		}
		if !strings.Contains(output, "0 blocked") {
			t.Errorf("expected '0 blocked', got: %s", output)
		}
	})
}

// TestCleanPartialDeletable - Mix of deletable and blocked closed tickets
func TestCleanPartialDeletable(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Create closed ticket with no relationships (deletable)
	idA, _ := ctx.exec("new", "Deletable closed")
	idA = strings.TrimSpace(idA)
	ctx.exec("close", idA)

	// Create closed ticket with dependant (blocked)
	idB, _ := ctx.exec("new", "Blocked closed")
	idB = strings.TrimSpace(idB)
	ctx.exec("close", idB)

	idC, _ := ctx.exec("new", "Dependant")
	idC = strings.TrimSpace(idC)
	ctx.exec("dep", idC, idB)

	output, err := ctx.exec("clean")
	if err != nil {
		t.Fatalf("clean command error: %v", err)
	}

	if !strings.Contains(output, "Found 2 closed ticket(s)") {
		t.Errorf("expected 'Found 2 closed ticket(s)', got: %s", output)
	}
	if !strings.Contains(output, "1 deletable") {
		t.Errorf("expected '1 deletable', got: %s", output)
	}
	if !strings.Contains(output, "1 blocked") {
		t.Errorf("expected '1 blocked', got: %s", output)
	}
	if !strings.Contains(output, "Blocked tickets:") {
		t.Errorf("expected 'Blocked tickets:' section, got: %s", output)
	}
	if !strings.Contains(output, idB) {
		t.Errorf("expected blocked ticket ID %s, got: %s", idB, output)
	}
	if !strings.Contains(output, "has dependants") {
		t.Errorf("expected 'has dependants' reason, got: %s", output)
	}
}

// TestCleanRefuseWithDependants - Closed ticket has dependants (any status)
func TestCleanRefuseWithDependants(t *testing.T) {
	t.Run("closed ticket with open dependant", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create closed ticket A
		idA, _ := ctx.exec("new", "Closed ticket")
		idA = strings.TrimSpace(idA)
		ctx.exec("close", idA)

		// Create open ticket B that depends on A
		idB, _ := ctx.exec("new", "Open dependant")
		idB = strings.TrimSpace(idB)
		ctx.exec("dep", idB, idA)

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "1 blocked") {
			t.Errorf("expected '1 blocked', got: %s", output)
		}
		if !strings.Contains(output, "has dependants") {
			t.Errorf("expected 'has dependants', got: %s", output)
		}
	})

	t.Run("closed ticket whose closed dependant is itself blocked", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create closed ticket A
		idA, _ := ctx.exec("new", "Closed ticket")
		idA = strings.TrimSpace(idA)
		ctx.exec("close", idA)

		// Create closed ticket B that depends on A
		idB, _ := ctx.exec("new", "Closed dependant")
		idB = strings.TrimSpace(idB)
		ctx.exec("dep", idB, idA)
		ctx.exec("close", idB)

		// Create open ticket C that depends on B, pinning B in place
		idC, _ := ctx.exec("new", "Open dependant")
		idC = strings.TrimSpace(idC)
		ctx.exec("dep", idC, idB)

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		// B is blocked by open C, so A is blocked by surviving B
		if !strings.Contains(output, "Found 2 closed ticket(s)") {
			t.Errorf("expected 'Found 2 closed ticket(s)', got: %s", output)
		}
		if !strings.Contains(output, "0 deletable") {
			t.Errorf("expected '0 deletable', got: %s", output)
		}
		if !strings.Contains(output, "2 blocked") {
			t.Errorf("expected '2 blocked', got: %s", output)
		}
		if !strings.Contains(output, "has dependants") {
			t.Errorf("expected 'has dependants', got: %s", output)
		}
	})

	t.Run("closed ticket with multiple dependants", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create closed ticket A
		idA, _ := ctx.exec("new", "Closed ticket")
		idA = strings.TrimSpace(idA)
		ctx.exec("close", idA)

		// Create multiple tickets that depend on A
		idB, _ := ctx.exec("new", "Dependant 1")
		idB = strings.TrimSpace(idB)
		ctx.exec("dep", idB, idA)

		idC, _ := ctx.exec("new", "Dependant 2")
		idC = strings.TrimSpace(idC)
		ctx.exec("dep", idC, idA)

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "1 blocked") {
			t.Errorf("expected '1 blocked', got: %s", output)
		}
		if !strings.Contains(output, "has dependants") {
			t.Errorf("expected 'has dependants', got: %s", output)
		}
	})
}

// TestCleanRefuseWithChildren - Closed ticket has children
func TestCleanRefuseWithChildren(t *testing.T) {
	t.Run("closed ticket with single child", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create closed parent ticket
		idParent, _ := ctx.exec("new", "Closed parent")
		idParent = strings.TrimSpace(idParent)
		ctx.exec("close", idParent)

		// Create child ticket
		idChild, _ := ctx.exec("new", "--parent", idParent, "Child ticket")
		idChild = strings.TrimSpace(idChild)

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "1 blocked") {
			t.Errorf("expected '1 blocked', got: %s", output)
		}
		if !strings.Contains(output, "has non-closed children") {
			t.Errorf("expected 'has non-closed children', got: %s", output)
		}
	})

	t.Run("closed ticket with multiple children", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create closed parent ticket
		idParent, _ := ctx.exec("new", "Closed parent")
		idParent = strings.TrimSpace(idParent)
		ctx.exec("close", idParent)

		// Create multiple children
		ctx.exec("new", "--parent", idParent, "Child 1")
		ctx.exec("new", "--parent", idParent, "Child 2")

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "1 blocked") {
			t.Errorf("expected '1 blocked', got: %s", output)
		}
		if !strings.Contains(output, "has non-closed children") {
			t.Errorf("expected 'has non-closed children', got: %s", output)
		}
	})
}

// TestCleanClosedParentWithAllClosedChildren - Closed parent with all closed children should be deletable
func TestCleanClosedParentWithAllClosedChildren(t *testing.T) {
	t.Run("closed parent with single closed child", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create closed parent ticket
		idParent, _ := ctx.exec("new", "Closed parent")
		idParent = strings.TrimSpace(idParent)
		ctx.exec("close", idParent)

		// Create closed child
		idChild, _ := ctx.exec("new", "--parent", idParent, "Closed child")
		idChild = strings.TrimSpace(idChild)
		ctx.exec("close", idChild)

		// Both tickets are closed - both should be deletable
		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "Found 2 closed ticket(s)") {
			t.Errorf("expected 'Found 2 closed ticket(s)', got: %s", output)
		}
		if !strings.Contains(output, "2 deletable") {
			t.Errorf("expected '2 deletable', got: %s", output)
		}
		if !strings.Contains(output, "0 blocked") {
			t.Errorf("expected '0 blocked', got: %s", output)
		}
	})

	t.Run("closed parent with multiple closed children", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create closed parent ticket
		idParent, _ := ctx.exec("new", "Closed parent")
		idParent = strings.TrimSpace(idParent)
		ctx.exec("close", idParent)

		// Create multiple closed children
		idChild1, _ := ctx.exec("new", "--parent", idParent, "Closed child 1")
		idChild1 = strings.TrimSpace(idChild1)
		ctx.exec("close", idChild1)

		idChild2, _ := ctx.exec("new", "--parent", idParent, "Closed child 2")
		idChild2 = strings.TrimSpace(idChild2)
		ctx.exec("close", idChild2)

		// All tickets are closed - all should be deletable
		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "Found 3 closed ticket(s)") {
			t.Errorf("expected 'Found 3 closed ticket(s)', got: %s", output)
		}
		if !strings.Contains(output, "3 deletable") {
			t.Errorf("expected '3 deletable', got: %s", output)
		}
		if !strings.Contains(output, "0 blocked") {
			t.Errorf("expected '0 blocked', got: %s", output)
		}
	})
}

// TestCleanClosedParentWithMixedStatusChildren - Closed parent with mixed status children
func TestCleanClosedParentWithMixedStatusChildren(t *testing.T) {
	t.Run("closed parent with closed and open children", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create closed parent ticket
		idParent, _ := ctx.exec("new", "Closed parent")
		idParent = strings.TrimSpace(idParent)
		ctx.exec("close", idParent)

		// Create one closed child
		idClosedChild, _ := ctx.exec("new", "--parent", idParent, "Closed child")
		idClosedChild = strings.TrimSpace(idClosedChild)
		ctx.exec("close", idClosedChild)

		// Create one open child
		idOpenChild, _ := ctx.exec("new", "--parent", idParent, "Open child")
		idOpenChild = strings.TrimSpace(idOpenChild)

		// Parent should be blocked due to open child
		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "Found 2 closed ticket(s)") {
			t.Errorf("expected 'Found 2 closed ticket(s)' (parent + closed child), got: %s", output)
		}
		if !strings.Contains(output, "1 deletable") {
			t.Errorf("expected '1 deletable' (closed child only), got: %s", output)
		}
		if !strings.Contains(output, "1 blocked") {
			t.Errorf("expected '1 blocked' (parent), got: %s", output)
		}
		if !strings.Contains(output, "has non-closed children") {
			t.Errorf("expected 'has non-closed children', got: %s", output)
		}
	})

	t.Run("closed parent with closed and in_progress children", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create closed parent ticket
		idParent, _ := ctx.exec("new", "Closed parent")
		idParent = strings.TrimSpace(idParent)
		ctx.exec("close", idParent)

		// Create one closed child
		idClosedChild, _ := ctx.exec("new", "--parent", idParent, "Closed child")
		idClosedChild = strings.TrimSpace(idClosedChild)
		ctx.exec("close", idClosedChild)

		// Create one in_progress child
		idInProgressChild, _ := ctx.exec("new", "--parent", idParent, "In progress child")
		idInProgressChild = strings.TrimSpace(idInProgressChild)
		ctx.exec("start", idInProgressChild)

		// Parent should be blocked due to in_progress child
		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean --fix command error: %v", err)
		}

		if !strings.Contains(output, "Found 2 closed ticket(s)") {
			t.Errorf("expected 'Found 2 closed ticket(s)' (parent + closed child), got: %s", output)
		}
		if !strings.Contains(output, "1 deletable") {
			t.Errorf("expected '1 deletable' (closed child only), got: %s", output)
		}
		if !strings.Contains(output, "1 blocked") {
			t.Errorf("expected '1 blocked' (parent), got: %s", output)
		}
		if !strings.Contains(output, "has non-closed children") {
			t.Errorf("expected 'has non-closed children', got: %s", output)
		}
	})
}

// TestCleanRefuseWithLinks - Closed ticket has bidirectional links
func TestCleanRefuseWithLinks(t *testing.T) {
	t.Run("closed ticket with single link", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create two tickets and link them
		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		ctx.exec("close", idA)

		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		ctx.exec("link", idA, idB)

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "1 blocked") {
			t.Errorf("expected '1 blocked', got: %s", output)
		}
		if !strings.Contains(output, "has links") {
			t.Errorf("expected 'has links', got: %s", output)
		}
	})

	t.Run("closed ticket with multiple links", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create closed ticket
		idA, _ := ctx.exec("new", "Closed ticket")
		idA = strings.TrimSpace(idA)
		ctx.exec("close", idA)

		// Create and link multiple tickets
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)
		ctx.exec("link", idA, idB)

		idC, _ := ctx.exec("new", "Ticket C")
		idC = strings.TrimSpace(idC)
		ctx.exec("link", idA, idC)

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "1 blocked") {
			t.Errorf("expected '1 blocked', got: %s", output)
		}
		if !strings.Contains(output, "has links") {
			t.Errorf("expected 'has links', got: %s", output)
		}
	})
}

// TestCleanTransitiveLinks - Links are resolved transitively, like deps/children
func TestCleanTransitiveLinks(t *testing.T) {
	t.Run("mutually linked closed tickets are deletable together", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		ctx.exec("link", idA, idB)
		ctx.exec("close", idA)
		ctx.exec("close", idB)

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "2 deletable") {
			t.Errorf("expected '2 deletable', got: %s", output)
		}
		if !strings.Contains(output, "0 blocked") {
			t.Errorf("expected '0 blocked', got: %s", output)
		}

		output, err = ctx.exec("clean", "--fix")
		if err != nil {
			t.Fatalf("clean --fix command error: %v", err)
		}
		if !strings.Contains(output, "Deleted 2 ticket(s)") {
			t.Errorf("expected 'Deleted 2 ticket(s)', got: %s", output)
		}

		if _, err := ctx.store().Get(idA); err == nil {
			t.Errorf("ticket %s should be deleted", idA)
		}
		if _, err := ctx.store().Get(idB); err == nil {
			t.Errorf("ticket %s should be deleted", idB)
		}
	})

	t.Run("mutually linked closed tickets with a dependency between them", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Matches the real-world case: A and B link to each other, and B
		// also depends on A. Both are closed and otherwise unblocked.
		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		ctx.exec("link", idA, idB)
		ctx.exec("dep", idB, idA)
		ctx.exec("close", idA)
		ctx.exec("close", idB)

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "2 deletable") {
			t.Errorf("expected '2 deletable', got: %s", output)
		}
		if !strings.Contains(output, "0 blocked") {
			t.Errorf("expected '0 blocked', got: %s", output)
		}
	})

	t.Run("link to a closed-but-blocked ticket still blocks", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// A links to B. B is closed but has a non-closed child C, so B is
		// blocked on its own -- A should stay blocked too, since B is not
		// actually being deleted this run.
		idA, _ := ctx.exec("new", "Ticket A")
		idA = strings.TrimSpace(idA)
		idB, _ := ctx.exec("new", "Ticket B")
		idB = strings.TrimSpace(idB)

		ctx.exec("link", idA, idB)
		ctx.exec("close", idA)
		ctx.exec("close", idB)

		ctx.exec("new", "--parent", idB, "Child of B")

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "0 deletable") {
			t.Errorf("expected '0 deletable', got: %s", output)
		}
		if !strings.Contains(output, "2 blocked") {
			t.Errorf("expected '2 blocked', got: %s", output)
		}
		if !strings.Contains(output, idB+" [closed]") || !strings.Contains(output, "has non-closed children") {
			t.Errorf("expected %s blocked with 'has non-closed children', got: %s", idB, output)
		}
		if !strings.Contains(output, idA+" [closed]") || !strings.Contains(output, "has links") {
			t.Errorf("expected %s blocked with 'has links', got: %s", idA, output)
		}
	})
}

// TestCleanDryRunNoChanges - Verify dry-run doesn't delete anything
func TestCleanDryRunNoChanges(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Create some closed deletable tickets
	var ids []string
	for i := 1; i <= 3; i++ {
		id, _ := ctx.exec("new", "Deletable ticket")
		id = strings.TrimSpace(id)
		ctx.exec("close", id)
		ids = append(ids, id)
	}

	// Run clean without --fix (dry-run)
	output, err := ctx.exec("clean")
	if err != nil {
		t.Fatalf("clean command error: %v", err)
	}

	if !strings.Contains(output, "Run with --fix") {
		t.Errorf("expected dry-run prompt, got: %s", output)
	}

	// Verify all tickets still exist
	for _, id := range ids {
		ticket, err := ctx.store().Get(id)
		if err != nil {
			t.Errorf("ticket %s should still exist after dry-run: %v", id, err)
		}
		if ticket == nil {
			t.Errorf("ticket %s should not be nil", id)
		}
	}

	// Verify file existence
	for _, id := range ids {
		path := filepath.Join(ctx.ticketsDir, id+".md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("ticket file %s should still exist after dry-run", path)
		}
	}
}

// TestCleanFixDeletes - Verify --fix actually deletes tickets
func TestCleanFixDeletes(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Create some closed deletable tickets
	var ids []string
	for i := 1; i <= 3; i++ {
		id, _ := ctx.exec("new", "Deletable ticket")
		id = strings.TrimSpace(id)
		ctx.exec("close", id)
		ids = append(ids, id)
	}

	// Run clean with --fix
	output, err := ctx.exec("clean", "--fix")
	if err != nil {
		t.Fatalf("clean --fix command error: %v", err)
	}

	if !strings.Contains(output, "Deleting closed tickets") {
		t.Errorf("expected 'Deleting closed tickets' header, got: %s", output)
	}

	// Verify all "Deleted: <id>" messages
	for _, id := range ids {
		if !strings.Contains(output, "Deleted: "+id) {
			t.Errorf("expected 'Deleted: %s' in output, got: %s", id, output)
		}
	}

	if !strings.Contains(output, "Deleted 3 ticket(s)") {
		t.Errorf("expected 'Deleted 3 ticket(s)' summary, got: %s", output)
	}

	// Verify all tickets are gone
	for _, id := range ids {
		_, err := ctx.store().Get(id)
		if err == nil {
			t.Errorf("ticket %s should be deleted", id)
		}

		// Verify file doesn't exist
		path := filepath.Join(ctx.ticketsDir, id+".md")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("ticket file %s should be deleted", path)
		}
	}
}

// TestCleanFixSkipsBlocked - Verify blocked tickets are skipped
func TestCleanFixSkipsBlocked(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Create deletable closed ticket
	idDeletable, _ := ctx.exec("new", "Deletable")
	idDeletable = strings.TrimSpace(idDeletable)
	ctx.exec("close", idDeletable)

	// Create blocked closed ticket (has dependant)
	idBlocked, _ := ctx.exec("new", "Blocked")
	idBlocked = strings.TrimSpace(idBlocked)
	ctx.exec("close", idBlocked)

	idDependant, _ := ctx.exec("new", "Dependant")
	idDependant = strings.TrimSpace(idDependant)
	ctx.exec("dep", idDependant, idBlocked)

	// Run clean with --fix
	output, err := ctx.exec("clean", "--fix")
	if err != nil {
		t.Fatalf("clean --fix command error: %v", err)
	}

	// Verify deletable was deleted
	if !strings.Contains(output, "Deleted: "+idDeletable) {
		t.Errorf("expected 'Deleted: %s', got: %s", idDeletable, output)
	}

	// Verify blocked was skipped
	if strings.Contains(output, "Deleted: "+idBlocked) {
		t.Errorf("blocked ticket %s should not be deleted, got: %s", idBlocked, output)
	}

	if !strings.Contains(output, "Deleted 1 ticket(s), skipped 1 blocked ticket(s)") {
		t.Errorf("expected correct summary, got: %s", output)
	}

	// Verify deletable is gone
	_, err = ctx.store().Get(idDeletable)
	if err == nil {
		t.Errorf("ticket %s should be deleted", idDeletable)
	}

	// Verify blocked still exists
	ticket, err := ctx.store().Get(idBlocked)
	if err != nil {
		t.Errorf("blocked ticket %s should still exist: %v", idBlocked, err)
	}
	if ticket == nil {
		t.Errorf("blocked ticket %s should not be nil", idBlocked)
	}
}

// TestCleanMixedStatuses - Only closed tickets deleted, not open/in_progress
func TestCleanMixedStatuses(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Create tickets with different statuses
	idOpen, _ := ctx.exec("new", "Open ticket")
	idOpen = strings.TrimSpace(idOpen)

	idInProgress, _ := ctx.exec("new", "In progress ticket")
	idInProgress = strings.TrimSpace(idInProgress)
	ctx.exec("start", idInProgress)

	idClosed, _ := ctx.exec("new", "Closed ticket")
	idClosed = strings.TrimSpace(idClosed)
	ctx.exec("close", idClosed)

	// Run clean with --fix
	output, err := ctx.exec("clean", "--fix")
	if err != nil {
		t.Fatalf("clean --fix command error: %v", err)
	}

	// Verify only closed ticket was deleted
	if !strings.Contains(output, "Deleted: "+idClosed) {
		t.Errorf("expected closed ticket %s to be deleted, got: %s", idClosed, output)
	}

	if strings.Contains(output, idOpen) {
		t.Errorf("open ticket %s should not be mentioned, got: %s", idOpen, output)
	}

	if strings.Contains(output, idInProgress) {
		t.Errorf("in_progress ticket %s should not be mentioned, got: %s", idInProgress, output)
	}

	// Verify open and in_progress still exist
	ticketOpen, err := ctx.store().Get(idOpen)
	if err != nil || ticketOpen == nil {
		t.Errorf("open ticket %s should still exist", idOpen)
	}

	ticketInProgress, err := ctx.store().Get(idInProgress)
	if err != nil || ticketInProgress == nil {
		t.Errorf("in_progress ticket %s should still exist", idInProgress)
	}

	// Verify closed is gone
	_, err = ctx.store().Get(idClosed)
	if err == nil {
		t.Errorf("closed ticket %s should be deleted", idClosed)
	}
}

// TestCleanPersistence - Verify deletions persist to disk
func TestCleanPersistence(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Create closed tickets
	var ids []string
	for i := 1; i <= 2; i++ {
		id, _ := ctx.exec("new", "Ticket to delete")
		id = strings.TrimSpace(id)
		ctx.exec("close", id)
		ids = append(ids, id)
	}

	// Run clean with --fix
	_, err := ctx.exec("clean", "--fix")
	if err != nil {
		t.Fatalf("clean --fix command error: %v", err)
	}

	// Create new store instance to verify persistence
	newStore := ticket.NewFileStore(ctx.ticketsDir)

	// Verify tickets are deleted
	for _, id := range ids {
		_, err := newStore.Get(id)
		if err == nil {
			t.Errorf("ticket %s should be deleted in new store", id)
		}
	}

	// Verify not in list
	allTickets, _ := newStore.List()
	for _, tkt := range allTickets {
		for _, id := range ids {
			if tkt.ID == id {
				t.Errorf("deleted ticket %s should not appear in list", id)
			}
		}
	}
}

// TestCleanNoDanglingRefs - Verify no dangling references created
func TestCleanNoDanglingRefs(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Create a closed ticket
	idA, _ := ctx.exec("new", "Closed ticket")
	idA = strings.TrimSpace(idA)
	ctx.exec("close", idA)

	// Create another ticket that depends on A
	idB, _ := ctx.exec("new", "Dependant")
	idB = strings.TrimSpace(idB)
	ctx.exec("dep", idB, idA)

	// Run clean with --fix (should skip A because B depends on it)
	_, err := ctx.exec("clean", "--fix")
	if err != nil {
		t.Fatalf("clean --fix command error: %v", err)
	}

	// Verify A still exists (was blocked)
	ticketA, err := ctx.store().Get(idA)
	if err != nil {
		t.Errorf("blocked ticket %s should still exist: %v", idA, err)
	}
	if ticketA == nil {
		t.Errorf("blocked ticket %s should not be nil", idA)
	}

	// Verify B still has valid dependency
	ticketB, err := ctx.store().Get(idB)
	if err != nil {
		t.Fatalf("ticket B should exist: %v", err)
	}

	hasDepA := false
	for _, dep := range ticketB.Deps {
		if dep == idA {
			hasDepA = true
			break
		}
	}
	if !hasDepA {
		t.Errorf("ticket B should still have dependency on A")
	}

	// Verify the dependency is valid (A exists)
	_, err = ctx.store().Get(idA)
	if err != nil {
		t.Errorf("dependency target %s should exist: %v", idA, err)
	}
}

// TestCleanNoArgs - Verify clean takes no arguments
func TestCleanNoArgs(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	_, err := ctx.exec("clean", "extra-arg")
	if err == nil {
		t.Error("clean should error with extra arguments")
	}
}

// TestCleanFixWithNoClosedTickets - --fix with no closed tickets
func TestCleanFixWithNoClosedTickets(t *testing.T) {
	t.Run("no tickets at all", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		output, err := ctx.exec("clean", "--fix")
		if err != nil {
			t.Fatalf("clean --fix command error: %v", err)
		}

		if !strings.Contains(output, "No closed tickets found") {
			t.Errorf("expected 'No closed tickets found', got: %s", output)
		}
	})

	t.Run("all closed tickets blocked", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create closed ticket with child (blocked)
		idParent, _ := ctx.exec("new", "Parent")
		idParent = strings.TrimSpace(idParent)
		ctx.exec("close", idParent)

		ctx.exec("new", "--parent", idParent, "Child")

		output, err := ctx.exec("clean", "--fix")
		if err != nil {
			t.Fatalf("clean --fix command error: %v", err)
		}

		if !strings.Contains(output, "No deletable tickets") {
			t.Errorf("expected 'No deletable tickets', got: %s", output)
		}
		if !strings.Contains(output, "1 closed ticket(s) are blocked") {
			t.Errorf("expected 'blocked' message, got: %s", output)
		}
	})
}

// TestCleanClosedDependsOnClosed - Closed ticket depends on another closed ticket
func TestCleanClosedDependsOnClosed(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Create ticket A, close it
	idA, _ := ctx.exec("new", "Ticket A")
	idA = strings.TrimSpace(idA)
	ctx.exec("close", idA)

	// Create ticket B, make it depend on A, then close it
	idB, _ := ctx.exec("new", "Ticket B")
	idB = strings.TrimSpace(idB)
	ctx.exec("dep", idB, idA)
	ctx.exec("close", idB)

	// Both are closed and nothing outside the pair references them, so both
	// are deletable in the same run: B is unreferenced, and A's only dependant
	// (B) is being deleted too.

	output, err := ctx.exec("clean")
	if err != nil {
		t.Fatalf("clean command error: %v", err)
	}

	if !strings.Contains(output, "Found 2 closed ticket(s)") {
		t.Errorf("expected 'Found 2 closed ticket(s)', got: %s", output)
	}
	if !strings.Contains(output, "2 deletable") {
		t.Errorf("expected '2 deletable', got: %s", output)
	}
	if !strings.Contains(output, "0 blocked") {
		t.Errorf("expected '0 blocked', got: %s", output)
	}

	// Run with --fix
	output, err = ctx.exec("clean", "--fix")
	if err != nil {
		t.Fatalf("clean --fix command error: %v", err)
	}

	for _, id := range []string{idA, idB} {
		if !strings.Contains(output, "Deleted: "+id) {
			t.Errorf("expected ticket %s to be deleted, got: %s", id, output)
		}
		if _, err := ctx.store().Get(id); err == nil {
			t.Errorf("ticket %s should be deleted", id)
		}
	}
}

// TestCleanDeepDependencyChain - An entire chain of closed tickets is removed
// in a single run, not one level per invocation.
func TestCleanDeepDependencyChain(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Build a 5-deep chain: ids[4] -> ids[3] -> ids[2] -> ids[1] -> ids[0]
	var ids []string
	for i := 0; i < 5; i++ {
		id, _ := ctx.exec("new", "Chain ticket")
		id = strings.TrimSpace(id)
		if i > 0 {
			ctx.exec("dep", id, ids[i-1])
		}
		ids = append(ids, id)
	}
	for _, id := range ids {
		ctx.exec("close", id)
	}

	// Dry-run should already report the whole chain as deletable
	output, err := ctx.exec("clean")
	if err != nil {
		t.Fatalf("clean command error: %v", err)
	}
	if !strings.Contains(output, "5 deletable") {
		t.Errorf("expected '5 deletable', got: %s", output)
	}
	if !strings.Contains(output, "0 blocked") {
		t.Errorf("expected '0 blocked', got: %s", output)
	}

	// A single --fix run must delete all of them
	output, err = ctx.exec("clean", "--fix")
	if err != nil {
		t.Fatalf("clean --fix command error: %v", err)
	}
	if !strings.Contains(output, "Deleted 5 ticket(s)") {
		t.Errorf("expected 'Deleted 5 ticket(s)' in one run, got: %s", output)
	}
	for _, id := range ids {
		if _, err := ctx.store().Get(id); err == nil {
			t.Errorf("ticket %s should be deleted after a single run", id)
		}
	}

	// Nothing should be left for a second run
	output, err = ctx.exec("clean", "--fix")
	if err != nil {
		t.Fatalf("second clean --fix command error: %v", err)
	}
	if !strings.Contains(output, "No closed tickets found") {
		t.Errorf("expected nothing left to clean, got: %s", output)
	}
}

// TestCleanDeepParentChain - A nested hierarchy of closed tickets is removed in
// a single run.
func TestCleanDeepParentChain(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Build a 4-deep hierarchy: ids[0] > ids[1] > ids[2] > ids[3]
	var ids []string
	for i := 0; i < 4; i++ {
		var id string
		if i == 0 {
			id, _ = ctx.exec("new", "Root ticket")
		} else {
			id, _ = ctx.exec("new", "--parent", ids[i-1], "Child ticket")
		}
		ids = append(ids, strings.TrimSpace(id))
	}
	for _, id := range ids {
		ctx.exec("close", id)
	}

	output, err := ctx.exec("clean", "--fix")
	if err != nil {
		t.Fatalf("clean --fix command error: %v", err)
	}
	if !strings.Contains(output, "Deleted 4 ticket(s)") {
		t.Errorf("expected 'Deleted 4 ticket(s)' in one run, got: %s", output)
	}
	for _, id := range ids {
		if _, err := ctx.store().Get(id); err == nil {
			t.Errorf("ticket %s should be deleted after a single run", id)
		}
	}
}

// TestCleanChainBlockedAtHead - An open dependant at the head of a closed chain
// blocks the whole chain, so no run creates dangling references.
func TestCleanChainBlockedAtHead(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Build a 3-deep closed chain: ids[2] -> ids[1] -> ids[0]
	var ids []string
	for i := 0; i < 3; i++ {
		id, _ := ctx.exec("new", "Chain ticket")
		id = strings.TrimSpace(id)
		if i > 0 {
			ctx.exec("dep", id, ids[i-1])
		}
		ids = append(ids, id)
	}
	for _, id := range ids {
		ctx.exec("close", id)
	}

	// An open ticket depends on the last link in the chain
	idOpen, _ := ctx.exec("new", "Open dependant")
	idOpen = strings.TrimSpace(idOpen)
	ctx.exec("dep", idOpen, ids[2])

	output, err := ctx.exec("clean", "--fix")
	if err != nil {
		t.Fatalf("clean --fix command error: %v", err)
	}
	if !strings.Contains(output, "No deletable tickets") {
		t.Errorf("expected nothing deletable, got: %s", output)
	}

	// Every ticket in the chain must survive, keeping all deps resolvable
	for _, id := range ids {
		if _, err := ctx.store().Get(id); err != nil {
			t.Errorf("ticket %s should still exist: %v", id, err)
		}
	}
}

// TestCleanBlockedClosedChildBlocksParent - A closed parent is not deleted when
// a closed child survives, which would leave a dangling parent reference.
func TestCleanBlockedClosedChildBlocksParent(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Closed parent, plus a standalone open ticket. The open ticket is created
	// before any --parent use: the new command's flag variable is process-wide
	// and would otherwise leak the parent onto it.
	idParent, _ := ctx.exec("new", "Closed parent")
	idParent = strings.TrimSpace(idParent)
	ctx.exec("close", idParent)

	idOpen, _ := ctx.exec("new", "Open dependant")
	idOpen = strings.TrimSpace(idOpen)

	// Closed child of the parent
	idChild, _ := ctx.exec("new", "--parent", idParent, "Closed child")
	idChild = strings.TrimSpace(idChild)
	ctx.exec("close", idChild)

	// The open ticket depends on the child, so the child must survive
	ctx.exec("dep", idOpen, idChild)

	output, err := ctx.exec("clean", "--fix")
	if err != nil {
		t.Fatalf("clean --fix command error: %v", err)
	}
	if !strings.Contains(output, "No deletable tickets") {
		t.Errorf("expected nothing deletable, got: %s", output)
	}

	// The child's parent reference must still resolve
	child, err := ctx.store().Get(idChild)
	if err != nil {
		t.Fatalf("child %s should still exist: %v", idChild, err)
	}
	if child.Parent != idParent {
		t.Errorf("child parent = %q, want %q", child.Parent, idParent)
	}
	if _, err := ctx.store().Get(idParent); err != nil {
		t.Errorf("parent %s should still exist: %v", idParent, err)
	}
}

// TestCleanDependencyCycle - Mutually dependent closed tickets are deleted
// together rather than blocking each other forever.
func TestCleanDependencyCycle(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	idA, _ := ctx.exec("new", "Ticket A")
	idA = strings.TrimSpace(idA)
	idB, _ := ctx.exec("new", "Ticket B")
	idB = strings.TrimSpace(idB)

	ctx.exec("dep", idA, idB)
	ctx.exec("dep", idB, idA)
	ctx.exec("close", idA)
	ctx.exec("close", idB)

	output, err := ctx.exec("clean", "--fix")
	if err != nil {
		t.Fatalf("clean --fix command error: %v", err)
	}
	if !strings.Contains(output, "Deleted 2 ticket(s)") {
		t.Errorf("expected 'Deleted 2 ticket(s)', got: %s", output)
	}
	for _, id := range []string{idA, idB} {
		if _, err := ctx.store().Get(id); err == nil {
			t.Errorf("ticket %s should be deleted", id)
		}
	}
}
