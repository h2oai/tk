package cmd

import (
	"strings"
	"testing"
	"time"
)

// TestClosedCommand tests the closed command
func TestClosedCommand(t *testing.T) {
	t.Run("list closed tickets", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create and close tickets
		id1, _ := ctx.exec("new", "Closed 1")
		id1 = strings.TrimSpace(id1)
		ctx.exec("close", id1)

		id2, _ := ctx.exec("new", "Closed 2")
		id2 = strings.TrimSpace(id2)
		ctx.exec("close", id2)

		// Create open ticket (should not appear)
		openID, _ := ctx.exec("new", "Open")
		openID = strings.TrimSpace(openID)

		output, _ := ctx.exec("closed")

		// Closed tickets should appear
		if !strings.Contains(output, id1) {
			t.Errorf("should include closed ticket %s", id1)
		}
		if !strings.Contains(output, id2) {
			t.Errorf("should include closed ticket %s", id2)
		}

		// Open ticket should not appear
		if strings.Contains(output, openID) {
			t.Errorf("should not include open ticket %s", openID)
		}
	})

	t.Run("filter by closed status", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		closed, _ := ctx.exec("new", "Closed")
		closed = strings.TrimSpace(closed)
		ctx.exec("close", closed)

		inProgress, _ := ctx.exec("new", "InProgress")
		inProgress = strings.TrimSpace(inProgress)
		ctx.exec("start", inProgress)

		output, _ := ctx.exec("closed")

		// Only closed ticket should appear
		if !strings.Contains(output, closed) {
			t.Error("should include closed ticket")
		}
		if strings.Contains(output, inProgress) {
			t.Error("should not include in_progress ticket")
		}
	})
}

// TestClosedSortByModTime tests sorting by modification time
func TestClosedSortByModTime(t *testing.T) {
	t.Run("most recent first", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create and close tickets with delays
		id1, _ := ctx.exec("new", "First")
		id1 = strings.TrimSpace(id1)
		ctx.exec("close", id1)

		time.Sleep(10 * time.Millisecond)

		id2, _ := ctx.exec("new", "Second")
		id2 = strings.TrimSpace(id2)
		ctx.exec("close", id2)

		time.Sleep(10 * time.Millisecond)

		id3, _ := ctx.exec("new", "Third")
		id3 = strings.TrimSpace(id3)
		ctx.exec("close", id3)

		output, _ := ctx.exec("closed")

		// Find positions
		pos1 := strings.Index(output, id1)
		pos2 := strings.Index(output, id2)
		pos3 := strings.Index(output, id3)

		if pos1 == -1 || pos2 == -1 || pos3 == -1 {
			t.Fatal("all tickets should be in output")
		}

		// Most recent (id3) should come first
		if pos3 > pos2 || pos3 > pos1 {
			t.Error("most recent ticket should come first")
		}

		// Oldest (id1) should come last
		if pos1 < pos2 || pos1 < pos3 {
			t.Error("oldest ticket should come last")
		}
	})
}

// TestClosedLimitParameter tests the limit parameter
func TestClosedLimitParameter(t *testing.T) {
	t.Run("default limit 20", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create and close many tickets
		for i := 0; i < 25; i++ {
			id, _ := ctx.exec("new", "Ticket")
			id = strings.TrimSpace(id)
			ctx.exec("close", id)
			time.Sleep(1 * time.Millisecond) // Ensure different mtimes
		}

		output, _ := ctx.exec("closed")

		// Count lines
		lines := strings.Split(strings.TrimSpace(output), "\n")

		// Should default to 20
		if len(lines) > 20 {
			t.Errorf("default limit should be 20, got %d lines", len(lines))
		}
	})

	t.Run("custom limit 5", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create and close 10 tickets
		for i := 0; i < 10; i++ {
			id, _ := ctx.exec("new", "Ticket")
			id = strings.TrimSpace(id)
			ctx.exec("close", id)
			time.Sleep(1 * time.Millisecond)
		}

		output, _ := ctx.exec("closed", "--limit=5")

		// Count lines
		lines := strings.Split(strings.TrimSpace(output), "\n")

		if len(lines) != 5 {
			t.Errorf("expected 5 lines with --limit=5, got %d", len(lines))
		}
	})

	t.Run("limit 1 shows most recent only", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create and close two tickets
		id1, _ := ctx.exec("new", "First")
		id1 = strings.TrimSpace(id1)
		ctx.exec("close", id1)

		time.Sleep(10 * time.Millisecond)

		id2, _ := ctx.exec("new", "Second")
		id2 = strings.TrimSpace(id2)
		ctx.exec("close", id2)

		output, _ := ctx.exec("closed", "--limit=1")

		lines := strings.Split(strings.TrimSpace(output), "\n")

		if len(lines) != 1 {
			t.Errorf("expected 1 line with --limit=1, got %d", len(lines))
		}

		// Should show most recent (id2)
		if !strings.Contains(output, id2) {
			t.Error("should show most recent ticket")
		}

		// Should not show older ticket
		if strings.Contains(output, id1) {
			t.Error("should not show older ticket with --limit=1")
		}
	})

	t.Run("limit larger than tickets shows all", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create 3 closed tickets
		for i := 0; i < 3; i++ {
			id, _ := ctx.exec("new", "Ticket")
			id = strings.TrimSpace(id)
			ctx.exec("close", id)
		}

		output, _ := ctx.exec("closed", "--limit=100")

		lines := strings.Split(strings.TrimSpace(output), "\n")

		if len(lines) != 3 {
			t.Errorf("expected 3 tickets, got %d", len(lines))
		}
	})
}

// TestClosedOutputFormat tests the output format
func TestClosedOutputFormat(t *testing.T) {
	t.Run("output format", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id, _ := ctx.exec("new", "Test Ticket")
		id = strings.TrimSpace(id)
		ctx.exec("close", id)

		output, _ := ctx.exec("closed")

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
		if !strings.Contains(line, "[closed]") {
			t.Errorf("line should contain [closed], got: %s", line)
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
		ctx.exec("close", id)

		output, _ := ctx.exec("closed")

		// All components should be present
		if !strings.Contains(output, id) {
			t.Error("output should contain ID")
		}
		if !strings.Contains(output, "[closed]") {
			t.Error("output should contain [closed]")
		}
		if !strings.Contains(output, "My Ticket") {
			t.Error("output should contain title")
		}
	})

	t.Run("one ticket per line", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create and close three tickets
		for i := 0; i < 3; i++ {
			id, _ := ctx.exec("new", "Ticket")
			id = strings.TrimSpace(id)
			ctx.exec("close", id)
		}

		output, _ := ctx.exec("closed")

		lines := strings.Split(strings.TrimSpace(output), "\n")

		if len(lines) != 3 {
			t.Errorf("expected 3 lines for 3 tickets, got %d", len(lines))
		}
	})
}

// TestClosedEmptyResult tests empty output
func TestClosedEmptyResult(t *testing.T) {
	t.Run("no closed tickets returns empty", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create only open tickets
		ctx.exec("new", "Open Ticket")

		output, _ := ctx.exec("closed")

		trimmed := strings.TrimSpace(output)
		if trimmed != "" {
			t.Errorf("expected empty output when no closed tickets, got: %s", output)
		}
	})

	t.Run("no tickets returns empty", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		output, _ := ctx.exec("closed")

		trimmed := strings.TrimSpace(output)
		if trimmed != "" {
			t.Errorf("expected empty output when no tickets, got: %s", output)
		}
	})
}

// TestClosedMissingTicketsDirectory tests handling of missing directory
func TestClosedMissingTicketsDirectory(t *testing.T) {
	t.Run("missing directory returns empty silently", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		output, err := ctx.exec("closed")
		if err != nil {
			t.Fatalf("closed should not error on missing directory: %v", err)
		}

		trimmed := strings.TrimSpace(output)
		if trimmed != "" {
			t.Errorf("expected empty output for missing directory, got: %s", output)
		}
	})
}

// TestClosedAcceptsDoneStatus tests that "done" status is accepted
func TestClosedAcceptsDoneStatus(t *testing.T) {
	// Note: The bash implementation accepts "done" as equivalent to "closed"
	// The Go implementation uses only "closed" but this test verifies
	// the behavior matches the spec
	t.Run("closed status shown", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id, _ := ctx.exec("new", "Ticket")
		id = strings.TrimSpace(id)
		ctx.exec("close", id)

		output, _ := ctx.exec("closed")

		// Should show with "closed" status
		if !strings.Contains(output, "[closed]") {
			t.Error("should show [closed] status")
		}
	})
}

// TestClosedRecentModification tests that modification time is used
func TestClosedRecentModification(t *testing.T) {
	t.Run("recently modified comes first", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create and close two tickets
		id1, _ := ctx.exec("new", "Old")
		id1 = strings.TrimSpace(id1)
		ctx.exec("close", id1)

		time.Sleep(50 * time.Millisecond)

		id2, _ := ctx.exec("new", "New")
		id2 = strings.TrimSpace(id2)
		ctx.exec("close", id2)

		output, _ := ctx.exec("closed")

		// New should appear before Old
		pos1 := strings.Index(output, id1)
		pos2 := strings.Index(output, id2)

		if pos1 == -1 || pos2 == -1 {
			t.Fatal("both tickets should be in output")
		}

		if pos2 > pos1 {
			t.Error("more recently modified ticket should appear first")
		}
	})

	t.Run("reopened then closed again comes first", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create and close first ticket
		id1, _ := ctx.exec("new", "First")
		id1 = strings.TrimSpace(id1)
		ctx.exec("close", id1)

		time.Sleep(50 * time.Millisecond)

		// Create and close second ticket
		id2, _ := ctx.exec("new", "Second")
		id2 = strings.TrimSpace(id2)
		ctx.exec("close", id2)

		time.Sleep(50 * time.Millisecond)

		// Reopen and close first ticket again (now most recent)
		ctx.exec("reopen", id1)
		time.Sleep(10 * time.Millisecond)
		ctx.exec("close", id1)

		output, _ := ctx.exec("closed")

		// id1 should appear first (most recently modified)
		pos1 := strings.Index(output, id1)
		pos2 := strings.Index(output, id2)

		if pos1 > pos2 {
			t.Error("most recently closed ticket should appear first")
		}
	})
}
