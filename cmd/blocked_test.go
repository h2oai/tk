package cmd

import (
	"strings"
	"testing"
	"time"
)

// TestBlockedCommand tests the blocked command
func TestBlockedCommand(t *testing.T) {
	t.Run("filter open tickets only", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create open dep
		dep, _ := ctx.exec("new", "Open Dep")
		dep = strings.TrimSpace(dep)

		// Create open ticket blocked by dep
		parent, _ := ctx.exec("new", "Blocked Ticket")
		parent = strings.TrimSpace(parent)
		ctx.exec("dep", parent, dep)

		// Create closed ticket (should not appear even if it has deps)
		closedParent, _ := ctx.exec("new", "Closed Parent")
		closedParent = strings.TrimSpace(closedParent)
		ctx.exec("dep", closedParent, dep)
		ctx.exec("close", closedParent)

		output, _ := ctx.exec("blocked")

		// Open blocked ticket should appear
		if !strings.Contains(output, parent) {
			t.Errorf("blocked should include open blocked ticket %s", parent)
		}

		// Closed ticket should not appear
		if strings.Contains(output, closedParent) {
			t.Errorf("blocked should not include closed ticket %s", closedParent)
		}
	})

	t.Run("filter in_progress tickets", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create open dep
		dep, _ := ctx.exec("new", "Dep")
		dep = strings.TrimSpace(dep)

		// Create in_progress ticket blocked by dep
		parent, _ := ctx.exec("new", "In Progress Blocked")
		parent = strings.TrimSpace(parent)
		ctx.exec("dep", parent, dep)
		ctx.exec("start", parent)

		output, _ := ctx.exec("blocked")

		// in_progress blocked ticket should appear
		if !strings.Contains(output, parent) {
			t.Errorf("blocked should include in_progress blocked ticket %s", parent)
		}
		if !strings.Contains(output, "[in_progress]") {
			t.Error("should show [in_progress] status")
		}
	})
}

// TestBlockedUnresolvedDeps tests filtering by unresolved dependencies
func TestBlockedUnresolvedDeps(t *testing.T) {
	t.Run("ticket with no deps is not blocked", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id, _ := ctx.exec("new", "No Deps")
		id = strings.TrimSpace(id)

		output, _ := ctx.exec("blocked")

		if strings.Contains(output, id) {
			t.Error("ticket with no deps should not be blocked")
		}
	})

	t.Run("ticket with all deps closed is not blocked", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create and close dep
		dep, _ := ctx.exec("new", "Closed Dep")
		dep = strings.TrimSpace(dep)
		ctx.exec("close", dep)

		// Create parent depending on closed dep
		parent, _ := ctx.exec("new", "Parent")
		parent = strings.TrimSpace(parent)
		ctx.exec("dep", parent, dep)

		output, _ := ctx.exec("blocked")

		if strings.Contains(output, parent) {
			t.Error("ticket with all closed deps should not be blocked")
		}
	})

	t.Run("ticket with any open dep is blocked", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create open dep
		dep, _ := ctx.exec("new", "Open Dep")
		dep = strings.TrimSpace(dep)

		// Create parent depending on open dep
		parent, _ := ctx.exec("new", "Parent")
		parent = strings.TrimSpace(parent)
		ctx.exec("dep", parent, dep)

		output, _ := ctx.exec("blocked")

		if !strings.Contains(output, parent) {
			t.Errorf("ticket with open dep should be blocked")
		}
	})

	t.Run("ticket with one open dep among many is blocked", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create closed dep
		dep1, _ := ctx.exec("new", "Closed Dep")
		dep1 = strings.TrimSpace(dep1)
		ctx.exec("close", dep1)

		// Create open dep
		dep2, _ := ctx.exec("new", "Open Dep")
		dep2 = strings.TrimSpace(dep2)

		// Create parent depending on both
		parent, _ := ctx.exec("new", "Parent")
		parent = strings.TrimSpace(parent)
		ctx.exec("dep", parent, dep1)
		ctx.exec("dep", parent, dep2)

		output, _ := ctx.exec("blocked")

		if !strings.Contains(output, parent) {
			t.Error("ticket with any open dep should be blocked")
		}
	})
}

// TestBlockedShowBlockers tests showing unclosed blockers
func TestBlockedShowBlockers(t *testing.T) {
	t.Run("shows unclosed blockers only", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create two deps: one closed, one open
		closedDep, _ := ctx.exec("new", "Closed Dep")
		closedDep = strings.TrimSpace(closedDep)
		ctx.exec("close", closedDep)

		openDep, _ := ctx.exec("new", "Open Dep")
		openDep = strings.TrimSpace(openDep)

		// Create parent depending on both
		parent, _ := ctx.exec("new", "Parent")
		parent = strings.TrimSpace(parent)
		ctx.exec("dep", parent, closedDep)
		ctx.exec("dep", parent, openDep)

		output, _ := ctx.exec("blocked")

		// Should show only the open blocker
		if !strings.Contains(output, openDep) {
			t.Errorf("should show open blocker %s", openDep)
		}

		// Should NOT show closed blocker in the blockers list
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, parent) {
				if strings.Contains(line, closedDep) {
					t.Errorf("should not show closed dep %s in blockers list", closedDep)
				}
			}
		}
	})

	t.Run("format shows which tickets blocking", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		dep, _ := ctx.exec("new", "Blocker")
		dep = strings.TrimSpace(dep)

		parent, _ := ctx.exec("new", "Blocked")
		parent = strings.TrimSpace(parent)
		ctx.exec("dep", parent, dep)

		output, _ := ctx.exec("blocked")

		// Should show the blocker ID
		lines := strings.Split(output, "\n")
		found := false
		for _, line := range lines {
			if strings.Contains(line, parent) {
				if strings.Contains(line, dep) {
					found = true
				}
			}
		}

		if !found {
			t.Errorf("output should show which ticket is blocking, got: %s", output)
		}
	})

	t.Run("shows multiple blockers", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		dep1, _ := ctx.exec("new", "Blocker 1")
		dep1 = strings.TrimSpace(dep1)

		dep2, _ := ctx.exec("new", "Blocker 2")
		dep2 = strings.TrimSpace(dep2)

		parent, _ := ctx.exec("new", "Blocked")
		parent = strings.TrimSpace(parent)
		ctx.exec("dep", parent, dep1)
		ctx.exec("dep", parent, dep2)

		output, _ := ctx.exec("blocked")

		// Should show both blockers
		if !strings.Contains(output, dep1) {
			t.Errorf("should show first blocker %s", dep1)
		}
		if !strings.Contains(output, dep2) {
			t.Errorf("should show second blocker %s", dep2)
		}
	})
}

// TestBlockedSorting tests sorting by priority and ID
func TestBlockedSorting(t *testing.T) {
	t.Run("sort by priority ascending", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create open dep
		dep, _ := ctx.exec("new", "Dep")
		dep = strings.TrimSpace(dep)

		// Create blocked tickets with different priorities
		id0, _ := ctx.exec("new", "Priority 0", "--priority", "0")
		id0 = strings.TrimSpace(id0)
		ctx.exec("dep", id0, dep)

		id2, _ := ctx.exec("new", "Priority 2", "--priority", "2")
		id2 = strings.TrimSpace(id2)
		ctx.exec("dep", id2, dep)

		id4, _ := ctx.exec("new", "Priority 4", "--priority", "4")
		id4 = strings.TrimSpace(id4)
		ctx.exec("dep", id4, dep)

		output, _ := ctx.exec("blocked", "--sort", "priority")

		// Find positions
		pos0 := strings.Index(output, id0)
		pos2 := strings.Index(output, id2)
		pos4 := strings.Index(output, id4)

		if pos0 == -1 || pos2 == -1 || pos4 == -1 {
			t.Fatal("all blocked tickets should be in output")
		}

		// Priority 0 should come first
		if pos0 > pos2 || pos0 > pos4 {
			t.Errorf("priority 0 should come first, got positions: 0=%d, 2=%d, 4=%d", pos0, pos2, pos4)
		}

		// Priority 4 should come last
		if pos4 < pos0 || pos4 < pos2 {
			t.Errorf("priority 4 should come last, got positions: 0=%d, 2=%d, 4=%d", pos0, pos2, pos4)
		}
	})

	t.Run("secondary sort by ID", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		dep, _ := ctx.exec("new", "Dep")
		dep = strings.TrimSpace(dep)

		// Create two blocked tickets with same priority
		id1, _ := ctx.exec("new", "First", "--priority", "2")
		id1 = strings.TrimSpace(id1)
		ctx.exec("dep", id1, dep)

		id2, _ := ctx.exec("new", "Second", "--priority", "2")
		id2 = strings.TrimSpace(id2)
		ctx.exec("dep", id2, dep)

		output, _ := ctx.exec("blocked", "--sort", "priority")

		// Both should be in output
		if !strings.Contains(output, id1) || !strings.Contains(output, id2) {
			t.Error("both tickets should be in output")
		}

		// Verify sorted by ID
		pos1 := strings.Index(output, id1)
		pos2 := strings.Index(output, id2)

		if id1 < id2 {
			if pos1 > pos2 {
				t.Error("tickets with same priority should be sorted by ID")
			}
		} else {
			if pos2 > pos1 {
				t.Error("tickets with same priority should be sorted by ID")
			}
		}
	})
}

// TestBlockedSortFlag tests the --sort flag on the blocked command
func TestBlockedSortFlag(t *testing.T) {
	t.Run("sort priority", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		dep, _ := ctx.exec("new", "Dep")
		dep = strings.TrimSpace(dep)

		id0, _ := ctx.exec("new", "High", "--priority", "0")
		id0 = strings.TrimSpace(id0)
		ctx.exec("dep", id0, dep)

		id4, _ := ctx.exec("new", "Low", "--priority", "4")
		id4 = strings.TrimSpace(id4)
		ctx.exec("dep", id4, dep)

		output, _ := ctx.exec("blocked", "--sort", "priority")

		pos0 := strings.Index(output, id0)
		pos4 := strings.Index(output, id4)

		if pos0 == -1 || pos4 == -1 {
			t.Fatal("both tickets should be in output")
		}

		if pos0 > pos4 {
			t.Error("priority 0 should appear before priority 4")
		}
	})

	t.Run("sort date newest first", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		dep, _ := ctx.exec("new", "Dep")
		dep = strings.TrimSpace(dep)

		id1, _ := ctx.exec("new", "Older")
		id1 = strings.TrimSpace(id1)
		ctx.exec("dep", id1, dep)

		id2, _ := ctx.exec("new", "Newer")
		id2 = strings.TrimSpace(id2)
		ctx.exec("dep", id2, dep)

		// Explicitly set timestamps so ordering is unambiguous
		now := time.Now()
		s := ctx.store()
		t1, _ := s.Get(id1)
		t2, _ := s.Get(id2)
		t1.Created = now.Add(-1 * time.Hour)
		t2.Created = now
		s.Update(t1)
		s.Update(t2)

		output, _ := ctx.exec("blocked", "--sort", "date")

		pos1 := strings.Index(output, id1)
		pos2 := strings.Index(output, id2)

		if pos1 == -1 || pos2 == -1 {
			t.Fatal("both tickets should be in output")
		}

		if pos2 > pos1 {
			t.Error("newer ticket should appear first with --sort date")
		}
	})

	t.Run("sort invalid returns error", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		dep, _ := ctx.exec("new", "Dep")
		dep = strings.TrimSpace(dep)

		id, _ := ctx.exec("new", "Test")
		id = strings.TrimSpace(id)
		ctx.exec("dep", id, dep)

		_, err := ctx.exec("blocked", "--sort", "bad")
		if err == nil {
			t.Error("expected error for invalid sort field")
		}
	})
}

// TestBlockedOutputFormat tests the output format
func TestBlockedOutputFormat(t *testing.T) {
	t.Run("output format with blockers", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		dep, _ := ctx.exec("new", "Blocker")
		dep = strings.TrimSpace(dep)

		parent, _ := ctx.exec("new", "Test Ticket", "--priority", "1")
		parent = strings.TrimSpace(parent)
		ctx.exec("dep", parent, dep)

		output, _ := ctx.exec("blocked")

		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) == 0 {
			t.Fatal("expected output")
		}

		line := lines[0]

		// Format: "%-8s [P%d][%s][%s] - %s <- %s"
		// Should contain ID
		if !strings.Contains(line, parent) {
			t.Error("line should contain parent ID")
		}

		// Should contain priority
		if !strings.Contains(line, "[P1]") {
			t.Errorf("line should contain [P1], got: %s", line)
		}

		// Should contain status
		if !strings.Contains(line, "[open]") {
			t.Errorf("line should contain [open], got: %s", line)
		}

		// Should contain type (default is task)
		if !strings.Contains(line, "[task]") {
			t.Errorf("line should contain [task], got: %s", line)
		}

		// Should contain title
		if !strings.Contains(line, "Test Ticket") {
			t.Errorf("line should contain title, got: %s", line)
		}

		// Should contain blocker arrow
		if !strings.Contains(line, " <- ") {
			t.Errorf("line should contain ' <- ' arrow, got: %s", line)
		}

		// Should contain blocker ID
		if !strings.Contains(line, dep) {
			t.Errorf("line should contain blocker ID %s, got: %s", dep, line)
		}

		// Blockers should be in brackets
		if !strings.Contains(line, "["+dep) {
			t.Errorf("blocker should be in brackets, got: %s", line)
		}
	})

	t.Run("shows ID, priority, status, title, and blockers", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		dep, _ := ctx.exec("new", "Dep")
		dep = strings.TrimSpace(dep)

		id, _ := ctx.exec("new", "My Ticket", "--priority", "3")
		id = strings.TrimSpace(id)
		ctx.exec("dep", id, dep)

		output, _ := ctx.exec("blocked")

		// All components should be present
		if !strings.Contains(output, id) {
			t.Error("output should contain ID")
		}
		if !strings.Contains(output, "[P3]") {
			t.Error("output should contain [P3]")
		}
		if !strings.Contains(output, "[open]") {
			t.Error("output should contain [open]")
		}
		if !strings.Contains(output, "My Ticket") {
			t.Error("output should contain title")
		}
		if !strings.Contains(output, dep) {
			t.Error("output should contain blocker ID")
		}
	})

	t.Run("blocker format with brackets", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		dep1, _ := ctx.exec("new", "Dep 1")
		dep1 = strings.TrimSpace(dep1)

		dep2, _ := ctx.exec("new", "Dep 2")
		dep2 = strings.TrimSpace(dep2)

		parent, _ := ctx.exec("new", "Parent")
		parent = strings.TrimSpace(parent)
		ctx.exec("dep", parent, dep1)
		ctx.exec("dep", parent, dep2)

		output, _ := ctx.exec("blocked")

		// Blockers should be in format [dep1, dep2]
		if !strings.Contains(output, "[") || !strings.Contains(output, "]") {
			t.Error("blockers should be in brackets")
		}

		// Should contain both deps
		if !strings.Contains(output, dep1) || !strings.Contains(output, dep2) {
			t.Error("should show all blockers")
		}
	})
}

// TestBlockedEmptyResult tests empty output
func TestBlockedEmptyResult(t *testing.T) {
	t.Run("no blocked tickets returns empty", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create ticket with no deps
		ctx.exec("new", "Unblocked Ticket")

		output, _ := ctx.exec("blocked")

		// Output should be empty
		trimmed := strings.TrimSpace(output)
		if trimmed != "" {
			t.Errorf("expected empty output, got: %s", output)
		}
	})

	t.Run("only ready tickets returns empty", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create dep and close it
		dep, _ := ctx.exec("new", "Closed Dep")
		dep = strings.TrimSpace(dep)
		ctx.exec("close", dep)

		// Create parent depending on closed dep
		parent, _ := ctx.exec("new", "Ready Ticket")
		parent = strings.TrimSpace(parent)
		ctx.exec("dep", parent, dep)

		output, _ := ctx.exec("blocked")

		// Output should be empty (no blocked tickets)
		trimmed := strings.TrimSpace(output)
		if trimmed != "" {
			t.Errorf("expected empty output when all deps closed, got: %s", output)
		}
	})

	t.Run("only closed tickets returns empty", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create open dep
		dep, _ := ctx.exec("new", "Dep")
		dep = strings.TrimSpace(dep)

		// Create parent, add dep, then close parent
		parent, _ := ctx.exec("new", "Closed Parent")
		parent = strings.TrimSpace(parent)
		ctx.exec("dep", parent, dep)
		ctx.exec("close", parent)

		output, _ := ctx.exec("blocked")

		// Closed ticket should not appear
		if strings.Contains(output, parent) {
			t.Error("closed ticket should not appear in blocked")
		}
	})
}

// TestBlockedMissingTicketsDirectory tests handling of missing directory
func TestBlockedMissingTicketsDirectory(t *testing.T) {
	t.Run("missing directory returns empty silently", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		output, err := ctx.exec("blocked")
		if err != nil {
			t.Fatalf("blocked should not error on missing directory: %v", err)
		}

		// Should return empty output
		trimmed := strings.TrimSpace(output)
		if trimmed != "" {
			t.Errorf("expected empty output for missing directory, got: %s", output)
		}
	})
}

// TestBlockedDefaultPriority tests handling of default priority
func TestBlockedDefaultPriority(t *testing.T) {
	t.Run("default priority is 2", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		dep, _ := ctx.exec("new", "Dep")
		dep = strings.TrimSpace(dep)

		// Create blocked ticket without specifying priority
		id, _ := ctx.exec("new", "Default Priority")
		id = strings.TrimSpace(id)
		ctx.exec("dep", id, dep)

		output, _ := ctx.exec("blocked")

		// Should show [P2] for default priority
		if !strings.Contains(output, "[P2]") {
			t.Errorf("should show default priority [P2], got: %s", output)
		}
	})
}
