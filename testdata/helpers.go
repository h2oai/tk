package testdata

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/h2oai/tk/internal/ticket"
)

// CreateTestStore creates a temporary store for testing
// Returns the store and the temp directory path
func CreateTestStore(t *testing.T) (*ticket.FileStore, string) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "gotk-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	ticketsDir := filepath.Join(tempDir, ".tickets")
	if err := os.MkdirAll(ticketsDir, 0755); err != nil {
		t.Fatalf("failed to create tickets dir: %v", err)
	}

	store := ticket.NewFileStore(ticketsDir)
	return store, tempDir
}

// CleanupTestStore removes the temporary directory
func CleanupTestStore(t *testing.T, tempDir string) {
	t.Helper()
	if err := os.RemoveAll(tempDir); err != nil {
		t.Errorf("failed to cleanup temp dir: %v", err)
	}
}

// CreateTestTicket creates a test ticket with the given title and returns its ID
func CreateTestTicket(t *testing.T, store *ticket.FileStore, title string) string {
	t.Helper()

	tk := &ticket.Ticket{
		ID:       ticket.GenerateID(),
		Status:   ticket.StatusOpen,
		Type:     ticket.TypeTask,
		Priority: 2,
		Created:  time.Now(),
		Assignee: "testuser",
		Title:    title,
		Body:     "Test ticket body for " + title,
	}

	if err := store.Create(tk); err != nil {
		t.Fatalf("failed to create test ticket: %v", err)
	}

	return tk.ID
}

// CreateTestTickets creates N test tickets and returns their IDs
func CreateTestTickets(t *testing.T, store *ticket.FileStore, count int) []string {
	t.Helper()

	ids := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = CreateTestTicket(t, store, "Test ticket "+string(rune('A'+i)))
	}
	return ids
}

// CreateTestTicketWithOptions creates a test ticket with specific options
func CreateTestTicketWithOptions(t *testing.T, store *ticket.FileStore, opts TicketOptions) string {
	t.Helper()

	tk := &ticket.Ticket{
		ID:       ticket.GenerateID(),
		Status:   opts.Status,
		Type:     opts.Type,
		Priority: opts.Priority,
		Created:  time.Now(),
		Assignee: opts.Assignee,
		Title:    opts.Title,
		Body:     opts.Body,
		Deps:     opts.Deps,
		Links:    opts.Links,
		Parent:   opts.Parent,
	}

	// Set defaults
	if tk.Status == "" {
		tk.Status = ticket.StatusOpen
	}
	if tk.Type == "" {
		tk.Type = ticket.TypeTask
	}
	if tk.Assignee == "" {
		tk.Assignee = "testuser"
	}

	if err := store.Create(tk); err != nil {
		t.Fatalf("failed to create test ticket: %v", err)
	}

	return tk.ID
}

// TicketOptions contains options for creating test tickets
type TicketOptions struct {
	Title    string
	Body     string
	Status   ticket.Status
	Type     ticket.Type
	Priority int
	Assignee string
	Deps     []string
	Links    []string
	Parent   string
}

// FixturePath returns the path to a fixture file
func FixturePath(name string) string {
	return filepath.Join("testdata", "fixtures", name)
}

// GoldenPath returns the path to a golden file
func GoldenPath(name string) string {
	return filepath.Join("testdata", "golden", name)
}

// AssertTicketEqual asserts two tickets are equal
func AssertTicketEqual(t *testing.T, expected, actual *ticket.Ticket) {
	t.Helper()

	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("ticket mismatch (-expected +actual):\n%s", diff)
	}
}

// AssertOutputEqual asserts two text outputs are equal
func AssertOutputEqual(t *testing.T, expected, actual string) {
	t.Helper()

	if expected != actual {
		t.Errorf("output mismatch:\nExpected:\n%s\n\nActual:\n%s", expected, actual)
	}
}

// AssertTicketValid checks that a ticket has all required fields
func AssertTicketValid(t *testing.T, tk *ticket.Ticket) {
	t.Helper()

	if tk.ID == "" {
		t.Error("ticket ID is empty")
	}
	if tk.Status == "" {
		t.Error("ticket status is empty")
	}
	if tk.Type == "" {
		t.Error("ticket type is empty")
	}
	if tk.Created.IsZero() {
		t.Error("ticket created time is zero")
	}
	if tk.Title == "" {
		t.Error("ticket title is empty")
	}
}

// AssertContains checks if a string contains a substring
func AssertContains(t *testing.T, haystack, needle string) {
	t.Helper()

	if !contains(haystack, needle) {
		t.Errorf("expected string to contain %q, got: %s", needle, haystack)
	}
}

// AssertNotContains checks if a string does not contain a substring
func AssertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()

	if contains(haystack, needle) {
		t.Errorf("expected string not to contain %q, got: %s", needle, haystack)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexString(s, substr) >= 0)
}

func indexString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
