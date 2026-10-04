package cmd

import (
	"strings"
	"testing"
	"time"
)

// TestListCommand tests the list command
func TestListCommand(t *testing.T) {
	t.Run("list all tickets", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create tickets with different statuses
		id1, _ := ctx.exec("new", "Open Ticket")
		id1 = strings.TrimSpace(id1)

		id2, _ := ctx.exec("new", "Closed Ticket")
		id2 = strings.TrimSpace(id2)
		ctx.exec("close", id2)

		id3, _ := ctx.exec("new", "InProgress Ticket")
		id3 = strings.TrimSpace(id3)
		ctx.exec("start", id3)

		output, _ := ctx.exec("ls")

		// All tickets should appear
		if !strings.Contains(output, id1) {
			t.Errorf("list should include open ticket %s", id1)
		}
		if !strings.Contains(output, id2) {
			t.Errorf("list should include closed ticket %s", id2)
		}
		if !strings.Contains(output, id3) {
			t.Errorf("list should include in_progress ticket %s", id3)
		}
	})

	t.Run("list alias works", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id, _ := ctx.exec("new", "Test")
		id = strings.TrimSpace(id)

		// Try 'list' command (alias of 'ls')
		output, _ := ctx.exec("list")

		if !strings.Contains(output, id) {
			t.Error("list alias should work")
		}
	})
}

// TestListByStatus tests filtering by status
func TestListByStatus(t *testing.T) {
	t.Run("filter by open status", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idOpen, _ := ctx.exec("new", "Open Ticket")
		idOpen = strings.TrimSpace(idOpen)

		idClosed, _ := ctx.exec("new", "Closed Ticket")
		idClosed = strings.TrimSpace(idClosed)
		ctx.exec("close", idClosed)

		output, _ := ctx.exec("ls", "--status=open")

		// Only open ticket should appear
		if !strings.Contains(output, idOpen) {
			t.Error("should include open ticket")
		}
		if strings.Contains(output, idClosed) {
			t.Error("should not include closed ticket when filtering by open")
		}
	})

	t.Run("filter by in_progress status", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idOpen, _ := ctx.exec("new", "Open Ticket")
		idOpen = strings.TrimSpace(idOpen)

		idInProgress, _ := ctx.exec("new", "InProgress Ticket")
		idInProgress = strings.TrimSpace(idInProgress)
		ctx.exec("start", idInProgress)

		output, _ := ctx.exec("ls", "--status=in_progress")

		// Only in_progress ticket should appear
		if !strings.Contains(output, idInProgress) {
			t.Error("should include in_progress ticket")
		}
		if strings.Contains(output, idOpen) {
			t.Error("should not include open ticket when filtering by in_progress")
		}
	})

	t.Run("filter by closed status", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idOpen, _ := ctx.exec("new", "Open Ticket")
		idOpen = strings.TrimSpace(idOpen)

		idClosed, _ := ctx.exec("new", "Closed Ticket")
		idClosed = strings.TrimSpace(idClosed)
		ctx.exec("close", idClosed)

		output, _ := ctx.exec("ls", "--status=closed")

		// Only closed ticket should appear
		if !strings.Contains(output, idClosed) {
			t.Error("should include closed ticket")
		}
		if strings.Contains(output, idOpen) {
			t.Error("should not include open ticket when filtering by closed")
		}
	})
}

// TestListSorting tests sorting behavior
func TestListSorting(t *testing.T) {
	t.Run("consistent ordering", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create multiple tickets
		id1, _ := ctx.exec("new", "Ticket 1")
		id1 = strings.TrimSpace(id1)

		id2, _ := ctx.exec("new", "Ticket 2")
		id2 = strings.TrimSpace(id2)

		id3, _ := ctx.exec("new", "Ticket 3")
		id3 = strings.TrimSpace(id3)

		// Run list multiple times
		output1, _ := ctx.exec("ls")
		output2, _ := ctx.exec("ls")

		// Output should be consistent
		if output1 != output2 {
			t.Error("list output should be consistent across calls")
		}
	})

	t.Run("default sort is newest first", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id1, _ := ctx.exec("new", "First")
		id1 = strings.TrimSpace(id1)

		id2, _ := ctx.exec("new", "Second")
		id2 = strings.TrimSpace(id2)

		// Explicitly set timestamps so id2 is definitively newer
		now := time.Now()
		s := ctx.store()
		t1, _ := s.Get(id1)
		t2, _ := s.Get(id2)
		t1.Created = now.Add(-1 * time.Hour)
		t2.Created = now
		s.Update(t1)
		s.Update(t2)

		output, _ := ctx.exec("ls")

		pos1 := strings.Index(output, id1)
		pos2 := strings.Index(output, id2)

		if pos1 == -1 || pos2 == -1 {
			t.Fatal("both tickets should be in output")
		}

		if pos2 > pos1 {
			t.Error("newer ticket (id2) should appear before older ticket (id1) with default date sort")
		}
	})
}

// TestListOutputFormat tests the output format
func TestListOutputFormat(t *testing.T) {
	t.Run("output format without deps", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id, _ := ctx.exec("new", "Test Ticket")
		id = strings.TrimSpace(id)

		output, _ := ctx.exec("ls")

		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) == 0 {
			t.Fatal("expected output")
		}

		line := lines[0]

		// Format: "%-8s [P%d][%s][%s] - %s"
		// Should contain ID
		if !strings.Contains(line, id) {
			t.Error("line should contain ID")
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

		// Should contain dash separator
		if !strings.Contains(line, " - ") {
			t.Errorf("line should contain ' - ' separator, got: %s", line)
		}
	})

	t.Run("shows ID, status, and title", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id, _ := ctx.exec("new", "My Ticket")
		id = strings.TrimSpace(id)

		output, _ := ctx.exec("ls")

		// All components should be present
		if !strings.Contains(output, id) {
			t.Error("output should contain ID")
		}
		if !strings.Contains(output, "[open]") {
			t.Error("output should contain [open]")
		}
		if !strings.Contains(output, "My Ticket") {
			t.Error("output should contain title")
		}
	})
}

// TestListDepsDisplay tests display of dependencies
func TestListDepsDisplay(t *testing.T) {
	t.Run("shows deps inline", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		dep, _ := ctx.exec("new", "Dependency")
		dep = strings.TrimSpace(dep)

		parent, _ := ctx.exec("new", "Parent")
		parent = strings.TrimSpace(parent)
		ctx.exec("dep", parent, dep)

		output, _ := ctx.exec("ls")

		lines := strings.Split(output, "\n")

		// Find parent line
		var parentLine string
		for _, line := range lines {
			if strings.Contains(line, parent) && strings.Contains(line, "Parent") {
				parentLine = line
				break
			}
		}

		if parentLine == "" {
			t.Fatal("could not find parent line in output")
		}

		// Should show deps with arrow
		if !strings.Contains(parentLine, " <- ") {
			t.Errorf("line should contain ' <- ' arrow, got: %s", parentLine)
		}

		// Should show dep ID
		if !strings.Contains(parentLine, dep) {
			t.Errorf("line should contain dep ID %s, got: %s", dep, parentLine)
		}

		// Deps should be in brackets
		if !strings.Contains(parentLine, "["+dep) {
			t.Errorf("deps should be in brackets, got: %s", parentLine)
		}
	})

	t.Run("shows multiple deps", func(t *testing.T) {
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

		output, _ := ctx.exec("ls")

		// Find parent line
		var parentLine string
		for _, line := range strings.Split(output, "\n") {
			if strings.Contains(line, parent) && strings.Contains(line, "Parent") {
				parentLine = line
				break
			}
		}

		// Should show both deps
		if !strings.Contains(parentLine, dep1) {
			t.Errorf("should show dep1 %s", dep1)
		}
		if !strings.Contains(parentLine, dep2) {
			t.Errorf("should show dep2 %s", dep2)
		}

		// Deps should be comma-separated
		if !strings.Contains(parentLine, ", ") {
			t.Error("multiple deps should be comma-separated")
		}
	})

	t.Run("no deps arrow when no deps", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id, _ := ctx.exec("new", "No Deps")
		id = strings.TrimSpace(id)

		output, _ := ctx.exec("ls")

		lines := strings.Split(output, "\n")

		// Find ticket line
		var ticketLine string
		for _, line := range lines {
			if strings.Contains(line, id) {
				ticketLine = line
				break
			}
		}

		// Should not show arrow when no deps
		if strings.Contains(ticketLine, " <- ") {
			t.Errorf("should not show arrow when no deps, got: %s", ticketLine)
		}
	})
}

// TestListEmptyResult tests empty output
func TestListEmptyResult(t *testing.T) {
	t.Run("no matching tickets returns empty", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create only open tickets
		ctx.exec("new", "Open Ticket")

		// Filter by closed (should be empty)
		output, _ := ctx.exec("ls", "--status=closed")

		trimmed := strings.TrimSpace(output)
		if trimmed != "" {
			t.Errorf("expected empty output, got: %s", output)
		}
	})

	t.Run("no tickets returns empty", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		output, _ := ctx.exec("ls")

		trimmed := strings.TrimSpace(output)
		if trimmed != "" {
			t.Errorf("expected empty output when no tickets, got: %s", output)
		}
	})
}

// TestListMissingTicketsDirectory tests handling of missing directory
func TestListMissingTicketsDirectory(t *testing.T) {
	t.Run("missing directory returns empty silently", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		output, err := ctx.exec("ls")
		if err != nil {
			t.Fatalf("list should not error on missing directory: %v", err)
		}

		trimmed := strings.TrimSpace(output)
		if trimmed != "" {
			t.Errorf("expected empty output for missing directory, got: %s", output)
		}
	})
}

// TestListSortFlag tests the --sort flag
func TestListSortFlag(t *testing.T) {
	t.Run("sort date explicit newest first", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id1, _ := ctx.exec("new", "Older")
		id1 = strings.TrimSpace(id1)

		id2, _ := ctx.exec("new", "Newer")
		id2 = strings.TrimSpace(id2)

		// Explicitly set timestamps so ordering is unambiguous
		now := time.Now()
		s := ctx.store()
		t1, _ := s.Get(id1)
		t2, _ := s.Get(id2)
		t1.Created = now.Add(-1 * time.Hour)
		t2.Created = now
		s.Update(t1)
		s.Update(t2)

		output, _ := ctx.exec("ls", "--sort", "date")

		pos1 := strings.Index(output, id1)
		pos2 := strings.Index(output, id2)

		if pos1 == -1 || pos2 == -1 {
			t.Fatal("both tickets should be in output")
		}

		if pos2 > pos1 {
			t.Error("newer ticket should appear first with --sort date")
		}
	})

	t.Run("sort priority ascending", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id0, _ := ctx.exec("new", "High", "--priority", "0")
		id0 = strings.TrimSpace(id0)

		id4, _ := ctx.exec("new", "Low", "--priority", "4")
		id4 = strings.TrimSpace(id4)

		output, _ := ctx.exec("ls", "--sort", "priority")

		pos0 := strings.Index(output, id0)
		pos4 := strings.Index(output, id4)

		if pos0 == -1 || pos4 == -1 {
			t.Fatal("both tickets should be in output")
		}

		if pos0 > pos4 {
			t.Error("priority 0 should appear before priority 4")
		}
	})

	t.Run("sort status alphabetical", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		idOpen, _ := ctx.exec("new", "Open Ticket")
		idOpen = strings.TrimSpace(idOpen)

		idClosed, _ := ctx.exec("new", "Closed Ticket")
		idClosed = strings.TrimSpace(idClosed)
		ctx.exec("close", idClosed)

		output, _ := ctx.exec("ls", "--sort", "status")

		posOpen := strings.Index(output, idOpen)
		posClosed := strings.Index(output, idClosed)

		if posOpen == -1 || posClosed == -1 {
			t.Fatal("both tickets should be in output")
		}

		// "closed" < "open" alphabetically
		if posClosed > posOpen {
			t.Error("closed status should appear before open status with --sort status")
		}
	})

	t.Run("sort invalid field returns error", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		ctx.exec("new", "Test")

		_, err := ctx.exec("ls", "--sort", "bad")
		if err == nil {
			t.Error("expected error for invalid sort field")
		}
	})

	t.Run("default same as sort date", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		ctx.exec("new", "First")
		ctx.exec("new", "Second")

		outputDefault, _ := ctx.exec("ls")
		outputDate, _ := ctx.exec("ls", "--sort", "date")

		if outputDefault != outputDate {
			t.Error("default output should match --sort date output")
		}
	})
}

// TestListOnePerLine tests that each ticket is on its own line
func TestListOnePerLine(t *testing.T) {
	t.Run("one ticket per line", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create three tickets
		ctx.exec("new", "Ticket 1")
		ctx.exec("new", "Ticket 2")
		ctx.exec("new", "Ticket 3")

		output, _ := ctx.exec("ls")

		lines := strings.Split(strings.TrimSpace(output), "\n")

		if len(lines) != 3 {
			t.Errorf("expected 3 lines for 3 tickets, got %d", len(lines))
		}
	})
}

// TestListStatusDisplay tests that all statuses are displayed correctly
func TestListStatusDisplay(t *testing.T) {
	t.Run("displays all status types", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create tickets with different statuses
		idOpen, _ := ctx.exec("new", "Open")
		idOpen = strings.TrimSpace(idOpen)

		idInProgress, _ := ctx.exec("new", "InProgress")
		idInProgress = strings.TrimSpace(idInProgress)
		ctx.exec("start", idInProgress)

		idClosed, _ := ctx.exec("new", "Closed")
		idClosed = strings.TrimSpace(idClosed)
		ctx.exec("close", idClosed)

		output, _ := ctx.exec("ls")

		// All statuses should be displayed
		if !strings.Contains(output, "[open]") {
			t.Error("should display [open] status")
		}
		if !strings.Contains(output, "[in_progress]") {
			t.Error("should display [in_progress] status")
		}
		if !strings.Contains(output, "[closed]") {
			t.Error("should display [closed] status")
		}
	})
}
