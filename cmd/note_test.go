package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

// TestAddNoteCommand tests the add-note command
func TestAddNoteCommand(t *testing.T) {
	t.Run("append note from args", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create a test ticket
		id, err := ctx.exec("new", "Note Test")
		if err != nil {
			t.Fatalf("failed to create ticket: %v", err)
		}
		id = strings.TrimSpace(id)

		// Add note with args
		output, err := ctx.exec("note", id, "This is my note")
		if err != nil {
			t.Fatalf("add-note command error: %v", err)
		}

		if !strings.Contains(output, "Note added to") {
			t.Errorf("expected success message, got: %s", output)
		}

		// Verify note was added
		ticket, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}

		if !strings.Contains(ticket.Body, "This is my note") {
			t.Error("note not found in ticket body")
		}
	})

	t.Run("append note with multiple args", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create a test ticket
		id, err := ctx.exec("new", "Multi Note Test")
		if err != nil {
			t.Fatalf("failed to create ticket: %v", err)
		}
		id = strings.TrimSpace(id)

		// Add note with multiple words
		output, err := ctx.exec("note", id, "Multiple", "word", "note")
		if err != nil {
			t.Fatalf("add-note command error: %v", err)
		}

		if !strings.Contains(output, "Note added to") {
			t.Errorf("expected success message, got: %s", output)
		}

		// Verify note was added (args should be joined with spaces)
		ticket, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}

		if !strings.Contains(ticket.Body, "Multiple word note") {
			t.Error("note with joined args not found in ticket body")
		}
	})

	t.Run("timestamp format", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create a test ticket
		id, err := ctx.exec("new", "Timestamp Test")
		if err != nil {
			t.Fatalf("failed to create ticket: %v", err)
		}
		id = strings.TrimSpace(id)

		// Add note
		before := time.Now().UTC().Add(-2 * time.Second)
		_, err = ctx.exec("note", id, "Timestamped note")
		if err != nil {
			t.Fatalf("add-note command error: %v", err)
		}
		after := time.Now().UTC().Add(2 * time.Second)

		// Get raw content
		_, _, content, err := ctx.store().GetRawContent(id)
		if err != nil {
			t.Fatalf("failed to get raw content: %v", err)
		}

		// Check for timestamp in RFC3339 format (e.g., **2025-01-11T10:30:00Z**)
		// The timestamp should be between before and after
		if !strings.Contains(content, "**") {
			t.Error("timestamp not found in bold format")
		}

		// Parse timestamp from content
		// Format is: **YYYY-MM-DDTHH:MM:SSZ**
		lines := strings.Split(content, "\n")
		var timestampLine string
		for _, line := range lines {
			if strings.HasPrefix(line, "**") && strings.HasSuffix(strings.TrimSpace(line), "**") {
				timestampLine = line
				break
			}
		}

		if timestampLine == "" {
			t.Fatal("timestamp line not found")
		}

		// Extract timestamp (remove **)
		tsStr := strings.Trim(timestampLine, "* ")
		ts, err := time.Parse(time.RFC3339, tsStr)
		if err != nil {
			t.Errorf("failed to parse timestamp %q: %v", tsStr, err)
		}

		// Verify timestamp is in expected range
		if ts.Before(before) || ts.After(after) {
			t.Errorf("timestamp %v not in expected range [%v, %v]", ts, before, after)
		}
	})

	t.Run("notes section created if missing", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create a test ticket
		id, err := ctx.exec("new", "Notes Section Test")
		if err != nil {
			t.Fatalf("failed to create ticket: %v", err)
		}
		id = strings.TrimSpace(id)

		// Add note
		_, err = ctx.exec("note", id, "First note")
		if err != nil {
			t.Fatalf("add-note command error: %v", err)
		}

		// Verify Notes section was created
		_, _, content, err := ctx.store().GetRawContent(id)
		if err != nil {
			t.Fatalf("failed to get raw content: %v", err)
		}

		if !strings.Contains(content, "## Notes") {
			t.Error("Notes section not created")
		}
	})

	t.Run("notes section appended if exists", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create a test ticket
		id, err := ctx.exec("new", "Append Notes Test")
		if err != nil {
			t.Fatalf("failed to create ticket: %v", err)
		}
		id = strings.TrimSpace(id)

		// Add first note
		_, err = ctx.exec("note", id, "First note")
		if err != nil {
			t.Fatalf("add-note command error: %v", err)
		}

		// Add second note
		_, err = ctx.exec("note", id, "Second note")
		if err != nil {
			t.Fatalf("add-note command error: %v", err)
		}

		// Verify both notes present
		_, _, content, err := ctx.store().GetRawContent(id)
		if err != nil {
			t.Fatalf("failed to get raw content: %v", err)
		}

		if !strings.Contains(content, "First note") {
			t.Error("First note not found")
		}
		if !strings.Contains(content, "Second note") {
			t.Error("Second note not found")
		}

		// Should only have one Notes section header
		count := strings.Count(content, "## Notes")
		if count != 1 {
			t.Errorf("expected 1 Notes section, found %d", count)
		}
	})

	t.Run("note format has blank lines", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create a test ticket
		id, err := ctx.exec("new", "Format Test")
		if err != nil {
			t.Fatalf("failed to create ticket: %v", err)
		}
		id = strings.TrimSpace(id)

		// Add note
		_, err = ctx.exec("note", id, "Format note")
		if err != nil {
			t.Fatalf("add-note command error: %v", err)
		}

		// Get raw content
		_, _, content, err := ctx.store().GetRawContent(id)
		if err != nil {
			t.Fatalf("failed to get raw content: %v", err)
		}

		// Check format: blank line before timestamp, blank line after timestamp
		// Format should be: \n**timestamp**\n\nnote\n
		if !strings.Contains(content, "\n**") {
			t.Error("missing newline before timestamp")
		}

		// Find timestamp line and check there's a blank line after it
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "**") && strings.HasSuffix(strings.TrimSpace(line), "**") {
				// Check next line is blank
				if i+1 < len(lines) && lines[i+1] != "" {
					t.Error("expected blank line after timestamp")
				}
				// Check note content follows
				if i+2 < len(lines) && !strings.Contains(lines[i+2], "Format note") {
					t.Error("note content not found after timestamp")
				}
				break
			}
		}
	})

	t.Run("partial ID resolution", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create a test ticket
		id, err := ctx.exec("new", "Partial ID Note Test")
		if err != nil {
			t.Fatalf("failed to create ticket: %v", err)
		}
		id = strings.TrimSpace(id)

		// Use partial ID (the first few characters)
		partial := id[:3]

		// Add note with partial ID
		output, err := ctx.exec("note", partial, "Partial ID note")
		if err != nil {
			t.Fatalf("add-note command with partial ID error: %v", err)
		}

		if !strings.Contains(output, "Note added to") {
			t.Errorf("expected success message, got: %s", output)
		}

		// Verify note was added
		ticket, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}

		if !strings.Contains(ticket.Body, "Partial ID note") {
			t.Error("note not found in ticket body")
		}
	})

	t.Run("preserve existing body", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create a test ticket with description
		id, err := ctx.exec("new", "Preserve Body Test", "--description", "Original description")
		if err != nil {
			t.Fatalf("failed to create ticket: %v", err)
		}
		id = strings.TrimSpace(id)

		// Add note
		_, err = ctx.exec("note", id, "New note")
		if err != nil {
			t.Fatalf("add-note command error: %v", err)
		}

		// Verify original description is preserved
		ticket, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}

		if !strings.Contains(ticket.Body, "Original description") {
			t.Error("original description not preserved")
		}
		if !strings.Contains(ticket.Body, "New note") {
			t.Error("new note not found")
		}
	})

	t.Run("output message format", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create a test ticket
		id, err := ctx.exec("new", "Output Test")
		if err != nil {
			t.Fatalf("failed to create ticket: %v", err)
		}
		id = strings.TrimSpace(id)

		// Add note
		output, err := ctx.exec("note", id, "Test note")
		if err != nil {
			t.Fatalf("add-note command error: %v", err)
		}

		// Check output format: "Note added to {id}"
		expected := "Note added to " + id
		if !strings.Contains(output, expected) {
			t.Errorf("expected output to contain %q, got: %s", expected, output)
		}
	})

	t.Run("no note provided error", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create a test ticket
		id, err := ctx.exec("new", "No Note Test")
		if err != nil {
			t.Fatalf("failed to create ticket: %v", err)
		}
		id = strings.TrimSpace(id)

		// Try to add note without providing text (in TTY mode, which is the test default)
		// This should fail because we don't provide args and stdin is a TTY
		_, err = ctx.exec("note", id)
		if err == nil {
			t.Error("expected error when no note provided, got nil")
		}
		if !strings.Contains(err.Error(), "no note provided") {
			t.Errorf("expected 'no note provided' error, got: %v", err)
		}
	})

	t.Run("missing ticket returns error", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Try to add note to non-existent ticket
		_, err := ctx.exec("note", "nonexistent", "Note text")
		if err == nil {
			t.Error("expected error for non-existent ticket, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected 'not found' error, got: %v", err)
		}
	})

	t.Run("alias 'note' works", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create a test ticket
		id, err := ctx.exec("new", "Alias Test")
		if err != nil {
			t.Fatalf("failed to create ticket: %v", err)
		}
		id = strings.TrimSpace(id)

		// Use 'note' alias instead of 'add-note'
		output, err := ctx.exec("note", id, "Alias note")
		if err != nil {
			t.Fatalf("note alias command error: %v", err)
		}

		if !strings.Contains(output, "Note added to") {
			t.Errorf("expected success message, got: %s", output)
		}

		// Verify note was added
		ticket, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}

		if !strings.Contains(ticket.Body, "Alias note") {
			t.Error("note not found in ticket body")
		}
	})
}

// TestAddNoteFromStdin tests adding notes via stdin
func TestAddNoteFromStdin(t *testing.T) {
	t.Run("append note from stdin", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create a test ticket
		idOutput, err := ctx.exec("new", "Stdin Note Test")
		if err != nil {
			t.Fatalf("failed to create ticket: %v", err)
		}
		id := strings.TrimSpace(idOutput)

		// Simulate stdin by temporarily replacing os.Stdin
		oldStdin := os.Stdin
		defer func() { os.Stdin = oldStdin }()

		// Create a pipe with note content
		r, w, _ := os.Pipe()
		os.Stdin = r
		go func() {
			w.Write([]byte("Note from stdin"))
			w.Close()
		}()

		// Execute add-note with no args (should read from stdin)
		// Reset command args
		rootCmd.SetArgs([]string{"--dir", ctx.ticketsDir, "note", id})

		// Capture output
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)

		err = rootCmd.Execute()
		if err != nil {
			t.Fatalf("add-note from stdin error: %v", err)
		}

		// Verify note was added
		ticket, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}

		if !strings.Contains(ticket.Body, "Note from stdin") {
			t.Error("note from stdin not found in ticket body")
		}
	})
}
