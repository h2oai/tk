package cmd

import (
	"strings"
	"testing"

	"github.com/h2oai/tk/internal/ticket"
)

// TestDepCommand tests the dep command (add dependency)
func TestDepCommand(t *testing.T) {
	t.Run("add single dependency", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create two tickets
		id1, _ := ctx.exec("new", "Parent Ticket")
		id1 = strings.TrimSpace(id1)
		id2, _ := ctx.exec("new", "Child Ticket")
		id2 = strings.TrimSpace(id2)

		// Add dependency
		output, err := ctx.exec("dep", id1, id2)
		if err != nil {
			t.Fatalf("dep command error: %v", err)
		}

		// Verify output message
		if !strings.Contains(output, "Added dependency:") {
			t.Errorf("output should contain 'Added dependency:', got: %s", output)
		}
		if !strings.Contains(output, id1) || !strings.Contains(output, id2) {
			t.Errorf("output should contain both IDs, got: %s", output)
		}

		// Verify dependency was added
		parent, _ := ctx.store().Get(id1)
		if len(parent.Deps) != 1 {
			t.Errorf("expected 1 dependency, got %d", len(parent.Deps))
		}
		if parent.Deps[0] != id2 {
			t.Errorf("dependency = %v, want %v", parent.Deps[0], id2)
		}
	})

	t.Run("add multiple dependencies in sequence", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create three tickets
		parent, _ := ctx.exec("new", "Parent")
		parent = strings.TrimSpace(parent)
		dep1, _ := ctx.exec("new", "Dep 1")
		dep1 = strings.TrimSpace(dep1)
		dep2, _ := ctx.exec("new", "Dep 2")
		dep2 = strings.TrimSpace(dep2)

		// Add first dependency
		_, err := ctx.exec("dep", parent, dep1)
		if err != nil {
			t.Fatalf("first dep error: %v", err)
		}

		// Add second dependency
		_, err = ctx.exec("dep", parent, dep2)
		if err != nil {
			t.Fatalf("second dep error: %v", err)
		}

		// Verify both dependencies
		p, _ := ctx.store().Get(parent)
		if len(p.Deps) != 2 {
			t.Errorf("expected 2 dependencies, got %d", len(p.Deps))
		}
		if p.Deps[0] != dep1 || p.Deps[1] != dep2 {
			t.Errorf("deps = %v, want [%s, %s]", p.Deps, dep1, dep2)
		}
	})

	t.Run("add existing dependency is idempotent", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id1, _ := ctx.exec("new", "Ticket 1")
		id1 = strings.TrimSpace(id1)
		id2, _ := ctx.exec("new", "Ticket 2")
		id2 = strings.TrimSpace(id2)

		// Add dependency first time
		_, err := ctx.exec("dep", id1, id2)
		if err != nil {
			t.Fatalf("first dep error: %v", err)
		}

		// Add same dependency again
		output, err := ctx.exec("dep", id1, id2)
		if err != nil {
			t.Fatalf("second dep error: %v", err)
		}

		// Should output "Dependency already exists"
		if !strings.Contains(output, "Dependency already exists") {
			t.Errorf("expected 'Dependency already exists' message, got: %s", output)
		}

		// Verify only one dependency
		parent, _ := ctx.store().Get(id1)
		if len(parent.Deps) != 1 {
			t.Errorf("expected 1 dependency after duplicate add, got %d", len(parent.Deps))
		}
	})

	t.Run("partial IDs resolved", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id1, _ := ctx.exec("new", "Ticket 1")
		id1 = strings.TrimSpace(id1)
		id2, _ := ctx.exec("new", "Ticket 2")
		id2 = strings.TrimSpace(id2)

		// Use partial IDs (the verb-noun portion)
		partial1 := id1[strings.Index(id1, "-")+1:]
		partial2 := id2[strings.Index(id2, "-")+1:]

		// Add dependency with partial IDs
		output, err := ctx.exec("dep", partial1, partial2)
		if err != nil {
			t.Fatalf("dep with partial IDs error: %v", err)
		}

		// Should succeed and show full IDs in output
		if !strings.Contains(output, "Added dependency:") {
			t.Errorf("expected success message, got: %s", output)
		}

		// Verify dependency was added
		parent, _ := ctx.store().Get(id1)
		if len(parent.Deps) != 1 || parent.Deps[0] != id2 {
			t.Errorf("dependency not added correctly with partial IDs")
		}
	})

	t.Run("both tickets must exist", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id1, _ := ctx.exec("new", "Ticket 1")
		id1 = strings.TrimSpace(id1)

		// Try to add non-existent dependency
		_, err := ctx.exec("dep", id1, "nonexistent")
		if err == nil {
			t.Error("expected error for non-existent dependency")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error should mention 'not found', got: %v", err)
		}
	})

	t.Run("parent ticket must exist", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id2, _ := ctx.exec("new", "Ticket 2")
		id2 = strings.TrimSpace(id2)

		// Try to add dependency to non-existent parent
		_, err := ctx.exec("dep", "nonexistent", id2)
		if err == nil {
			t.Error("expected error for non-existent parent")
		}
	})
}

// TestDepArrayFormat tests the formatting of dependency arrays
func TestDepArrayFormat(t *testing.T) {
	t.Run("empty deps to single dep", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id1, _ := ctx.exec("new", "Ticket 1")
		id1 = strings.TrimSpace(id1)
		id2, _ := ctx.exec("new", "Ticket 2")
		id2 = strings.TrimSpace(id2)

		// Add first dependency
		ctx.exec("dep", id1, id2)

		// Check YAML format
		parent, _ := ctx.store().Get(id1)
		if len(parent.Deps) != 1 {
			t.Fatalf("expected 1 dep, got %d", len(parent.Deps))
		}

		// Format should be [dep] with brackets
		expected := id2
		if parent.Deps[0] != expected {
			t.Errorf("deps[0] = %v, want %v", parent.Deps[0], expected)
		}
	})

	t.Run("single dep to multiple deps", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		parent, _ := ctx.exec("new", "Parent")
		parent = strings.TrimSpace(parent)
		dep1, _ := ctx.exec("new", "Dep 1")
		dep1 = strings.TrimSpace(dep1)
		dep2, _ := ctx.exec("new", "Dep 2")
		dep2 = strings.TrimSpace(dep2)

		// Add first dep
		ctx.exec("dep", parent, dep1)

		// Add second dep
		ctx.exec("dep", parent, dep2)

		// Verify format
		p, _ := ctx.store().Get(parent)
		if len(p.Deps) != 2 {
			t.Fatalf("expected 2 deps, got %d", len(p.Deps))
		}

		// Should be comma-space separated
		if p.Deps[0] != dep1 || p.Deps[1] != dep2 {
			t.Errorf("deps = %v, want [%s, %s]", p.Deps, dep1, dep2)
		}
	})
}

// TestUndepCommand tests the undep command (remove dependency)
func TestUndepCommand(t *testing.T) {
	t.Run("remove existing dependency", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id1, _ := ctx.exec("new", "Ticket 1")
		id1 = strings.TrimSpace(id1)
		id2, _ := ctx.exec("new", "Ticket 2")
		id2 = strings.TrimSpace(id2)

		// Add dependency
		ctx.exec("dep", id1, id2)

		// Remove dependency
		output, err := ctx.exec("undep", id1, id2)
		if err != nil {
			t.Fatalf("undep error: %v", err)
		}

		// Verify output message
		if !strings.Contains(output, "Removed dependency:") {
			t.Errorf("output should contain 'Removed dependency:', got: %s", output)
		}
		if !strings.Contains(output, "-/->") {
			t.Errorf("output should contain '-/->', got: %s", output)
		}

		// Verify dependency was removed
		parent, _ := ctx.store().Get(id1)
		if len(parent.Deps) != 0 {
			t.Errorf("expected 0 dependencies after removal, got %d", len(parent.Deps))
		}
	})

	t.Run("remove non-existent dependency", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id1, _ := ctx.exec("new", "Ticket 1")
		id1 = strings.TrimSpace(id1)
		id2, _ := ctx.exec("new", "Ticket 2")
		id2 = strings.TrimSpace(id2)

		// Try to remove non-existent dependency
		_, err := ctx.exec("undep", id1, id2)
		if err == nil {
			t.Error("expected error for non-existent dependency")
		}
		if !strings.Contains(err.Error(), "dependency not found") {
			t.Errorf("error should mention 'dependency not found', got: %v", err)
		}
	})

	t.Run("partial IDs resolved", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id1, _ := ctx.exec("new", "Ticket 1")
		id1 = strings.TrimSpace(id1)
		id2, _ := ctx.exec("new", "Ticket 2")
		id2 = strings.TrimSpace(id2)

		// Add dependency
		ctx.exec("dep", id1, id2)

		// Use partial IDs to remove
		partial1 := id1[strings.Index(id1, "-")+1:]
		partial2 := id2[strings.Index(id2, "-")+1:]

		output, err := ctx.exec("undep", partial1, partial2)
		if err != nil {
			t.Fatalf("undep with partial IDs error: %v", err)
		}

		if !strings.Contains(output, "Removed dependency:") {
			t.Errorf("expected success message, got: %s", output)
		}

		// Verify dependency was removed
		parent, _ := ctx.store().Get(id1)
		if len(parent.Deps) != 0 {
			t.Errorf("expected 0 dependencies after removal, got %d", len(parent.Deps))
		}
	})

	t.Run("remove from multiple deps", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		parent, _ := ctx.exec("new", "Parent")
		parent = strings.TrimSpace(parent)
		dep1, _ := ctx.exec("new", "Dep 1")
		dep1 = strings.TrimSpace(dep1)
		dep2, _ := ctx.exec("new", "Dep 2")
		dep2 = strings.TrimSpace(dep2)

		// Add both dependencies
		ctx.exec("dep", parent, dep1)
		ctx.exec("dep", parent, dep2)

		// Remove first dependency
		_, err := ctx.exec("undep", parent, dep1)
		if err != nil {
			t.Fatalf("undep error: %v", err)
		}

		// Verify only dep2 remains
		p, _ := ctx.store().Get(parent)
		if len(p.Deps) != 1 {
			t.Errorf("expected 1 dependency, got %d", len(p.Deps))
		}
		if p.Deps[0] != dep2 {
			t.Errorf("remaining dep = %v, want %v", p.Deps[0], dep2)
		}
	})

	t.Run("remove last dependency results in empty array", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id1, _ := ctx.exec("new", "Ticket 1")
		id1 = strings.TrimSpace(id1)
		id2, _ := ctx.exec("new", "Ticket 2")
		id2 = strings.TrimSpace(id2)

		// Add and remove dependency
		ctx.exec("dep", id1, id2)
		ctx.exec("undep", id1, id2)

		// Verify empty array
		parent, _ := ctx.store().Get(id1)
		if len(parent.Deps) != 0 {
			t.Errorf("expected 0 dependencies, got %d", len(parent.Deps))
		}
	})
}

// TestDepPersistence tests that dependency changes are persisted
func TestDepPersistence(t *testing.T) {
	t.Run("changes written to disk", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id1, _ := ctx.exec("new", "Ticket 1")
		id1 = strings.TrimSpace(id1)
		id2, _ := ctx.exec("new", "Ticket 2")
		id2 = strings.TrimSpace(id2)

		// Add dependency
		ctx.exec("dep", id1, id2)

		// Create new store instance to verify persistence
		newStore := ticket.NewFileStore(ctx.ticketsDir)
		reloaded, err := newStore.Get(id1)
		if err != nil {
			t.Fatalf("failed to reload ticket: %v", err)
		}

		if len(reloaded.Deps) != 1 || reloaded.Deps[0] != id2 {
			t.Errorf("dependency not persisted correctly")
		}
	})

	t.Run("reloading ticket shows updated deps", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		parent, _ := ctx.exec("new", "Parent")
		parent = strings.TrimSpace(parent)
		dep1, _ := ctx.exec("new", "Dep 1")
		dep1 = strings.TrimSpace(dep1)
		dep2, _ := ctx.exec("new", "Dep 2")
		dep2 = strings.TrimSpace(dep2)

		// Add dependencies
		ctx.exec("dep", parent, dep1)
		ctx.exec("dep", parent, dep2)

		// Remove one
		ctx.exec("undep", parent, dep1)

		// Reload and verify
		newStore := ticket.NewFileStore(ctx.ticketsDir)
		reloaded, err := newStore.Get(parent)
		if err != nil {
			t.Fatalf("failed to reload ticket: %v", err)
		}

		if len(reloaded.Deps) != 1 || reloaded.Deps[0] != dep2 {
			t.Errorf("updated deps not persisted correctly")
		}
	})
}

// TestDepTreeCommand tests the dep tree command
func TestDepTreeCommand(t *testing.T) {
	t.Run("simple linear chain", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create A <- B <- C
		a, _ := ctx.exec("new", "Ticket A")
		a = strings.TrimSpace(a)
		b, _ := ctx.exec("new", "Ticket B")
		b = strings.TrimSpace(b)
		c, _ := ctx.exec("new", "Ticket C")
		c = strings.TrimSpace(c)

		// Add dependencies: B depends on A, C depends on B
		ctx.exec("dep", b, a)
		ctx.exec("dep", c, b)

		// Show tree from C
		output, err := ctx.exec("dep", "tree", c)
		if err != nil {
			t.Fatalf("dep tree error: %v", err)
		}

		// Tree should show C -> B -> A
		if !strings.Contains(output, c) {
			t.Errorf("tree should contain root %s", c)
		}
		if !strings.Contains(output, b) {
			t.Errorf("tree should contain %s", b)
		}
		if !strings.Contains(output, a) {
			t.Errorf("tree should contain %s", a)
		}
	})

	t.Run("single ticket with no deps", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id, _ := ctx.exec("new", "Single Ticket")
		id = strings.TrimSpace(id)

		// Show tree
		output, err := ctx.exec("dep", "tree", id)
		if err != nil {
			t.Fatalf("dep tree error: %v", err)
		}

		// Should just show the ticket itself
		if !strings.Contains(output, id) {
			t.Errorf("tree should contain ticket %s", id)
		}
	})

	t.Run("partial ID resolution", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id, _ := ctx.exec("new", "Test Ticket")
		id = strings.TrimSpace(id)

		// Use partial ID
		partial := id[strings.Index(id, "-")+1:]

		output, err := ctx.exec("dep", "tree", partial)
		if err != nil {
			t.Fatalf("dep tree with partial ID error: %v", err)
		}

		// Should resolve and show full ID
		if !strings.Contains(output, id) {
			t.Errorf("tree should contain full ID %s", id)
		}
	})

	t.Run("tree shows status", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id, _ := ctx.exec("new", "Test Ticket")
		id = strings.TrimSpace(id)

		output, err := ctx.exec("dep", "tree", id)
		if err != nil {
			t.Fatalf("dep tree error: %v", err)
		}

		// Should show status in brackets
		if !strings.Contains(output, "[open]") {
			t.Errorf("tree should show status [open], got: %s", output)
		}
	})

	t.Run("tree shows title", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		title := "My Special Ticket"
		id, _ := ctx.exec("new", title)
		id = strings.TrimSpace(id)

		output, err := ctx.exec("dep", "tree", id)
		if err != nil {
			t.Fatalf("dep tree error: %v", err)
		}

		// Should show title
		if !strings.Contains(output, title) {
			t.Errorf("tree should show title '%s', got: %s", title, output)
		}
	})
}

// TestDepTreeCommandFull tests the --full flag for dep tree
func TestDepTreeCommandFull(t *testing.T) {
	t.Run("full flag shows all occurrences", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create diamond dependency structure
		//      D
		//     / \
		//    B   C
		//     \ /
		//      A
		a, _ := ctx.exec("new", "Ticket A")
		a = strings.TrimSpace(a)
		b, _ := ctx.exec("new", "Ticket B")
		b = strings.TrimSpace(b)
		c, _ := ctx.exec("new", "Ticket C")
		c = strings.TrimSpace(c)
		d, _ := ctx.exec("new", "Ticket D")
		d = strings.TrimSpace(d)

		// Set up diamond: D->B->A, D->C->A
		ctx.exec("dep", b, a)
		ctx.exec("dep", c, a)
		ctx.exec("dep", d, b)
		ctx.exec("dep", d, c)

		// Show tree with --full flag
		output, err := ctx.exec("dep", "tree", "--full", d)
		if err != nil {
			t.Fatalf("dep tree --full error: %v", err)
		}

		// In full mode, A should appear twice (under both B and C)
		// Count occurrences of A in output
		count := strings.Count(output, a)
		if count < 2 {
			t.Errorf("with --full, ticket A should appear at least twice, appeared %d times", count)
		}
	})
}

// TestDepCommandOutput tests the output format of dep commands
func TestDepCommandOutput(t *testing.T) {
	t.Run("dep output format", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id1, _ := ctx.exec("new", "Ticket 1")
		id1 = strings.TrimSpace(id1)
		id2, _ := ctx.exec("new", "Ticket 2")
		id2 = strings.TrimSpace(id2)

		output, _ := ctx.exec("dep", id1, id2)

		// Format should be: "Added dependency: {id} -> {dep-id}"
		if !strings.Contains(output, "Added dependency:") {
			t.Errorf("missing 'Added dependency:' in output")
		}
		if !strings.Contains(output, "->") {
			t.Errorf("missing '->' in output")
		}
	})

	t.Run("undep output format", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id1, _ := ctx.exec("new", "Ticket 1")
		id1 = strings.TrimSpace(id1)
		id2, _ := ctx.exec("new", "Ticket 2")
		id2 = strings.TrimSpace(id2)

		ctx.exec("dep", id1, id2)
		output, _ := ctx.exec("undep", id1, id2)

		// Format should be: "Removed dependency: {id} -/-> {dep-id}"
		if !strings.Contains(output, "Removed dependency:") {
			t.Errorf("missing 'Removed dependency:' in output")
		}
		if !strings.Contains(output, "-/->") {
			t.Errorf("missing '-/->' in output")
		}
	})
}
