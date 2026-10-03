package cmd

import (
	"strings"
	"testing"

	"github.com/h2oai/tk/internal/ticket"
)

// createTestTicket creates a ticket for testing status commands
func createTestTicket(ctx *testContext, t *testing.T, title string, status ticket.Status) string {
	t.Helper()

	// Create ticket with open status first
	output, err := ctx.exec("new", title)
	if err != nil {
		t.Fatalf("failed to create test ticket: %v", err)
	}
	id := strings.TrimSpace(output)

	// If we need a different status, update it
	if status != ticket.StatusOpen {
		if _, err := ctx.store().UpdateField(id, "status", string(status)); err != nil {
			t.Fatalf("failed to set initial status: %v", err)
		}
	}

	return id
}

// TestStatusCommand tests the status command
func TestStatusCommand(t *testing.T) {
	t.Run("update status to in_progress", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Test Ticket", ticket.StatusOpen)

		output, err := ctx.exec("status", id, "in_progress")
		if err != nil {
			t.Fatalf("status command error: %v", err)
		}

		if !strings.Contains(output, id) {
			t.Errorf("output should contain ticket ID, got: %s", output)
		}
		if !strings.Contains(output, "in_progress") {
			t.Errorf("output should contain new status, got: %s", output)
		}

		// Verify status was updated
		tk, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}
		if tk.Status != ticket.StatusInProgress {
			t.Errorf("Status = %v, want %v", tk.Status, ticket.StatusInProgress)
		}
	})

	t.Run("update status to closed", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Test Ticket", ticket.StatusOpen)

		_, err := ctx.exec("status", id, "closed")
		if err != nil {
			t.Fatalf("status command error: %v", err)
		}

		tk, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}
		if tk.Status != ticket.StatusClosed {
			t.Errorf("Status = %v, want %v", tk.Status, ticket.StatusClosed)
		}
	})

	t.Run("update status to open", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Test Ticket", ticket.StatusClosed)

		_, err := ctx.exec("status", id, "open")
		if err != nil {
			t.Fatalf("status command error: %v", err)
		}

		tk, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}
		if tk.Status != ticket.StatusOpen {
			t.Errorf("Status = %v, want %v", tk.Status, ticket.StatusOpen)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Test Ticket", ticket.StatusOpen)

		_, err := ctx.exec("status", id, "invalid_status")
		if err == nil {
			t.Error("expected error for invalid status, got nil")
		}
		if !strings.Contains(err.Error(), "invalid status") {
			t.Errorf("error should mention invalid status, got: %v", err)
		}
	})

	t.Run("status with partial ID", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Test Ticket", ticket.StatusOpen)

		// Use the verb-noun portion as partial ID
		if len(id) < 5 {
			t.Skip("ID too short for partial matching")
		}
		partial := id[strings.Index(id, "-")+1:]

		_, err := ctx.exec("status", partial, "in_progress")
		if err != nil {
			t.Fatalf("status command with partial ID error: %v", err)
		}

		tk, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}
		if tk.Status != ticket.StatusInProgress {
			t.Errorf("Status = %v, want %v", tk.Status, ticket.StatusInProgress)
		}
	})

	t.Run("non-existent ticket", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		_, err := ctx.exec("status", "nonexistent-id", "open")
		if err == nil {
			t.Error("expected error for non-existent ticket, got nil")
		}
	})
}

// TestStartCommand tests the start command
func TestStartCommand(t *testing.T) {
	t.Run("start ticket", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Test Ticket", ticket.StatusOpen)

		output, err := ctx.exec("start", id)
		if err != nil {
			t.Fatalf("start command error: %v", err)
		}

		if !strings.Contains(output, id) {
			t.Errorf("output should contain ticket ID, got: %s", output)
		}
		if !strings.Contains(output, "in_progress") {
			t.Errorf("output should contain status, got: %s", output)
		}

		// Verify status
		tk, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}
		if tk.Status != ticket.StatusInProgress {
			t.Errorf("Status = %v, want %v", tk.Status, ticket.StatusInProgress)
		}
	})

	t.Run("start already in_progress ticket", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Test Ticket", ticket.StatusInProgress)

		// Starting again should succeed (idempotent)
		_, err := ctx.exec("start", id)
		if err != nil {
			t.Fatalf("start command error: %v", err)
		}

		tk, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}
		if tk.Status != ticket.StatusInProgress {
			t.Errorf("Status = %v, want %v", tk.Status, ticket.StatusInProgress)
		}
	})

	t.Run("start closed ticket", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Test Ticket", ticket.StatusClosed)

		// Should be able to start a closed ticket (moves it back to in_progress)
		_, err := ctx.exec("start", id)
		if err != nil {
			t.Fatalf("start command error: %v", err)
		}

		tk, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}
		if tk.Status != ticket.StatusInProgress {
			t.Errorf("Status = %v, want %v", tk.Status, ticket.StatusInProgress)
		}
	})

	t.Run("start with partial ID", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Test Ticket", ticket.StatusOpen)

		if len(id) < 5 {
			t.Skip("ID too short for partial matching")
		}
		partial := id[strings.Index(id, "-")+1:]

		_, err := ctx.exec("start", partial)
		if err != nil {
			t.Fatalf("start command error: %v", err)
		}

		tk, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}
		if tk.Status != ticket.StatusInProgress {
			t.Errorf("Status = %v, want %v", tk.Status, ticket.StatusInProgress)
		}
	})

	t.Run("start non-existent ticket", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		_, err := ctx.exec("start", "nonexistent-id")
		if err == nil {
			t.Error("expected error for non-existent ticket, got nil")
		}
	})
}

// TestCloseCommand tests the close command
func TestCloseCommand(t *testing.T) {
	t.Run("close open ticket", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Test Ticket", ticket.StatusOpen)

		output, err := ctx.exec("close", id)
		if err != nil {
			t.Fatalf("close command error: %v", err)
		}

		if !strings.Contains(output, id) {
			t.Errorf("output should contain ticket ID, got: %s", output)
		}
		if !strings.Contains(output, "closed") {
			t.Errorf("output should contain status, got: %s", output)
		}

		tk, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}
		if tk.Status != ticket.StatusClosed {
			t.Errorf("Status = %v, want %v", tk.Status, ticket.StatusClosed)
		}
	})

	t.Run("close in_progress ticket", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Test Ticket", ticket.StatusInProgress)

		_, err := ctx.exec("close", id)
		if err != nil {
			t.Fatalf("close command error: %v", err)
		}

		tk, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}
		if tk.Status != ticket.StatusClosed {
			t.Errorf("Status = %v, want %v", tk.Status, ticket.StatusClosed)
		}
	})

	t.Run("close already closed ticket", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Test Ticket", ticket.StatusClosed)

		// Closing again should succeed (idempotent)
		_, err := ctx.exec("close", id)
		if err != nil {
			t.Fatalf("close command error: %v", err)
		}

		tk, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}
		if tk.Status != ticket.StatusClosed {
			t.Errorf("Status = %v, want %v", tk.Status, ticket.StatusClosed)
		}
	})

	t.Run("close with partial ID", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Test Ticket", ticket.StatusOpen)

		if len(id) < 5 {
			t.Skip("ID too short for partial matching")
		}
		partial := id[strings.Index(id, "-")+1:]

		_, err := ctx.exec("close", partial)
		if err != nil {
			t.Fatalf("close command error: %v", err)
		}

		tk, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}
		if tk.Status != ticket.StatusClosed {
			t.Errorf("Status = %v, want %v", tk.Status, ticket.StatusClosed)
		}
	})

	t.Run("close non-existent ticket", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		_, err := ctx.exec("close", "nonexistent-id")
		if err == nil {
			t.Error("expected error for non-existent ticket, got nil")
		}
	})
}

// TestReopenCommand tests the reopen command
func TestReopenCommand(t *testing.T) {
	t.Run("reopen closed ticket", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Test Ticket", ticket.StatusClosed)

		output, err := ctx.exec("reopen", id)
		if err != nil {
			t.Fatalf("reopen command error: %v", err)
		}

		if !strings.Contains(output, id) {
			t.Errorf("output should contain ticket ID, got: %s", output)
		}
		if !strings.Contains(output, "open") {
			t.Errorf("output should contain status, got: %s", output)
		}

		tk, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}
		if tk.Status != ticket.StatusOpen {
			t.Errorf("Status = %v, want %v", tk.Status, ticket.StatusOpen)
		}
	})

	t.Run("reopen already open ticket", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Test Ticket", ticket.StatusOpen)

		// Reopening an open ticket should succeed (idempotent)
		_, err := ctx.exec("reopen", id)
		if err != nil {
			t.Fatalf("reopen command error: %v", err)
		}

		tk, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}
		if tk.Status != ticket.StatusOpen {
			t.Errorf("Status = %v, want %v", tk.Status, ticket.StatusOpen)
		}
	})

	t.Run("reopen in_progress ticket", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Test Ticket", ticket.StatusInProgress)

		// Reopening in_progress ticket should set to open
		_, err := ctx.exec("reopen", id)
		if err != nil {
			t.Fatalf("reopen command error: %v", err)
		}

		tk, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}
		if tk.Status != ticket.StatusOpen {
			t.Errorf("Status = %v, want %v", tk.Status, ticket.StatusOpen)
		}
	})

	t.Run("reopen with partial ID", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Test Ticket", ticket.StatusClosed)

		if len(id) < 5 {
			t.Skip("ID too short for partial matching")
		}
		partial := id[strings.Index(id, "-")+1:]

		_, err := ctx.exec("reopen", partial)
		if err != nil {
			t.Fatalf("reopen command error: %v", err)
		}

		tk, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}
		if tk.Status != ticket.StatusOpen {
			t.Errorf("Status = %v, want %v", tk.Status, ticket.StatusOpen)
		}
	})

	t.Run("reopen non-existent ticket", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		_, err := ctx.exec("reopen", "nonexistent-id")
		if err == nil {
			t.Error("expected error for non-existent ticket, got nil")
		}
	})
}

// TestStatusTransitions tests various status transitions
func TestStatusTransitions(t *testing.T) {
	t.Run("full lifecycle: open -> in_progress -> closed -> open", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Lifecycle Ticket", ticket.StatusOpen)

		// Verify initial state
		tk, _ := ctx.store().Get(id)
		if tk.Status != ticket.StatusOpen {
			t.Fatalf("Initial status = %v, want %v", tk.Status, ticket.StatusOpen)
		}

		// Open -> In Progress
		_, err := ctx.exec("start", id)
		if err != nil {
			t.Fatalf("start error: %v", err)
		}
		tk, _ = ctx.store().Get(id)
		if tk.Status != ticket.StatusInProgress {
			t.Errorf("After start: Status = %v, want %v", tk.Status, ticket.StatusInProgress)
		}

		// In Progress -> Closed
		_, err = ctx.exec("close", id)
		if err != nil {
			t.Fatalf("close error: %v", err)
		}
		tk, _ = ctx.store().Get(id)
		if tk.Status != ticket.StatusClosed {
			t.Errorf("After close: Status = %v, want %v", tk.Status, ticket.StatusClosed)
		}

		// Closed -> Open (reopen)
		_, err = ctx.exec("reopen", id)
		if err != nil {
			t.Fatalf("reopen error: %v", err)
		}
		tk, _ = ctx.store().Get(id)
		if tk.Status != ticket.StatusOpen {
			t.Errorf("After reopen: Status = %v, want %v", tk.Status, ticket.StatusOpen)
		}
	})

	t.Run("persistence: status change written to disk", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		id := createTestTicket(ctx, t, "Persist Ticket", ticket.StatusOpen)

		_, err := ctx.exec("start", id)
		if err != nil {
			t.Fatalf("start error: %v", err)
		}

		// Read again from disk
		tk, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to read from disk: %v", err)
		}
		if tk.Status != ticket.StatusInProgress {
			t.Errorf("Status after reading from disk = %v, want %v", tk.Status, ticket.StatusInProgress)
		}

		// Verify file is valid ticket format
		_, content, err := ctx.store().ReadRaw(id)
		if err != nil {
			t.Fatalf("failed to read raw content: %v", err)
		}

		if !strings.Contains(content, "status: in_progress") {
			t.Error("File content missing updated status")
		}
	})
}
