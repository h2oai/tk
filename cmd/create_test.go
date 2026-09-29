package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/lo5/tk/internal/ticket"
	"github.com/spf13/cobra"
)

// testContext holds test context including the tickets directory
type testContext struct {
	ticketsDir string
	t          *testing.T
}

// setupTestCmd creates a test environment with a temp store
func setupTestCmd(t *testing.T) (*testContext, func()) {
	t.Helper()
	tempDir := t.TempDir()
	ticketsDir := filepath.Join(tempDir, ".tickets")

	cleanup := func() {
		// Reset flags to defaults
		newID = ""
		newDescription = ""
		newDesign = ""
		newAcceptance = ""
		newPriority = 2
		newType = "task"
		newAssignee = ""
		newExternalRef = ""
		newParent = ""
		listStatus = ""
		listSort = "date"
		blockedSort = "date"
		readySort = "priority"
		closedLimit = 20
		rmForce = false
		pruneFix = false
		cleanFix = false
	}

	ctx := &testContext{
		ticketsDir: ticketsDir,
		t:          t,
	}

	return ctx, cleanup
}

// exec executes a command with the test context's tickets directory
func (ctx *testContext) exec(args ...string) (string, error) {
	// Prepend --dir flag
	fullArgs := append([]string{"--dir", ctx.ticketsDir}, args...)
	return executeCommand(rootCmd, fullArgs...)
}

// store returns a FileStore for the test context
func (ctx *testContext) store() *ticket.FileStore {
	return ticket.NewFileStore(ctx.ticketsDir)
}

// executeCommand executes a command and returns the output
func executeCommand(cmd *cobra.Command, args ...string) (string, error) {
	// Capture stdout since commands use fmt.Println
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Channel to collect the captured output
	outC := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	// Also set command output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	// Execute command
	err := cmd.Execute()

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Get captured output
	captured := <-outC

	// Combine both outputs
	output := captured + buf.String()
	return output, err
}

// TestNewCommand tests the new command
func TestNewCommand(t *testing.T) {
	t.Run("basic new with title", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		output, err := ctx.exec("new", "My Test Ticket")
		if err != nil {
			t.Fatalf("new command error: %v", err)
		}

		// Should output the generated ID
		id := strings.TrimSpace(output)
		if id == "" {
			t.Error("create command did not output ticket ID")
		}

		// Verify ticket file was created
		files, _ := os.ReadDir(ctx.ticketsDir)
		if len(files) != 1 {
			t.Errorf("expected 1 ticket file, got %d", len(files))
		}

		// Verify ticket content
		t2, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve created ticket: %v", err)
		}

		if t2.Title != "My Test Ticket" {
			t.Errorf("Title = %v, want %v", t2.Title, "My Test Ticket")
		}
		if t2.Status != ticket.StatusOpen {
			t.Errorf("Status = %v, want %v", t2.Status, ticket.StatusOpen)
		}
		if t2.Type != ticket.TypeTask {
			t.Errorf("Type = %v, want %v", t2.Type, ticket.TypeTask)
		}
		if t2.Priority != 2 {
			t.Errorf("Priority = %v, want %v", t2.Priority, 2)
		}
	})

	t.Run("create with all flags", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		output, err := ctx.exec("new", "Full Ticket",
			"--description", "This is a description",
			"--design", "Design notes here",
			"--acceptance", "Accept criteria",
			"--type", "bug",
			"--priority", "1",
			"--assignee", "alice",
			"--external-ref", "BUG-123",
		)
		if err != nil {
			t.Fatalf("new command error: %v", err)
		}

		id := strings.TrimSpace(output)
		t2, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve created ticket: %v", err)
		}

		if t2.Title != "Full Ticket" {
			t.Errorf("Title = %v, want %v", t2.Title, "Full Ticket")
		}
		if t2.Type != ticket.TypeBug {
			t.Errorf("Type = %v, want %v", t2.Type, ticket.TypeBug)
		}
		if t2.Priority != 1 {
			t.Errorf("Priority = %v, want %v", t2.Priority, 1)
		}
		if t2.Assignee != "alice" {
			t.Errorf("Assignee = %v, want %v", t2.Assignee, "alice")
		}
		if t2.ExternalRef != "BUG-123" {
			t.Errorf("ExternalRef = %v, want %v", t2.ExternalRef, "BUG-123")
		}
		if !strings.Contains(t2.Body, "This is a description") {
			t.Error("Body missing description")
		}
		if !strings.Contains(t2.Body, "Design notes here") {
			t.Error("Body missing design")
		}
		if !strings.Contains(t2.Body, "Accept criteria") {
			t.Error("Body missing acceptance criteria")
		}
	})

	t.Run("default values", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		output, err := ctx.exec("new", "Default Ticket")
		if err != nil {
			t.Fatalf("new command error: %v", err)
		}

		id := strings.TrimSpace(output)
		t2, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve created ticket: %v", err)
		}

		// Default status: open
		if t2.Status != ticket.StatusOpen {
			t.Errorf("Default Status = %v, want %v", t2.Status, ticket.StatusOpen)
		}

		// Default type: task
		if t2.Type != ticket.TypeTask {
			t.Errorf("Default Type = %v, want %v", t2.Type, ticket.TypeTask)
		}

		// Default priority: 2
		if t2.Priority != 2 {
			t.Errorf("Default Priority = %v, want %v", t2.Priority, 2)
		}
	})

	t.Run("priority validation", func(t *testing.T) {
		tests := []struct {
			priority string
			wantErr  bool
		}{
			{"0", false},
			{"1", false},
			{"2", false},
			{"3", false},
			{"4", false},
			{"-1", true},
			{"5", true},
			{"10", true},
		}

		for _, tt := range tests {
			t.Run("priority_"+tt.priority, func(t *testing.T) {
				ctx, cleanup := setupTestCmd(t)
				defer cleanup()

				_, err := ctx.exec("new", "Test", "--priority", tt.priority)
				if (err != nil) != tt.wantErr {
					t.Errorf("priority %s: error = %v, wantErr %v", tt.priority, err, tt.wantErr)
				}
			})
		}
	})

	t.Run("type validation", func(t *testing.T) {
		validTypes := []string{"bug", "feature", "task", "epic", "chore"}
		for _, typ := range validTypes {
			t.Run("valid_"+typ, func(t *testing.T) {
				ctx, cleanup := setupTestCmd(t)
				defer cleanup()

				output, err := ctx.exec("new", "Test", "--type", typ)
				if err != nil {
					t.Errorf("valid type %s failed: %v", typ, err)
				}

				id := strings.TrimSpace(output)
				t2, _ := ctx.store().Get(id)
				if string(t2.Type) != typ {
					t.Errorf("Type = %v, want %v", t2.Type, typ)
				}
			})
		}

		// Test invalid type
		t.Run("invalid_type", func(t *testing.T) {
			ctx, cleanup := setupTestCmd(t)
			defer cleanup()

			_, err := ctx.exec("new", "Test", "--type", "invalid")
			if err == nil {
				t.Error("expected error for invalid type, got nil")
			}
			if !strings.Contains(err.Error(), "invalid type") {
				t.Errorf("error message should mention invalid type, got: %v", err)
			}
		})
	})

	t.Run("empty title uses Untitled", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create with no args (should use "Untitled")
		output, err := ctx.exec("new")
		if err != nil {
			t.Fatalf("new command error: %v", err)
		}

		id := strings.TrimSpace(output)
		t2, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve created ticket: %v", err)
		}

		if t2.Title != "Untitled" {
			t.Errorf("Title = %v, want %v", t2.Title, "Untitled")
		}
	})

	t.Run("special characters in title", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		specialTitle := "Test: Special & \"Chars\" <>"
		output, err := ctx.exec("new", specialTitle)
		if err != nil {
			t.Fatalf("new command error: %v", err)
		}

		id := strings.TrimSpace(output)
		t2, err := ctx.store().Get(id)
		if err != nil {
			t.Fatalf("failed to retrieve created ticket: %v", err)
		}

		if t2.Title != specialTitle {
			t.Errorf("Title = %v, want %v", t2.Title, specialTitle)
		}
	})

	t.Run("output format includes ID", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		output, err := ctx.exec("new", "Test")
		if err != nil {
			t.Fatalf("new command error: %v", err)
		}

		id := strings.TrimSpace(output)

		// ID should be 5 lowercase letters plus a digit, with no prefix
		if !regexp.MustCompile(`^[a-z]{5}[0-9]$`).MatchString(id) {
			t.Errorf("ID should be 5 lowercase letters plus a digit with no prefix, got: %s", id)
		}
	})

	t.Run("ticket file created in correct location", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		output, err := ctx.exec("new", "Test")
		if err != nil {
			t.Fatalf("new command error: %v", err)
		}

		id := strings.TrimSpace(output)
		expectedPath := filepath.Join(ctx.ticketsDir, id+".md")

		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			t.Errorf("ticket file not created at expected path: %s", expectedPath)
		}
	})
}

// TestNewCommand_WithID tests the --id flag
func TestNewCommand_WithID(t *testing.T) {
	t.Run("explicit id is used", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		output, err := ctx.exec("new", "Custom ID Ticket", "--id", "custom-abc123")
		if err != nil {
			t.Fatalf("new command error: %v", err)
		}

		id := strings.TrimSpace(output)
		if id != "custom-abc123" {
			t.Errorf("id = %q, want %q", id, "custom-abc123")
		}

		t2, err := ctx.store().Get("custom-abc123")
		if err != nil {
			t.Fatalf("failed to retrieve ticket: %v", err)
		}
		if t2.ID != "custom-abc123" {
			t.Errorf("ticket ID = %q, want %q", t2.ID, "custom-abc123")
		}
	})

	t.Run("duplicate id is rejected", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		if _, err := ctx.exec("new", "First", "--id", "dupe-id"); err != nil {
			t.Fatalf("first new error: %v", err)
		}

		_, err := ctx.exec("new", "Second", "--id", "dupe-id")
		if err == nil {
			t.Fatal("expected collision error, got nil")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("error should mention existing ID, got: %v", err)
		}
	})

	t.Run("invalid ids are rejected", func(t *testing.T) {
		cases := map[string]string{
			"uppercase":     "Custom-ID",
			"space":         "custom id",
			"path traverse": "../evil",
			"slash":         "a/b",
			"empty":         " ",
		}
		for name, value := range cases {
			t.Run(name, func(t *testing.T) {
				ctx, cleanup := setupTestCmd(t)
				defer cleanup()

				_, err := ctx.exec("new", "Test", "--id", value)
				if err == nil {
					t.Fatalf("expected error for id %q, got nil", value)
				}
			})
		}
	})

	t.Run("no id flag still auto-generates", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		output, err := ctx.exec("new", "Auto Ticket")
		if err != nil {
			t.Fatalf("new command error: %v", err)
		}
		if strings.TrimSpace(output) == "" {
			t.Error("expected auto-generated id")
		}
	})
}

// TestNewCommand_WithParent tests creating tickets with parent relationships
func TestNewCommand_WithParent(t *testing.T) {
	t.Run("new with parent", func(t *testing.T) {
		ctx, cleanup := setupTestCmd(t)
		defer cleanup()

		// Create parent ticket first
		parentOutput, err := ctx.exec("new", "Parent Ticket")
		if err != nil {
			t.Fatalf("new parent error: %v", err)
		}
		parentID := strings.TrimSpace(parentOutput)

		// Create child with parent
		childOutput, err := ctx.exec("new", "Child Ticket", "--parent", parentID)
		if err != nil {
			t.Fatalf("new child error: %v", err)
		}
		childID := strings.TrimSpace(childOutput)

		// Verify child has parent set
		child, err := ctx.store().Get(childID)
		if err != nil {
			t.Fatalf("failed to retrieve child ticket: %v", err)
		}

		if child.Parent != parentID {
			t.Errorf("Parent = %v, want %v", child.Parent, parentID)
		}
	})
}
