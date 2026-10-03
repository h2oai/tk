//go:build integration
// +build integration

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/h2oai/tk/internal/ticket"
	"github.com/h2oai/tk/testdata"
)

// TestCompleteWorkflow tests the complete ticket workflow:
// Create tickets, add dependencies, close tickets, check blocked/ready status
func TestCompleteWorkflow(t *testing.T) {
	store, tempDir := testdata.CreateTestStore(t)
	defer testdata.CleanupTestStore(t, tempDir)

	// Step 1: Create ticket A with title
	idA := testdata.CreateTestTicketWithOptions(t, store, testdata.TicketOptions{
		Title:    "Ticket A",
		Body:     "This is ticket A",
		Status:   ticket.StatusOpen,
		Priority: 2,
	})

	// Step 2: Create ticket B depends on A
	idB := testdata.CreateTestTicketWithOptions(t, store, testdata.TicketOptions{
		Title:    "Ticket B",
		Body:     "This is ticket B",
		Status:   ticket.StatusOpen,
		Priority: 2,
		Deps:     []string{idA},
	})

	// Step 3: Create ticket C depends on A
	idC := testdata.CreateTestTicketWithOptions(t, store, testdata.TicketOptions{
		Title:    "Ticket C",
		Body:     "This is ticket C",
		Status:   ticket.StatusOpen,
		Priority: 2,
		Deps:     []string{idA},
	})

	// Step 4: Verify dependency structure
	ticketB, err := store.Get(idB)
	if err != nil {
		t.Fatalf("failed to get ticket B: %v", err)
	}
	if len(ticketB.Deps) != 1 || ticketB.Deps[0] != idA {
		t.Errorf("ticket B should depend on A, got deps: %v", ticketB.Deps)
	}

	ticketC, err := store.Get(idC)
	if err != nil {
		t.Fatalf("failed to get ticket C: %v", err)
	}
	if len(ticketC.Deps) != 1 || ticketC.Deps[0] != idA {
		t.Errorf("ticket C should depend on A, got deps: %v", ticketC.Deps)
	}

	// Step 5: Check that B and C are blocked (A is still open)
	allTickets, err := store.List()
	if err != nil {
		t.Fatalf("failed to list tickets: %v", err)
	}

	// B and C should be blocked because A is open
	for _, tk := range allTickets {
		if tk.ID == idB || tk.ID == idC {
			if len(tk.Deps) == 0 {
				t.Errorf("ticket %s should have dependencies", tk.ID)
			}
			depA, err := store.Get(idA)
			if err != nil {
				t.Fatalf("failed to get dependency A: %v", err)
			}
			if depA.Status == ticket.StatusClosed {
				t.Error("A should still be open")
			}
		}
	}

	// Step 6: Close ticket A
	ticketA, err := store.Get(idA)
	if err != nil {
		t.Fatalf("failed to get ticket A: %v", err)
	}
	ticketA.Status = ticket.StatusClosed
	if err := store.Update(ticketA); err != nil {
		t.Fatalf("failed to close ticket A: %v", err)
	}

	// Step 7: Verify A is closed
	ticketA, err = store.Get(idA)
	if err != nil {
		t.Fatalf("failed to get ticket A after closing: %v", err)
	}
	if ticketA.Status != ticket.StatusClosed {
		t.Errorf("ticket A should be closed, got status: %s", ticketA.Status)
	}

	// Step 8: Now B and C should be ready (no blockers)
	ticketB, err = store.Get(idB)
	if err != nil {
		t.Fatalf("failed to get ticket B: %v", err)
	}
	// Verify B's dependency A is closed
	depA, err := store.Get(ticketB.Deps[0])
	if err != nil {
		t.Fatalf("failed to get B's dependency: %v", err)
	}
	if depA.Status != ticket.StatusClosed {
		t.Error("B's dependency A should be closed")
	}

	// Step 9: Close ticket B
	ticketB.Status = ticket.StatusClosed
	if err := store.Update(ticketB); err != nil {
		t.Fatalf("failed to close ticket B: %v", err)
	}

	// Step 10: List closed tickets - should show A and B
	closedTickets := []string{}
	allTickets, err = store.List()
	if err != nil {
		t.Fatalf("failed to list tickets: %v", err)
	}
	for _, tk := range allTickets {
		if tk.Status == ticket.StatusClosed {
			closedTickets = append(closedTickets, tk.ID)
		}
	}

	if len(closedTickets) != 2 {
		t.Errorf("expected 2 closed tickets, got %d: %v", len(closedTickets), closedTickets)
	}

	// Verify A and B are in the closed list
	hasA, hasB := false, false
	for _, id := range closedTickets {
		if id == idA {
			hasA = true
		}
		if id == idB {
			hasB = true
		}
	}
	if !hasA || !hasB {
		t.Errorf("closed list should contain A and B, got: %v", closedTickets)
	}

	// Verify C is still open
	ticketC, err = store.Get(idC)
	if err != nil {
		t.Fatalf("failed to get ticket C: %v", err)
	}
	if ticketC.Status != ticket.StatusOpen {
		t.Errorf("ticket C should still be open, got: %s", ticketC.Status)
	}
}

// TestLinkingWorkflow tests the linking workflow:
// Create tickets, link them, verify links, unlink
func TestLinkingWorkflow(t *testing.T) {
	store, tempDir := testdata.CreateTestStore(t)
	defer testdata.CleanupTestStore(t, tempDir)

	// Step 1: Create tickets A, B, C
	idA := testdata.CreateTestTicket(t, store, "Ticket A")
	idB := testdata.CreateTestTicket(t, store, "Ticket B")
	idC := testdata.CreateTestTicket(t, store, "Ticket C")

	// Step 2: Link A ↔ B
	ticketA, err := store.Get(idA)
	if err != nil {
		t.Fatalf("failed to get ticket A: %v", err)
	}
	ticketA.Links = append(ticketA.Links, idB)
	if err := store.Update(ticketA); err != nil {
		t.Fatalf("failed to update ticket A: %v", err)
	}

	ticketB, err := store.Get(idB)
	if err != nil {
		t.Fatalf("failed to get ticket B: %v", err)
	}
	ticketB.Links = append(ticketB.Links, idA)
	if err := store.Update(ticketB); err != nil {
		t.Fatalf("failed to update ticket B: %v", err)
	}

	// Step 3: Link B ↔ C
	ticketB, err = store.Get(idB)
	if err != nil {
		t.Fatalf("failed to get ticket B: %v", err)
	}
	ticketB.Links = append(ticketB.Links, idC)
	if err := store.Update(ticketB); err != nil {
		t.Fatalf("failed to update ticket B: %v", err)
	}

	ticketC, err := store.Get(idC)
	if err != nil {
		t.Fatalf("failed to get ticket C: %v", err)
	}
	ticketC.Links = append(ticketC.Links, idB)
	if err := store.Update(ticketC); err != nil {
		t.Fatalf("failed to update ticket C: %v", err)
	}

	// Step 4: Show A (shows B as linked)
	ticketA, err = store.Get(idA)
	if err != nil {
		t.Fatalf("failed to get ticket A: %v", err)
	}
	if len(ticketA.Links) != 1 || ticketA.Links[0] != idB {
		t.Errorf("ticket A should be linked to B, got: %v", ticketA.Links)
	}

	// Step 5: Show B (shows A and C as linked)
	ticketB, err = store.Get(idB)
	if err != nil {
		t.Fatalf("failed to get ticket B: %v", err)
	}
	if len(ticketB.Links) != 2 {
		t.Errorf("ticket B should have 2 links, got: %d", len(ticketB.Links))
	}
	hasLinkA, hasLinkC := false, false
	for _, link := range ticketB.Links {
		if link == idA {
			hasLinkA = true
		}
		if link == idC {
			hasLinkC = true
		}
	}
	if !hasLinkA || !hasLinkC {
		t.Errorf("ticket B should be linked to A and C, got: %v", ticketB.Links)
	}

	// Step 6: Unlink A ↔ B
	ticketA, err = store.Get(idA)
	if err != nil {
		t.Fatalf("failed to get ticket A: %v", err)
	}
	// Remove B from A's links
	newLinks := []string{}
	for _, link := range ticketA.Links {
		if link != idB {
			newLinks = append(newLinks, link)
		}
	}
	ticketA.Links = newLinks
	if err := store.Update(ticketA); err != nil {
		t.Fatalf("failed to update ticket A: %v", err)
	}

	ticketB, err = store.Get(idB)
	if err != nil {
		t.Fatalf("failed to get ticket B: %v", err)
	}
	// Remove A from B's links
	newLinks = []string{}
	for _, link := range ticketB.Links {
		if link != idA {
			newLinks = append(newLinks, link)
		}
	}
	ticketB.Links = newLinks
	if err := store.Update(ticketB); err != nil {
		t.Fatalf("failed to update ticket B: %v", err)
	}

	// Step 7: Show A (no linked section)
	ticketA, err = store.Get(idA)
	if err != nil {
		t.Fatalf("failed to get ticket A: %v", err)
	}
	if len(ticketA.Links) != 0 {
		t.Errorf("ticket A should have no links, got: %v", ticketA.Links)
	}

	// Step 8: Show B (only C linked)
	ticketB, err = store.Get(idB)
	if err != nil {
		t.Fatalf("failed to get ticket B: %v", err)
	}
	if len(ticketB.Links) != 1 || ticketB.Links[0] != idC {
		t.Errorf("ticket B should only be linked to C, got: %v", ticketB.Links)
	}
}

// TestQueryWorkflow tests the query workflow:
// Create tickets with mixed priorities, query by priority and status
func TestQueryWorkflow(t *testing.T) {
	store, tempDir := testdata.CreateTestStore(t)
	defer testdata.CleanupTestStore(t, tempDir)

	// Step 1: Create 5 tickets with mixed priorities
	priorities := []int{0, 1, 2, 3, 4}
	ids := make([]string, 5)

	for i, priority := range priorities {
		ids[i] = testdata.CreateTestTicketWithOptions(t, store, testdata.TicketOptions{
			Title:    "Ticket " + string(rune('A'+i)),
			Body:     "Test ticket body",
			Priority: priority,
			Status:   ticket.StatusOpen,
		})
	}

	// Step 2: Query - get priority 0 tickets
	allTickets, err := store.List()
	if err != nil {
		t.Fatalf("failed to list tickets: %v", err)
	}

	priority0Tickets := []string{}
	for _, tk := range allTickets {
		if tk.Priority == 0 {
			priority0Tickets = append(priority0Tickets, tk.ID)
		}
	}

	if len(priority0Tickets) != 1 {
		t.Errorf("expected 1 priority 0 ticket, got %d", len(priority0Tickets))
	}
	if priority0Tickets[0] != ids[0] {
		t.Errorf("priority 0 ticket should be %s, got %s", ids[0], priority0Tickets[0])
	}

	// Step 3: Query - get open tickets
	openTickets := []string{}
	for _, tk := range allTickets {
		if tk.Status == ticket.StatusOpen {
			openTickets = append(openTickets, tk.ID)
		}
	}

	if len(openTickets) != 5 {
		t.Errorf("expected 5 open tickets, got %d", len(openTickets))
	}

	// Step 4: Close one ticket
	ticketToClose, err := store.Get(ids[0])
	if err != nil {
		t.Fatalf("failed to get ticket: %v", err)
	}
	ticketToClose.Status = ticket.StatusClosed
	if err := store.Update(ticketToClose); err != nil {
		t.Fatalf("failed to close ticket: %v", err)
	}

	// Step 5: Query - get open tickets again
	allTickets, err = store.List()
	if err != nil {
		t.Fatalf("failed to list tickets: %v", err)
	}

	openTickets = []string{}
	for _, tk := range allTickets {
		if tk.Status == ticket.StatusOpen {
			openTickets = append(openTickets, tk.ID)
		}
	}

	if len(openTickets) != 4 {
		t.Errorf("expected 4 open tickets, got %d", len(openTickets))
	}

	// Step 6: Query - combine filters (open AND priority >= 2)
	combinedTickets := []string{}
	for _, tk := range allTickets {
		if tk.Status == ticket.StatusOpen && tk.Priority >= 2 {
			combinedTickets = append(combinedTickets, tk.ID)
		}
	}

	if len(combinedTickets) != 3 {
		t.Errorf("expected 3 tickets matching combined filter, got %d", len(combinedTickets))
	}
}

// TestEditWorkflow tests the edit and note workflow:
// Create ticket, add note, verify note appears in body
func TestEditWorkflow(t *testing.T) {
	store, tempDir := testdata.CreateTestStore(t)
	defer testdata.CleanupTestStore(t, tempDir)

	// Step 1: Create ticket
	id := testdata.CreateTestTicket(t, store, "Test Ticket")

	// Step 2: Get the ticket
	tk, err := store.Get(id)
	if err != nil {
		t.Fatalf("failed to get ticket: %v", err)
	}

	originalBody := tk.Body

	// Step 3: Add note
	timestamp := time.Now().UTC().Format(time.RFC3339)
	noteText := "This is a test note"

	// Simulate add-note command by appending to body
	notesSection := "\n\n## Notes\n\n**" + timestamp + "**\n\n" + noteText + "\n"

	if strings.Contains(tk.Body, "## Notes") {
		// Append to existing notes section
		tk.Body = tk.Body + "\n**" + timestamp + "**\n\n" + noteText + "\n"
	} else {
		// Create new notes section
		tk.Body = tk.Body + notesSection
	}

	if err := store.Update(tk); err != nil {
		t.Fatalf("failed to update ticket with note: %v", err)
	}

	// Step 4: Show → includes note
	tk, err = store.Get(id)
	if err != nil {
		t.Fatalf("failed to get ticket after adding note: %v", err)
	}

	if !strings.Contains(tk.Body, noteText) {
		t.Errorf("ticket body should contain note text, got: %s", tk.Body)
	}

	if !strings.Contains(tk.Body, "## Notes") {
		t.Error("ticket body should contain Notes section")
	}

	// Verify original body is still present
	if !strings.Contains(tk.Body, originalBody) {
		t.Error("original body should be preserved")
	}

	// Step 5: Add another note
	timestamp2 := time.Now().UTC().Format(time.RFC3339)
	noteText2 := "This is a second note"

	tk.Body = tk.Body + "\n**" + timestamp2 + "**\n\n" + noteText2 + "\n"

	if err := store.Update(tk); err != nil {
		t.Fatalf("failed to update ticket with second note: %v", err)
	}

	// Verify both notes are present
	tk, err = store.Get(id)
	if err != nil {
		t.Fatalf("failed to get ticket after adding second note: %v", err)
	}

	if !strings.Contains(tk.Body, noteText) {
		t.Error("first note should still be present")
	}

	if !strings.Contains(tk.Body, noteText2) {
		t.Error("second note should be present")
	}
}

// TestTicketFileFormat tests that tickets are written in the correct format
func TestTicketFileFormat(t *testing.T) {
	store, tempDir := testdata.CreateTestStore(t)
	defer testdata.CleanupTestStore(t, tempDir)

	// Create a ticket
	id := testdata.CreateTestTicketWithOptions(t, store, testdata.TicketOptions{
		Title:    "Test Ticket",
		Body:     "Test body content",
		Status:   ticket.StatusOpen,
		Type:     ticket.TypeTask,
		Priority: 2,
		Assignee: "testuser",
		Deps:     []string{"dep-1", "dep-2"},
		Links:    []string{"link-1"},
	})

	// Read the file directly
	filePath := filepath.Join(tempDir, ".tickets", id+".md")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read ticket file: %v", err)
	}

	contentStr := string(content)

	// Verify frontmatter delimiters
	if !strings.HasPrefix(contentStr, "---\n") {
		t.Error("file should start with ---")
	}

	lines := strings.Split(contentStr, "\n")
	secondDelimiterFound := false
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			secondDelimiterFound = true
			break
		}
	}
	if !secondDelimiterFound {
		t.Error("file should have closing --- delimiter")
	}

	// Verify array format (flow style with comma-space)
	if !strings.Contains(contentStr, "deps: [dep-1, dep-2]") {
		t.Error("deps should be in flow style with comma-space separator")
	}

	if !strings.Contains(contentStr, "links: [link-1]") {
		t.Error("links should be in flow style")
	}

	// Verify title format
	if !strings.Contains(contentStr, "# Test Ticket") {
		t.Error("title should be in markdown heading format")
	}

	// Verify body is after frontmatter
	bodyIndex := strings.Index(contentStr, "Test body content")
	if bodyIndex == -1 {
		t.Error("body content not found in file")
	}

	// Verify body comes after second ---
	secondDelimiterIndex := 0
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			secondDelimiterIndex = len(strings.Join(lines[:i+1], "\n"))
			break
		}
	}

	if bodyIndex < secondDelimiterIndex {
		t.Error("body should come after frontmatter")
	}
}

// TestConcurrentAccess tests that multiple operations can happen concurrently
func TestConcurrentAccess(t *testing.T) {
	store, tempDir := testdata.CreateTestStore(t)
	defer testdata.CleanupTestStore(t, tempDir)

	// Create initial tickets
	ids := testdata.CreateTestTickets(t, store, 5)

	// Create a second store instance accessing the same directory
	store2 := ticket.NewFileStore(filepath.Join(tempDir, ".tickets"))

	// Concurrent reads
	done := make(chan bool)
	errors := make(chan error, 10)

	// Reader 1
	go func() {
		for _, id := range ids {
			if _, err := store.Get(id); err != nil {
				errors <- err
			}
		}
		done <- true
	}()

	// Reader 2
	go func() {
		for _, id := range ids {
			if _, err := store2.Get(id); err != nil {
				errors <- err
			}
		}
		done <- true
	}()

	// Writer
	go func() {
		for _, id := range ids {
			tk, err := store.Get(id)
			if err != nil {
				errors <- err
				continue
			}
			tk.Priority = 3
			if err := store.Update(tk); err != nil {
				errors <- err
			}
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	close(errors)

	// Check for errors
	var buf bytes.Buffer
	for err := range errors {
		buf.WriteString(err.Error() + "\n")
	}

	if buf.Len() > 0 {
		t.Errorf("concurrent access errors:\n%s", buf.String())
	}

	// Verify all tickets were updated
	for _, id := range ids {
		tk, err := store.Get(id)
		if err != nil {
			t.Errorf("failed to get ticket after concurrent updates: %v", err)
			continue
		}
		if tk.Priority != 3 {
			t.Errorf("ticket %s priority should be 3, got %d", id, tk.Priority)
		}
	}
}
