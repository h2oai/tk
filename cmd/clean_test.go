package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/h2oai/tk/internal/ticket"
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

// TestCleanRefuseWithLinksBlockPolicy - Closed ticket has bidirectional links,
// which block under the historical --links=block policy.
func TestCleanRefuseWithLinksBlockPolicy(t *testing.T) {
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

		output, err := ctx.exec("clean", "--links=block")
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

		output, err := ctx.exec("clean", "--links=block")
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

// TestCleanTransitiveLinks - Under --links=block, links are resolved
// transitively, like deps/children.
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

		output, err := ctx.exec("clean", "--links=block")
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

		output, err := ctx.exec("clean", "--links=block")
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

		output, err := ctx.exec("clean", "--links=block")
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

// TestCleanLinksPolicy - --links selects whether links block deletion. The
// default is ignore, so a closed ticket linked to a surviving ticket is
// deletable; --links=block reproduces the historical behavior. Deps and parent
// blocking are unaffected by the flag.
func TestCleanLinksPolicy(t *testing.T) {
	t.Run("default ignores a link to an open ticket", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Closed ticket")
		idA = strings.TrimSpace(idA)
		ctx.exec("close", idA)

		idB, _ := ctx.exec("new", "Open linked ticket")
		idB = strings.TrimSpace(idB)
		ctx.exec("link", idA, idB)

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}
		if !strings.Contains(output, "1 deletable") || !strings.Contains(output, "0 blocked") {
			t.Errorf("expected the linked ticket to be deletable by default, got: %s", output)
		}

		output, err = ctx.exec("clean", "--fix")
		if err != nil {
			t.Fatalf("clean --fix command error: %v", err)
		}
		if !strings.Contains(output, "Deleted 1 ticket(s)") {
			t.Errorf("expected 'Deleted 1 ticket(s)', got: %s", output)
		}
		if _, err := ctx.store().Get(idA); err == nil {
			t.Errorf("ticket %s should be deleted under --links=ignore", idA)
		}
	})

	t.Run("explicit ignore matches the default", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Closed ticket")
		idA = strings.TrimSpace(idA)
		ctx.exec("close", idA)

		idB, _ := ctx.exec("new", "Open linked ticket")
		idB = strings.TrimSpace(idB)
		ctx.exec("link", idA, idB)

		output, err := ctx.exec("clean", "--links=ignore")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}
		if !strings.Contains(output, "1 deletable") || !strings.Contains(output, "0 blocked") {
			t.Errorf("expected '1 deletable' and '0 blocked', got: %s", output)
		}
	})

	t.Run("block reproduces the historical behavior", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Closed ticket")
		idA = strings.TrimSpace(idA)
		ctx.exec("close", idA)

		idB, _ := ctx.exec("new", "Open linked ticket")
		idB = strings.TrimSpace(idB)
		ctx.exec("link", idA, idB)

		output, err := ctx.exec("clean", "--links=block")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}
		if !strings.Contains(output, "0 deletable") || !strings.Contains(output, "1 blocked") {
			t.Errorf("expected '0 deletable' and '1 blocked', got: %s", output)
		}
		if !strings.Contains(output, "has links") {
			t.Errorf("expected 'has links', got: %s", output)
		}

		output, err = ctx.exec("clean", "--fix", "--links=block")
		if err != nil {
			t.Fatalf("clean --fix command error: %v", err)
		}
		if _, err := ctx.store().Get(idA); err != nil {
			t.Errorf("ticket %s should survive under --links=block: %v", idA, err)
		}
	})

	t.Run("link to a blocked closed ticket", func(t *testing.T) {
		// A links to B. B is closed but pinned by an open child, so B is
		// blocked regardless of the link policy. Under ignore only B is
		// blocked; under block A is dragged in as well.
		newLinkedChild := func(t *testing.T) (string, string, string) {
			t.Helper()
			ctx, cleanup := setupTestCmd(t)
			t.Cleanup(cleanup)

			idA, _ := ctx.exec("new", "Ticket A")
			idA = strings.TrimSpace(idA)
			idB, _ := ctx.exec("new", "Ticket B")
			idB = strings.TrimSpace(idB)
			ctx.exec("link", idA, idB)
			ctx.exec("close", idA)
			ctx.exec("close", idB)
			ctx.exec("new", "--parent", idB, "Child of B")
			return idA, idB, ctx.ticketsDir
		}

		t.Run("ignore only blocks the ticket anchored by the child", func(t *testing.T) {
			_, idB, dir := newLinkedChild(t)
			ctx := &testContext{ticketsDir: dir, t: t}

			output, err := ctx.exec("clean", "--links=ignore")
			if err != nil {
				t.Fatalf("clean command error: %v", err)
			}
			if !strings.Contains(output, "1 deletable") || !strings.Contains(output, "1 blocked") {
				t.Errorf("expected '1 deletable' and '1 blocked', got: %s", output)
			}
			if strings.Contains(output, "has links") {
				t.Errorf("expected no link-based block under --links=ignore, got: %s", output)
			}
			if !strings.Contains(output, idB+" [closed]") || !strings.Contains(output, "has non-closed children") {
				t.Errorf("expected %s blocked by its child, got: %s", idB, output)
			}
		})

		t.Run("block drags the linked ticket in", func(t *testing.T) {
			idA, idB, dir := newLinkedChild(t)
			ctx := &testContext{ticketsDir: dir, t: t}

			output, err := ctx.exec("clean", "--links=block")
			if err != nil {
				t.Fatalf("clean command error: %v", err)
			}
			if !strings.Contains(output, "0 deletable") || !strings.Contains(output, "2 blocked") {
				t.Errorf("expected '0 deletable' and '2 blocked', got: %s", output)
			}
			if !strings.Contains(output, idA+" [closed]") || !strings.Contains(output, "has links") {
				t.Errorf("expected %s blocked with 'has links', got: %s", idA, output)
			}
			if !strings.Contains(output, idB+" [closed]") || !strings.Contains(output, "has non-closed children") {
				t.Errorf("expected %s blocked by its child, got: %s", idB, output)
			}
		})
	})

	t.Run("deps and parent blocking are unaffected", func(t *testing.T) {
		for _, policy := range []string{"ignore", "block"} {
			t.Run("open dependant blocks under --links="+policy, func(t *testing.T) {
				ctx, cleanup := setupTestCmd(t)
				defer cleanup()

				idA, _ := ctx.exec("new", "Closed ticket")
				idA = strings.TrimSpace(idA)
				ctx.exec("close", idA)

				idB, _ := ctx.exec("new", "Open dependant")
				idB = strings.TrimSpace(idB)
				ctx.exec("dep", idB, idA)

				output, err := ctx.exec("clean", "--links="+policy)
				if err != nil {
					t.Fatalf("clean command error: %v", err)
				}
				if !strings.Contains(output, "1 blocked") || !strings.Contains(output, "has dependants") {
					t.Errorf("expected dependant blocking under --links=%s, got: %s", policy, output)
				}
			})

			t.Run("open child blocks under --links="+policy, func(t *testing.T) {
				ctx, cleanup := setupTestCmd(t)
				defer cleanup()

				idParent, _ := ctx.exec("new", "Closed parent")
				idParent = strings.TrimSpace(idParent)
				ctx.exec("close", idParent)
				ctx.exec("new", "--parent", idParent, "Open child")

				output, err := ctx.exec("clean", "--links="+policy)
				if err != nil {
					t.Fatalf("clean command error: %v", err)
				}
				if !strings.Contains(output, "1 blocked") || !strings.Contains(output, "has non-closed children") {
					t.Errorf("expected child blocking under --links=%s, got: %s", policy, output)
				}
			})
		}
	})

	t.Run("dangling link follows the policy", func(t *testing.T) {
		// A closed ticket links to an ID whose file has been removed. Under
		// ignore the dangling link is inert; under block it is the historical
		// [missing] anchor.
		newDangling := func(t *testing.T) (string, string) {
			t.Helper()
			ctx, cleanup := setupTestCmd(t)
			t.Cleanup(cleanup)

			idA, _ := ctx.exec("new", "Closed with dangling link")
			idA = strings.TrimSpace(idA)
			idB, _ := ctx.exec("new", "Link target")
			idB = strings.TrimSpace(idB)
			ctx.exec("link", idA, idB)
			ctx.exec("close", idA)
			ctx.exec("close", idB)
			if err := os.Remove(filepath.Join(ctx.ticketsDir, idB+".md")); err != nil {
				t.Fatalf("failed to remove link target: %v", err)
			}
			return idA, ctx.ticketsDir
		}

		t.Run("ignore leaves the ticket deletable", func(t *testing.T) {
			idA, dir := newDangling(t)
			ctx := &testContext{ticketsDir: dir, t: t}

			output, err := ctx.exec("clean", "--links=ignore")
			if err != nil {
				t.Fatalf("clean command error: %v", err)
			}
			if !strings.Contains(output, "1 deletable") || !strings.Contains(output, "0 blocked") {
				t.Errorf("expected dangling link to be ignored, got: %s", output)
			}

			output, err = ctx.exec("clean", "--fix", "--links=ignore")
			if err != nil {
				t.Fatalf("clean --fix command error: %v", err)
			}
			if !strings.Contains(output, "Deleted 1 ticket(s)") {
				t.Errorf("expected 'Deleted 1 ticket(s)', got: %s", output)
			}
			if _, err := ctx.store().Get(idA); err == nil {
				t.Errorf("ticket %s with a dangling link should be deleted", idA)
			}
		})

		t.Run("block keeps the missing anchor", func(t *testing.T) {
			_, dir := newDangling(t)
			ctx := &testContext{ticketsDir: dir, t: t}

			output, err := ctx.exec("clean", "--links=block")
			if err != nil {
				t.Fatalf("clean command error: %v", err)
			}
			if !strings.Contains(output, "1 blocked") || !strings.Contains(output, "[missing]") {
				t.Errorf("expected a [missing] anchor under --links=block, got: %s", output)
			}
		})
	})

	t.Run("invalid policy is rejected", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idA, _ := ctx.exec("new", "Closed ticket")
		idA = strings.TrimSpace(idA)
		ctx.exec("close", idA)

		if _, err := ctx.exec("clean", "--links=bogus"); err == nil {
			t.Error("expected an error for an invalid --links value")
		}

		// The rejected run must not delete anything.
		if _, err := ctx.store().Get(idA); err != nil {
			t.Errorf("ticket %s should survive a rejected run: %v", idA, err)
		}
	})
}

// TestCleanHelpDocumentsLinksPolicy - clean --help carries the --links
// behavior table and states that deps/parent blocking is unaffected.
func TestCleanHelpDocumentsLinksPolicy(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Cobra's --help flag is sticky on the command, so clear it once the
	// captured invocation returns; otherwise later tests print help instead of
	// running the command.
	defer func() {
		if flag := cleanCmd.Flags().Lookup("help"); flag != nil {
			_ = flag.Value.Set("false")
		}
	}()

	output, err := ctx.exec("clean", "--help")
	if err != nil {
		t.Fatalf("clean --help error: %v", err)
	}

	for _, want := range []string{
		"--links",
		"--links=ignore",
		"--links=block",
		"Dependants and children are always hard blockers",
		"tk prune",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("clean --help missing %q:\n%s", want, output)
		}
	}
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

// cleanPlanJSONTest mirrors the documented `tk clean --json` dry-run schema.
type cleanPlanJSONTest struct {
	Closed     int                   `json:"closed"`
	Deletable  int                   `json:"deletable"`
	Blocked    int                   `json:"blocked"`
	Direct     int                   `json:"direct"`
	Transitive int                   `json:"transitive"`
	Anchors    []cleanAnchorJSONTest `json:"anchors"`
	Tickets    []cleanTicketJSONTest `json:"tickets"`
}

type cleanAnchorJSONTest struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	BlockedCount int    `json:"blocked_count"`
}

type cleanTicketJSONTest struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Deletable bool   `json:"deletable"`
	Reason    string `json:"reason"`
	BlockedBy string `json:"blocked_by"`
	Anchor    string `json:"anchor"`
}

// cleanFixJSONTest mirrors the documented `tk clean --json --fix` schema.
type cleanFixJSONTest struct {
	Deleted int                   `json:"deleted"`
	Skipped int                   `json:"skipped"`
	Errors  int                   `json:"errors"`
	Results []cleanResultJSONTest `json:"results"`
}

type cleanResultJSONTest struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
	Error   string `json:"error"`
}

// parseCleanPlan unmarshals and validates the shape of `tk clean --json`
// output, failing the test on anything that is not parseable JSON.
func parseCleanPlan(t *testing.T, output string) cleanPlanJSONTest {
	t.Helper()
	var plan cleanPlanJSONTest
	if err := json.Unmarshal([]byte(output), &plan); err != nil {
		t.Fatalf("clean --json output is not valid JSON: %v\n%s", err, output)
	}
	return plan
}

// TestCleanJSONPlan - --json emits the dry-run plan with counts, anchors, and
// per-ticket entries, and no human prose.
func TestCleanJSONPlan(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Deletable closed ticket.
	idDeletable, _ := ctx.exec("new", "Deletable closed")
	idDeletable = strings.TrimSpace(idDeletable)
	ctx.exec("close", idDeletable)

	// Closed ticket pinned by an open dependant.
	idBlocked, _ := ctx.exec("new", "Blocked closed")
	idBlocked = strings.TrimSpace(idBlocked)
	ctx.exec("close", idBlocked)
	idOpen, _ := ctx.exec("new", "Open anchor")
	idOpen = strings.TrimSpace(idOpen)
	ctx.exec("dep", idOpen, idBlocked)

	output, err := ctx.exec("clean", "--json")
	if err != nil {
		t.Fatalf("clean --json command error: %v", err)
	}

	plan := parseCleanPlan(t, output)
	if plan.Closed != 2 || plan.Deletable != 1 || plan.Blocked != 1 {
		t.Errorf("counts = closed %d, deletable %d, blocked %d; want 2, 1, 1", plan.Closed, plan.Deletable, plan.Blocked)
	}
	if plan.Direct != 1 || plan.Transitive != 0 {
		t.Errorf("split = direct %d, transitive %d; want 1, 0", plan.Direct, plan.Transitive)
	}

	if len(plan.Anchors) != 1 {
		t.Fatalf("expected 1 anchor, got %d: %s", len(plan.Anchors), output)
	}
	if plan.Anchors[0].ID != idOpen || plan.Anchors[0].Status != "open" || plan.Anchors[0].BlockedCount != 1 {
		t.Errorf("unexpected anchor: %+v", plan.Anchors[0])
	}

	if len(plan.Tickets) != 2 {
		t.Fatalf("expected 2 ticket entries, got %d: %s", len(plan.Tickets), output)
	}
	byID := make(map[string]cleanTicketJSONTest, len(plan.Tickets))
	for _, e := range plan.Tickets {
		byID[e.ID] = e
	}

	blocked, ok := byID[idBlocked]
	if !ok {
		t.Fatalf("expected entry for blocked ticket %s: %s", idBlocked, output)
	}
	if blocked.Status != "closed" || blocked.Deletable {
		t.Errorf("blocked entry = %+v; want closed and not deletable", blocked)
	}
	if blocked.Reason != "dependant" || blocked.BlockedBy != idOpen || blocked.Anchor != idOpen {
		t.Errorf("blocked entry = %+v; want reason=dependant blocked_by=%s anchor=%s", blocked, idOpen, idOpen)
	}

	deletable, ok := byID[idDeletable]
	if !ok {
		t.Fatalf("expected entry for deletable ticket %s: %s", idDeletable, output)
	}
	if !deletable.Deletable || deletable.Status != "closed" {
		t.Errorf("deletable entry = %+v; want closed and deletable", deletable)
	}
	if deletable.Reason != "" || deletable.BlockedBy != "" || deletable.Anchor != "" {
		t.Errorf("deletable entry = %+v; want empty reason/blocked_by/anchor", deletable)
	}

	for _, prose := range []string{"Found ", "Run with --fix", "Blocked tickets:", "Anchors:", "Deleted:"} {
		if strings.Contains(output, prose) {
			t.Errorf("JSON output must not contain prose %q: %s", prose, output)
		}
	}
}

// TestCleanJSONMissingAnchor - a dangling link produces a [missing] anchor
// with the normalized "link" reason.
func TestCleanJSONMissingAnchor(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	idA, _ := ctx.exec("new", "Closed with dangling link")
	idA = strings.TrimSpace(idA)
	idB, _ := ctx.exec("new", "Link target")
	idB = strings.TrimSpace(idB)
	ctx.exec("link", idA, idB)
	ctx.exec("close", idA)
	ctx.exec("close", idB)

	if err := os.Remove(filepath.Join(ctx.ticketsDir, idB+".md")); err != nil {
		t.Fatalf("failed to remove link target: %v", err)
	}

	output, err := ctx.exec("clean", "--json", "--links=block")
	if err != nil {
		t.Fatalf("clean --json command error: %v", err)
	}
	plan := parseCleanPlan(t, output)

	if len(plan.Anchors) != 1 || plan.Anchors[0].ID != idB || plan.Anchors[0].Status != "missing" {
		t.Fatalf("expected one [missing] anchor %s, got %+v", idB, plan.Anchors)
	}
	entry := plan.Tickets[0]
	if entry.Reason != "link" || entry.BlockedBy != idB || entry.Anchor != idB {
		t.Errorf("entry = %+v; want reason=link blocked_by=anchor=%s", entry, idB)
	}
}

// TestCleanJSONEmpty - --json still emits valid JSON when there is nothing to
// clean.
func TestCleanJSONEmpty(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	ctx.exec("new", "Open ticket")

	output, err := ctx.exec("clean", "--json")
	if err != nil {
		t.Fatalf("clean --json command error: %v", err)
	}
	plan := parseCleanPlan(t, output)
	if plan.Closed != 0 || plan.Deletable != 0 || plan.Blocked != 0 {
		t.Errorf("counts = %+v; want all zero", plan)
	}
	if plan.Direct != 0 || plan.Transitive != 0 {
		t.Errorf("split = direct %d, transitive %d; want 0, 0", plan.Direct, plan.Transitive)
	}
	if plan.Anchors == nil || plan.Tickets == nil {
		t.Errorf("anchors and tickets must be present as arrays: %s", output)
	}
	if strings.Contains(output, "No closed tickets found") {
		t.Errorf("JSON mode must not emit prose: %s", output)
	}
}

// TestCleanJSONDirectTransitiveCounts - --json reports the direct/transitive
// split of the blocked set, and the two counts partition the blocked count.
func TestCleanJSONDirectTransitiveCounts(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// A 4-deep closed chain pinned by one open anchor: ids[3] is directly
	// anchored, the other three are demoted transitively.
	var ids []string
	for i := 0; i < 4; i++ {
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
	idOpen, _ := ctx.exec("new", "Open anchor")
	idOpen = strings.TrimSpace(idOpen)
	ctx.exec("dep", idOpen, ids[3])

	output, err := ctx.exec("clean", "--json")
	if err != nil {
		t.Fatalf("clean --json command error: %v", err)
	}
	plan := parseCleanPlan(t, output)
	if plan.Blocked != 4 || plan.Direct != 1 || plan.Transitive != 3 {
		t.Errorf("counts = blocked %d, direct %d, transitive %d; want 4, 1, 3", plan.Blocked, plan.Direct, plan.Transitive)
	}
	if plan.Direct+plan.Transitive != plan.Blocked {
		t.Errorf("direct %d + transitive %d != blocked %d", plan.Direct, plan.Transitive, plan.Blocked)
	}
}

// TestCleanJSONStableOrdering - repeated runs are byte-identical and the
// arrays are ordered deterministically.
func TestCleanJSONStableOrdering(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// A three-ticket closed chain pinned by one open anchor.
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
	idOpen1, _ := ctx.exec("new", "Open anchor 1")
	idOpen1 = strings.TrimSpace(idOpen1)
	ctx.exec("dep", idOpen1, ids[2])

	// A single closed ticket pinned by a second open anchor.
	idSingle, _ := ctx.exec("new", "Single blocked")
	idSingle = strings.TrimSpace(idSingle)
	ctx.exec("close", idSingle)
	idOpen2, _ := ctx.exec("new", "Open anchor 2")
	idOpen2 = strings.TrimSpace(idOpen2)
	ctx.exec("dep", idOpen2, idSingle)

	first, err := ctx.exec("clean", "--json")
	if err != nil {
		t.Fatalf("clean --json command error: %v", err)
	}
	second, err := ctx.exec("clean", "--json")
	if err != nil {
		t.Fatalf("clean --json command error: %v", err)
	}
	if first != second {
		t.Errorf("expected byte-identical output across runs\nfirst:  %s\nsecond: %s", first, second)
	}

	plan := parseCleanPlan(t, first)
	if len(plan.Anchors) != 2 || plan.Anchors[0].ID != idOpen1 || plan.Anchors[0].BlockedCount != 3 || plan.Anchors[1].ID != idOpen2 {
		t.Errorf("anchors not sorted by blocked_count desc: %+v", plan.Anchors)
	}

	gotIDs := make([]string, 0, len(plan.Tickets))
	for _, e := range plan.Tickets {
		gotIDs = append(gotIDs, e.ID)
	}
	wantIDs := append([]string(nil), gotIDs...)
	sort.Strings(wantIDs)
	if strings.Join(gotIDs, ",") != strings.Join(wantIDs, ",") {
		t.Errorf("tickets not sorted by id: %v", gotIDs)
	}
}

// TestCleanJSONFix - --json --fix deletes tickets and reports results instead
// of the plan.
func TestCleanJSONFix(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	var ids []string
	for i := 0; i < 2; i++ {
		id, _ := ctx.exec("new", "Deletable")
		id = strings.TrimSpace(id)
		ctx.exec("close", id)
		ids = append(ids, id)
	}

	// One blocked closed ticket.
	idBlocked, _ := ctx.exec("new", "Blocked")
	idBlocked = strings.TrimSpace(idBlocked)
	ctx.exec("close", idBlocked)
	idOpen, _ := ctx.exec("new", "Open anchor")
	idOpen = strings.TrimSpace(idOpen)
	ctx.exec("dep", idOpen, idBlocked)

	output, err := ctx.exec("clean", "--json", "--fix")
	if err != nil {
		t.Fatalf("clean --json --fix command error: %v", err)
	}

	var result cleanFixJSONTest
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("clean --json --fix output is not valid JSON: %v\n%s", err, output)
	}
	if result.Deleted != 2 || result.Skipped != 1 || result.Errors != 0 {
		t.Errorf("result = %+v; want deleted 2, skipped 1, errors 0", result)
	}
	if len(result.Results) != 2 {
		t.Fatalf("expected 2 results, got %d: %s", len(result.Results), output)
	}
	for _, r := range result.Results {
		if !r.Deleted || r.Error != "" {
			t.Errorf("result entry = %+v; want deleted with no error", r)
		}
	}

	for _, prose := range []string{"Deleting closed tickets", "Deleted: ", "Run with --fix", "Found "} {
		if strings.Contains(output, prose) {
			t.Errorf("JSON output must not contain prose %q: %s", prose, output)
		}
	}

	for _, id := range ids {
		if _, err := ctx.store().Get(id); err == nil {
			t.Errorf("ticket %s should be deleted", id)
		}
	}
	if _, err := ctx.store().Get(idBlocked); err != nil {
		t.Errorf("blocked ticket %s should survive: %v", idBlocked, err)
	}
}

// TestCleanJSONFixEmpty - --json --fix with no deletable tickets still emits
// valid JSON.
func TestCleanJSONFixEmpty(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	output, err := ctx.exec("clean", "--json", "--fix")
	if err != nil {
		t.Fatalf("clean --json --fix command error: %v", err)
	}
	var result cleanFixJSONTest
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, output)
	}
	if result.Deleted != 0 || result.Skipped != 0 || result.Errors != 0 {
		t.Errorf("result = %+v; want all zero", result)
	}
	if result.Results == nil {
		t.Errorf("results must be present as an array: %s", output)
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

// TestCleanAnchorSingleOpenAnchorOverChain - A closed chain pinned by one open
// dependant is grouped under that single surviving anchor.
func TestCleanAnchorSingleOpenAnchorOverChain(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Build a closed chain: ids[1] depends on ids[0], ids[2] on ids[1].
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

	// An open ticket depends on the tail, pinning the whole chain.
	idOpen, _ := ctx.exec("new", "Open anchor")
	idOpen = strings.TrimSpace(idOpen)
	ctx.exec("dep", idOpen, ids[2])

	output, err := ctx.exec("clean")
	if err != nil {
		t.Fatalf("clean command error: %v", err)
	}

	if !strings.Contains(output, "3 blocked - 1 directly anchored, 2 transitively blocked") {
		t.Errorf("expected '3 blocked - 1 directly anchored, 2 transitively blocked', got: %s", output)
	}
	if !strings.Contains(output, "1 surviving anchor(s)") {
		t.Errorf("expected '1 surviving anchor(s)', got: %s", output)
	}
	if !strings.Contains(output, "Anchors:") {
		t.Errorf("expected anchor section, got: %s", output)
	}
	if !strings.Contains(output, idOpen+" [open] blocks 3") {
		t.Errorf("expected anchor line for %s, got: %s", idOpen, output)
	}
	// The anchor section must precede the per-ticket dump.
	if ai, bi := strings.Index(output, "Anchors:"), strings.Index(output, "Blocked tickets:"); ai < 0 || bi < 0 || ai > bi {
		t.Errorf("expected anchor section before blocked list, got: %s", output)
	}
	// Every blocked ticket is listed.
	for _, id := range ids {
		if !strings.Contains(output, id) {
			t.Errorf("expected blocked ticket %s in output, got: %s", id, output)
		}
	}
}

// TestCleanAnchorMissingID - A dangling link names a [missing] anchor.
func TestCleanAnchorMissingID(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	idA, _ := ctx.exec("new", "Closed with dangling link")
	idA = strings.TrimSpace(idA)
	idB, _ := ctx.exec("new", "Link target")
	idB = strings.TrimSpace(idB)

	ctx.exec("link", idA, idB)
	ctx.exec("close", idA)
	ctx.exec("close", idB)

	// Remove the target's file so idA's link dangles.
	if err := os.Remove(filepath.Join(ctx.ticketsDir, idB+".md")); err != nil {
		t.Fatalf("failed to remove link target: %v", err)
	}

	output, err := ctx.exec("clean", "--links=block")
	if err != nil {
		t.Fatalf("clean command error: %v", err)
	}

	if !strings.Contains(output, "1 blocked - 1 directly anchored, 0 transitively blocked") {
		t.Errorf("expected '1 blocked - 1 directly anchored, 0 transitively blocked', got: %s", output)
	}
	if !strings.Contains(output, "1 surviving anchor(s)") {
		t.Errorf("expected '1 surviving anchor(s)', got: %s", output)
	}
	if !strings.Contains(output, idB+" [missing] blocks 1") {
		t.Errorf("expected [missing] anchor %s, got: %s", idB, output)
	}
	if !strings.Contains(output, "(link -> "+idA+")") {
		t.Errorf("expected 'link -> %s', got: %s", idA, output)
	}
}

// TestCleanAnchorTwoDisjointAnchors - Blocks from independent anchors are
// grouped and counted separately.
func TestCleanAnchorTwoDisjointAnchors(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Anchor 1 blocks closed ticket A.
	idA, _ := ctx.exec("new", "Closed A")
	idA = strings.TrimSpace(idA)
	ctx.exec("close", idA)
	idOpen1, _ := ctx.exec("new", "Open anchor 1")
	idOpen1 = strings.TrimSpace(idOpen1)
	ctx.exec("dep", idOpen1, idA)

	// Anchor 2 blocks closed ticket B.
	idB, _ := ctx.exec("new", "Closed B")
	idB = strings.TrimSpace(idB)
	ctx.exec("close", idB)
	idOpen2, _ := ctx.exec("new", "Open anchor 2")
	idOpen2 = strings.TrimSpace(idOpen2)
	ctx.exec("dep", idOpen2, idB)

	output, err := ctx.exec("clean")
	if err != nil {
		t.Fatalf("clean command error: %v", err)
	}

	if !strings.Contains(output, "2 blocked - 2 directly anchored, 0 transitively blocked") {
		t.Errorf("expected '2 blocked - 2 directly anchored, 0 transitively blocked', got: %s", output)
	}
	if !strings.Contains(output, "2 surviving anchor(s)") {
		t.Errorf("expected '2 surviving anchor(s)', got: %s", output)
	}
	if !strings.Contains(output, idOpen1+" [open] blocks 1") {
		t.Errorf("expected anchor %s, got: %s", idOpen1, output)
	}
	if !strings.Contains(output, idOpen2+" [open] blocks 1") {
		t.Errorf("expected anchor %s, got: %s", idOpen2, output)
	}
}

// TestCleanAnchorCycleOnlyDeletable - A closed component with no non-candidate
// reference stays deletable and never appears as blocked or anchored.
func TestCleanAnchorCycleOnlyDeletable(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	idA, _ := ctx.exec("new", "Ticket A")
	idA = strings.TrimSpace(idA)
	idB, _ := ctx.exec("new", "Ticket B")
	idB = strings.TrimSpace(idB)

	ctx.exec("link", idA, idB)
	ctx.exec("close", idA)
	ctx.exec("close", idB)

	output, err := ctx.exec("clean", "--links=block")
	if err != nil {
		t.Fatalf("clean command error: %v", err)
	}

	if !strings.Contains(output, "2 deletable") || !strings.Contains(output, "0 blocked") {
		t.Errorf("expected cycle-only component to be deletable, got: %s", output)
	}
	if strings.Contains(output, "Anchors:") {
		t.Errorf("cycle-only component must not produce anchors, got: %s", output)
	}

	output, err = ctx.exec("clean", "--fix", "--links=block")
	if err != nil {
		t.Fatalf("clean --fix command error: %v", err)
	}
	if !strings.Contains(output, "Deleted 2 ticket(s)") {
		t.Errorf("expected 'Deleted 2 ticket(s)', got: %s", output)
	}
}

// TestCleanBlockedListCap - The human dry-run blocked listing is capped at
// cleanBlockedCap unless --verbose is set.
func TestCleanBlockedListCap(t *testing.T) {
	// makeBlocked creates count closed tickets, each pinned in place by its
	// own open dependant, and returns the blocked ticket IDs.
	makeBlocked := func(ctx *testContext, count int) []string {
		ids := make([]string, 0, count)
		for i := 0; i < count; i++ {
			id, _ := ctx.exec("new", "Blocked closed")
			id = strings.TrimSpace(id)
			ctx.exec("close", id)
			dep, _ := ctx.exec("new", "Open dependant")
			dep = strings.TrimSpace(dep)
			ctx.exec("dep", dep, id)
			ids = append(ids, id)
		}
		return ids
	}

	countListed := func(output string, ids []string) int {
		n := 0
		for _, id := range ids {
			// Match the per-ticket blocked line, not the bare ID an anchor
			// label may mention for a single blocked ticket.
			if strings.Contains(output, id+" [closed]") {
				n++
			}
		}
		return n
	}

	t.Run("below cap lists all without truncation", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		ids := makeBlocked(ctx, cleanBlockedCap-1)

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}
		if !strings.Contains(output, "Blocked tickets:") {
			t.Errorf("expected 'Blocked tickets:' header, got: %s", output)
		}
		if strings.Contains(output, "... and") {
			t.Errorf("expected no truncation line, got: %s", output)
		}
		if got := countListed(output, ids); got != len(ids) {
			t.Errorf("expected all %d tickets listed, got %d: %s", len(ids), got, output)
		}
		if !strings.Contains(output, "has dependants") {
			t.Errorf("expected reason field preserved, got: %s", output)
		}
	})

	t.Run("at cap lists all without truncation", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		ids := makeBlocked(ctx, cleanBlockedCap)

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}
		if !strings.Contains(output, "Blocked tickets:") {
			t.Errorf("expected untruncated 'Blocked tickets:' header, got: %s", output)
		}
		if strings.Contains(output, "... and") {
			t.Errorf("expected no truncation line at cap, got: %s", output)
		}
		if got := countListed(output, ids); got != len(ids) {
			t.Errorf("expected all %d tickets listed, got %d: %s", len(ids), got, output)
		}
	})

	t.Run("above cap truncates and reports remainder", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		ids := makeBlocked(ctx, cleanBlockedCap+1)

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}
		wantHeader := fmt.Sprintf("Blocked tickets (showing %d of %d):", cleanBlockedCap, cleanBlockedCap+1)
		if !strings.Contains(output, wantHeader) {
			t.Errorf("expected %q, got: %s", wantHeader, output)
		}
		if !strings.Contains(output, "... and 1 more; re-run with --verbose for the full list.") {
			t.Errorf("expected truncation line, got: %s", output)
		}
		if got := countListed(output, ids); got != cleanBlockedCap {
			t.Errorf("expected %d tickets listed, got %d: %s", cleanBlockedCap, got, output)
		}
		if !strings.Contains(output, "has dependants") {
			t.Errorf("expected reason field preserved, got: %s", output)
		}
	})

	t.Run("verbose lifts the cap", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		ids := makeBlocked(ctx, cleanBlockedCap+1)

		output, err := ctx.exec("clean", "--verbose")
		if err != nil {
			t.Fatalf("clean --verbose command error: %v", err)
		}
		if strings.Contains(output, "(showing") {
			t.Errorf("expected uncapped header with --verbose, got: %s", output)
		}
		if strings.Contains(output, "... and") {
			t.Errorf("expected no truncation line with --verbose, got: %s", output)
		}
		if got := countListed(output, ids); got != len(ids) {
			t.Errorf("expected all %d tickets listed, got %d: %s", len(ids), got, output)
		}
	})
}

// TestCleanDirectTransitiveSplit - The blocked count is split into tickets
// anchored by their own blocking edge and tickets demoted only as cascade.
func TestCleanDirectTransitiveSplit(t *testing.T) {
	t.Run("open anchor at head of chain yields one direct and N-1 transitive", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Build a 4-deep closed chain: ids[3] -> ids[2] -> ids[1] -> ids[0].
		var ids []string
		for i := 0; i < 4; i++ {
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

		// An open ticket depends on the head, directly anchoring only ids[3];
		// the rest are demoted transitively.
		idOpen, _ := ctx.exec("new", "Open anchor")
		idOpen = strings.TrimSpace(idOpen)
		ctx.exec("dep", idOpen, ids[3])

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "4 blocked - 1 directly anchored, 3 transitively blocked") {
			t.Errorf("expected '4 blocked - 1 directly anchored, 3 transitively blocked', got: %s", output)
		}
		if !strings.Contains(output, "1 surviving anchor(s)") {
			t.Errorf("expected '1 surviving anchor(s)', got: %s", output)
		}
	})

	t.Run("every blocked ticket directly anchored counts all as direct", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Each closed ticket has its own open dependant, so every block is
		// directly anchored with no cascade.
		for i := 0; i < 3; i++ {
			id, _ := ctx.exec("new", "Blocked closed")
			id = strings.TrimSpace(id)
			ctx.exec("close", id)

			dep, _ := ctx.exec("new", "Open dependant")
			dep = strings.TrimSpace(dep)
			ctx.exec("dep", dep, id)
		}

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "3 blocked - 3 directly anchored, 0 transitively blocked") {
			t.Errorf("expected '3 blocked - 3 directly anchored, 0 transitively blocked', got: %s", output)
		}
	})

	t.Run("cycle-only set reports zero blocked", func(t *testing.T) {
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

		output, err := ctx.exec("clean")
		if err != nil {
			t.Fatalf("clean command error: %v", err)
		}

		if !strings.Contains(output, "0 blocked") {
			t.Errorf("expected '0 blocked' for a cycle-only set, got: %s", output)
		}
		if strings.Contains(output, "directly anchored") {
			t.Errorf("cycle-only set must not report a direct/transitive split, got: %s", output)
		}
	})
}

// componentRowTest is one parsed line of `tk clean --components` output.
type componentRowTest struct {
	size   int
	open   int
	closed int
	status string
}

// parseComponentRows extracts the component table from `tk clean --components`
// output.
func parseComponentRows(t *testing.T, output string) []componentRowTest {
	t.Helper()
	var rows []componentRowTest
	started := false
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Components:" {
			started = true
			continue
		}
		if !started || trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 5 || fields[0] == "#" {
			continue
		}
		size, errSize := strconv.Atoi(fields[1])
		open, errOpen := strconv.Atoi(fields[2])
		closed, errClosed := strconv.Atoi(fields[3])
		if errSize != nil || errOpen != nil || errClosed != nil {
			continue
		}
		rows = append(rows, componentRowTest{
			size:   size,
			open:   open,
			closed: closed,
			status: strings.Join(fields[4:], " "),
		})
	}
	return rows
}

// TestCleanComponentsGrouping - the component view groups linked tickets,
// labels anchored/deletable/n/a components, and orders by size descending.
func TestCleanComponentsGrouping(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Anchored component: three closed tickets linked to one open anchor.
	var chain []string
	for i := 0; i < 3; i++ {
		id, _ := ctx.exec("new", fmt.Sprintf("Chain %d", i))
		id = strings.TrimSpace(id)
		ctx.exec("close", id)
		chain = append(chain, id)
	}
	idOpen, _ := ctx.exec("new", "Open anchor")
	idOpen = strings.TrimSpace(idOpen)
	ctx.exec("link", chain[0], chain[1], chain[2], idOpen)

	// Deletable component: two closed tickets linked together.
	idD1, _ := ctx.exec("new", "Deletable 1")
	idD1 = strings.TrimSpace(idD1)
	ctx.exec("close", idD1)
	idD2, _ := ctx.exec("new", "Deletable 2")
	idD2 = strings.TrimSpace(idD2)
	ctx.exec("close", idD2)
	ctx.exec("link", idD1, idD2)

	// Isolated tickets: one open (n/a), one closed (deletable singleton).
	ctx.exec("new", "Solo open")
	idSolo, _ := ctx.exec("new", "Solo closed")
	idSolo = strings.TrimSpace(idSolo)
	ctx.exec("close", idSolo)

	output, err := ctx.exec("clean", "--components")
	if err != nil {
		t.Fatalf("clean --components command error: %v", err)
	}

	rows := parseComponentRows(t, output)
	if len(rows) != 4 {
		t.Fatalf("expected 4 components, got %d:\n%s", len(rows), output)
	}

	if rows[0].size != 4 || rows[0].open != 1 || rows[0].closed != 3 {
		t.Errorf("first component = %+v; want size 4, open 1, closed 3\n%s", rows[0], output)
	}
	if !strings.HasPrefix(rows[0].status, "anchored") || !strings.Contains(rows[0].status, idOpen) {
		t.Errorf("first component status = %q; want anchored naming %s", rows[0].status, idOpen)
	}
	if rows[1].size != 2 || rows[1].open != 0 || rows[1].closed != 2 || rows[1].status != "deletable" {
		t.Errorf("second component = %+v; want size 2, closed 2, deletable", rows[1])
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].size > rows[i-1].size {
			t.Errorf("components not sorted by size descending: %+v", rows)
			break
		}
	}
	var sawNA, sawDeletable bool
	for _, r := range rows[2:] {
		switch r.status {
		case "n/a":
			sawNA = true
		case "deletable":
			sawDeletable = true
		}
	}
	if !sawNA || !sawDeletable {
		t.Errorf("expected both an n/a and a deletable singleton, got: %+v\n%s", rows, output)
	}
}

// TestCleanComponentsIsolatedDanglingSelfLink - isolated tickets, dangling
// references, and self-links are all handled without panicking and without
// inventing phantom members.
func TestCleanComponentsIsolatedDanglingSelfLink(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	// Self-link: a ticket linked to itself stays a singleton.
	idSelf, _ := ctx.exec("new", "Self linked")
	idSelf = strings.TrimSpace(idSelf)
	ctx.exec("link", idSelf, idSelf)

	// Dangling reference: remove the link target's file after linking.
	idDangling, _ := ctx.exec("new", "Dangling source")
	idDangling = strings.TrimSpace(idDangling)
	idTarget, _ := ctx.exec("new", "Link target")
	idTarget = strings.TrimSpace(idTarget)
	ctx.exec("link", idDangling, idTarget)
	if err := os.Remove(filepath.Join(ctx.ticketsDir, idTarget+".md")); err != nil {
		t.Fatalf("failed to remove link target: %v", err)
	}

	// Truly isolated ticket.
	idIsolated, _ := ctx.exec("new", "Isolated")
	idIsolated = strings.TrimSpace(idIsolated)

	output, err := ctx.exec("clean", "--components")
	if err != nil {
		t.Fatalf("clean --components command error: %v", err)
	}

	rows := parseComponentRows(t, output)
	if len(rows) != 3 {
		t.Fatalf("expected 3 singleton components, got %d:\n%s", len(rows), output)
	}
	for _, r := range rows {
		if r.size != 1 || r.open != 1 || r.closed != 0 || r.status != "n/a" {
			t.Errorf("singleton component = %+v; want size 1, open 1, n/a", r)
		}
	}
	if strings.Contains(output, idTarget) {
		t.Errorf("dangling target %s must not appear as a component member:\n%s", idTarget, output)
	}
}

// TestCleanComponentsJSON - --components --json emits the component view as
// machine-readable JSON and nothing else.
func TestCleanComponentsJSON(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	idClosed, _ := ctx.exec("new", "Closed")
	idClosed = strings.TrimSpace(idClosed)
	ctx.exec("close", idClosed)
	idOpen, _ := ctx.exec("new", "Open")
	idOpen = strings.TrimSpace(idOpen)
	ctx.exec("link", idClosed, idOpen)

	output, err := ctx.exec("clean", "--components", "--json")
	if err != nil {
		t.Fatalf("clean --components --json command error: %v", err)
	}

	var payload struct {
		Components []struct {
			Size    int      `json:"size"`
			Open    int      `json:"open"`
			Closed  int      `json:"closed"`
			Status  string   `json:"status"`
			OpenIDs []string `json:"open_ids"`
		} `json:"components"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("clean --components --json output is not valid JSON: %v\n%s", err, output)
	}
	if len(payload.Components) != 1 {
		t.Fatalf("expected 1 component, got %d: %s", len(payload.Components), output)
	}
	got := payload.Components[0]
	if got.Size != 2 || got.Open != 1 || got.Closed != 1 || got.Status != "anchored" {
		t.Errorf("component = %+v; want size 2, open 1, closed 1, anchored", got)
	}
	if len(got.OpenIDs) != 1 || got.OpenIDs[0] != idOpen {
		t.Errorf("open_ids = %v; want [%s]", got.OpenIDs, idOpen)
	}
	for _, prose := range []string{"Components:", "Found ", "Run with --fix"} {
		if strings.Contains(output, prose) {
			t.Errorf("JSON output must not contain prose %q: %s", prose, output)
		}
	}
}

// TestCleanComponentsDoesNotDelete - --components is a report: it takes
// precedence over --fix and leaves every ticket in place.
func TestCleanComponentsDoesNotDelete(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	id, _ := ctx.exec("new", "Closed")
	id = strings.TrimSpace(id)
	ctx.exec("close", id)

	output, err := ctx.exec("clean", "--components", "--fix")
	if err != nil {
		t.Fatalf("clean --components --fix command error: %v", err)
	}
	if !strings.Contains(output, "Components:") {
		t.Errorf("expected component listing, got: %s", output)
	}
	if _, err := ctx.store().Get(id); err != nil {
		t.Errorf("ticket %s must survive --components --fix: %v", id, err)
	}
}

// TestCleanDefaultOutputUnchanged - without --components the output is the
// ordinary deletion plan and never the component listing.
func TestCleanDefaultOutputUnchanged(t *testing.T) {
	ctx, cleanup := setupTestCmd(t)
	defer cleanup()

	id, _ := ctx.exec("new", "Closed")
	id = strings.TrimSpace(id)
	ctx.exec("close", id)

	output, err := ctx.exec("clean")
	if err != nil {
		t.Fatalf("clean command error: %v", err)
	}
	if strings.Contains(output, "Components:") {
		t.Errorf("default clean output must not contain the component listing: %s", output)
	}
	if !strings.Contains(output, "Found 1 closed ticket(s)") {
		t.Errorf("default clean output changed: %s", output)
	}
}
