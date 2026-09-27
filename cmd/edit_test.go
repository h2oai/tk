package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEditCommand tests the edit command
func TestEditCommand(t *testing.T) {
	t.Run("non-interactive mode prints file path", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create a test ticket
		id, err := ctx.exec("new", "Edit Test")
		if err != nil {
			t.Fatalf("failed to create ticket: %v", err)
		}
		id = strings.TrimSpace(id)

		// Use Path to get the ticket file path (this is what edit command uses internally)
		expectedPath := filepath.Join(ctx.ticketsDir, id+".md")
		actualPath, err := ctx.store().Path(id)
		if err != nil {
			t.Fatalf("failed to get ticket path: %v", err)
		}

		if actualPath != expectedPath {
			t.Errorf("path = %v, want %v", actualPath, expectedPath)
		}

		// Verify file exists
		if _, err := os.Stat(actualPath); os.IsNotExist(err) {
			t.Errorf("ticket file does not exist at path: %s", actualPath)
		}
	})

	t.Run("partial ID resolution", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create a test ticket
		id, err := ctx.exec("new", "Edit Partial Test")
		if err != nil {
			t.Fatalf("failed to create ticket: %v", err)
		}
		id = strings.TrimSpace(id)

		// Use partial ID (the suffix portion)
		parts := strings.Split(id, "-")
		if len(parts) != 2 {
			t.Fatalf("unexpected ID format: %s", id)
		}
		partial := parts[1] // Just the suffix part

		// Path resolution with partial ID should work
		actualPath, err := ctx.store().Path(partial)
		if err != nil {
			t.Fatalf("path resolution with partial ID error: %v", err)
		}

		expectedPath := filepath.Join(ctx.ticketsDir, id+".md")
		if actualPath != expectedPath {
			t.Errorf("expected path %s, got: %s", expectedPath, actualPath)
		}
	})

	t.Run("missing ticket returns error", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Try to get path for non-existent ticket
		_, err := ctx.store().Path("nonexistent")
		if err == nil {
			t.Error("expected error for non-existent ticket, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got: %v", err)
		}
	})

	t.Run("ticket file path exists", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create a test ticket
		id, err := ctx.exec("new", "Edit Exists Test")
		if err != nil {
			t.Fatalf("failed to create ticket: %v", err)
		}
		id = strings.TrimSpace(id)

		// Get path
		actualPath, err := ctx.store().Path(id)
		if err != nil {
			t.Fatalf("failed to get ticket path: %v", err)
		}

		// Verify file actually exists
		if _, err := os.Stat(actualPath); os.IsNotExist(err) {
			t.Errorf("ticket file does not exist at path: %s", actualPath)
		}
	})
}
