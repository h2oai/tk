package deptree

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/h2oai/tk/internal/ticket"
)

// captureOutput captures stdout during function execution
func captureOutput(f func()) string {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outC := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	f()

	w.Close()
	os.Stdout = oldStdout
	return <-outC
}

// createTestTicket creates a ticket with specified ID, title, status, and deps
func createTestTicket(id, title string, status ticket.Status, deps []string) *ticket.Ticket {
	return &ticket.Ticket{
		ID:      id,
		Status:  status,
		Title:   title,
		Deps:    deps,
		Created: time.Now(),
		Type:    ticket.TypeTask,
	}
}

// TestSimpleLinearChain tests A <- B <- C dependency chain
func TestSimpleLinearChain(t *testing.T) {
	tickets := map[string]*ticket.Ticket{
		"a-1111": createTestTicket("a-1111", "Ticket A", ticket.StatusOpen, []string{}),
		"b-2222": createTestTicket("b-2222", "Ticket B", ticket.StatusOpen, []string{"a-1111"}),
		"c-3333": createTestTicket("c-3333", "Ticket C", ticket.StatusOpen, []string{"b-2222"}),
	}

	tree := Build(tickets, "c-3333", false)

	output := captureOutput(func() {
		tree.Render()
	})

	// Should contain all three tickets
	if !strings.Contains(output, "c-3333") {
		t.Error("tree should contain root c-3333")
	}
	if !strings.Contains(output, "b-2222") {
		t.Error("tree should contain b-2222")
	}
	if !strings.Contains(output, "a-1111") {
		t.Error("tree should contain a-1111")
	}

	// Check order: C at root, then B, then A
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}

	if !strings.Contains(lines[0], "c-3333") {
		t.Errorf("first line should be c-3333, got: %s", lines[0])
	}
	if !strings.Contains(lines[1], "b-2222") {
		t.Errorf("second line should be b-2222, got: %s", lines[1])
	}
	if !strings.Contains(lines[2], "a-1111") {
		t.Errorf("third line should be a-1111, got: %s", lines[2])
	}
}

// TestDiamondDependency tests diamond structure with deduplication
func TestDiamondDependency(t *testing.T) {
	// Create diamond:
	//      D
	//     / \
	//    B   C
	//     \ /
	//      A
	tickets := map[string]*ticket.Ticket{
		"a-1111": createTestTicket("a-1111", "Ticket A", ticket.StatusOpen, []string{}),
		"b-2222": createTestTicket("b-2222", "Ticket B", ticket.StatusOpen, []string{"a-1111"}),
		"c-3333": createTestTicket("c-3333", "Ticket C", ticket.StatusOpen, []string{"a-1111"}),
		"d-4444": createTestTicket("d-4444", "Ticket D", ticket.StatusOpen, []string{"b-2222", "c-3333"}),
	}

	tree := Build(tickets, "d-4444", false)

	output := captureOutput(func() {
		tree.Render()
	})

	// In normal mode, A should appear only once (at max depth)
	count := strings.Count(output, "a-1111")
	if count != 1 {
		t.Errorf("in normal mode, A should appear once, appeared %d times", count)
	}

	// All four tickets should be present
	if !strings.Contains(output, "d-4444") {
		t.Error("tree should contain d-4444")
	}
	if !strings.Contains(output, "b-2222") {
		t.Error("tree should contain b-2222")
	}
	if !strings.Contains(output, "c-3333") {
		t.Error("tree should contain c-3333")
	}
	if !strings.Contains(output, "a-1111") {
		t.Error("tree should contain a-1111")
	}
}

// TestMaxDepthComputation tests max depth calculation
func TestMaxDepthComputation(t *testing.T) {
	// Create structure where A appears at multiple depths
	tickets := map[string]*ticket.Ticket{
		"a-1111": createTestTicket("a-1111", "Ticket A", ticket.StatusOpen, []string{}),
		"b-2222": createTestTicket("b-2222", "Ticket B", ticket.StatusOpen, []string{"a-1111"}),
		"c-3333": createTestTicket("c-3333", "Ticket C", ticket.StatusOpen, []string{"b-2222"}),
		"d-4444": createTestTicket("d-4444", "Ticket D", ticket.StatusOpen, []string{"a-1111", "c-3333"}),
	}

	tree := Build(tickets, "d-4444", false)

	// A appears at depth 1 (via D->A) and depth 3 (via D->C->B->A)
	// MaxDepth should be 3
	nodeA := tree.nodes["a-1111"]
	if nodeA.MaxDepth != 3 {
		t.Errorf("A MaxDepth = %d, want 3", nodeA.MaxDepth)
	}
}

// TestSubtreeDepthComputation tests subtree depth calculation
func TestSubtreeDepthComputation(t *testing.T) {
	tickets := map[string]*ticket.Ticket{
		"a-1111": createTestTicket("a-1111", "Ticket A", ticket.StatusOpen, []string{}),
		"b-2222": createTestTicket("b-2222", "Ticket B", ticket.StatusOpen, []string{}),
		"c-3333": createTestTicket("c-3333", "Ticket C", ticket.StatusOpen, []string{"a-1111"}),
		"d-4444": createTestTicket("d-4444", "Ticket D", ticket.StatusOpen, []string{"b-2222", "c-3333"}),
	}

	tree := Build(tickets, "d-4444", false)

	// D has two children: B (leaf, depth 1) and C (depth 2 with A)
	// D's subtreeDepth should be max of its children
	nodeD := tree.nodes["d-4444"]
	nodeC := tree.nodes["c-3333"]
	nodeB := tree.nodes["b-2222"]

	if nodeB.SubtreeDepth < nodeC.SubtreeDepth {
		t.Logf("B subtree depth (%d) < C subtree depth (%d) - as expected", nodeB.SubtreeDepth, nodeC.SubtreeDepth)
	}

	// Verify D has the maximum of its children
	if nodeD.SubtreeDepth < nodeC.SubtreeDepth {
		t.Errorf("D subtree depth should be at least C's depth")
	}
}

// TestSortingOfChildren tests that children are sorted by subtree depth
func TestSortingOfChildren(t *testing.T) {
	// Create structure where D has children with different subtree depths
	tickets := map[string]*ticket.Ticket{
		"a-1111": createTestTicket("a-1111", "Ticket A", ticket.StatusOpen, []string{}),
		"b-2222": createTestTicket("b-2222", "Ticket B", ticket.StatusOpen, []string{}),
		"c-3333": createTestTicket("c-3333", "Ticket C", ticket.StatusOpen, []string{"a-1111"}),
		"d-4444": createTestTicket("d-4444", "Ticket D", ticket.StatusOpen, []string{"b-2222", "c-3333"}),
	}

	tree := Build(tickets, "d-4444", false)

	output := captureOutput(func() {
		tree.Render()
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Find positions of B and C in output
	posB := -1
	posC := -1
	for i, line := range lines {
		if strings.Contains(line, "b-2222") {
			posB = i
		}
		if strings.Contains(line, "c-3333") {
			posC = i
		}
	}

	if posB == -1 || posC == -1 {
		t.Fatal("could not find B or C in output")
	}

	// B (shallower) should come before C (deeper)
	if posB > posC {
		t.Errorf("B (shallow) should appear before C (deep), but posB=%d, posC=%d", posB, posC)
	}
}

// TestCycleDetection tests that cycles are handled without infinite loops
func TestCycleDetection(t *testing.T) {
	// Create cycle: A -> B -> C -> A
	tickets := map[string]*ticket.Ticket{
		"a-1111": createTestTicket("a-1111", "Ticket A", ticket.StatusOpen, []string{"c-3333"}),
		"b-2222": createTestTicket("b-2222", "Ticket B", ticket.StatusOpen, []string{"a-1111"}),
		"c-3333": createTestTicket("c-3333", "Ticket C", ticket.StatusOpen, []string{"b-2222"}),
	}

	tree := Build(tickets, "a-1111", false)

	// Should not panic or hang
	output := captureOutput(func() {
		tree.Render()
	})

	// Should complete and contain all tickets
	if !strings.Contains(output, "a-1111") {
		t.Error("tree should contain a-1111")
	}
	if !strings.Contains(output, "b-2222") {
		t.Error("tree should contain b-2222")
	}
	if !strings.Contains(output, "c-3333") {
		t.Error("tree should contain c-3333")
	}

	// Verify no infinite output
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) > 10 {
		t.Errorf("output too long for cycle test, got %d lines", len(lines))
	}
}

// TestRootNode tests that root is always printed
func TestRootNode(t *testing.T) {
	tickets := map[string]*ticket.Ticket{
		"a-1111": createTestTicket("a-1111", "Root Ticket", ticket.StatusOpen, []string{}),
	}

	tree := Build(tickets, "a-1111", false)

	output := captureOutput(func() {
		tree.Render()
	})

	// Root should be in output
	if !strings.Contains(output, "a-1111") {
		t.Error("root ticket should be in output")
	}
	if !strings.Contains(output, "Root Ticket") {
		t.Error("root ticket title should be in output")
	}
}

// TestTreeRenderingOutput tests the format of tree output
func TestTreeRenderingOutput(t *testing.T) {
	tickets := map[string]*ticket.Ticket{
		"a-1111": createTestTicket("a-1111", "Ticket A", ticket.StatusOpen, []string{}),
		"b-2222": createTestTicket("b-2222", "Ticket B", ticket.StatusInProgress, []string{"a-1111"}),
	}

	tree := Build(tickets, "b-2222", false)

	output := captureOutput(func() {
		tree.Render()
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Check root format: {id} [{status}] {title}
	if !strings.Contains(lines[0], "b-2222") {
		t.Error("root line should contain ID")
	}
	if !strings.Contains(lines[0], "[in_progress]") {
		t.Error("root line should contain status in brackets")
	}
	if !strings.Contains(lines[0], "Ticket B") {
		t.Error("root line should contain title")
	}

	// Check child format with connector
	if len(lines) > 1 {
		if !strings.Contains(lines[1], "└── ") && !strings.Contains(lines[1], "├── ") {
			t.Error("child line should have tree connector")
		}
		if !strings.Contains(lines[1], "a-1111") {
			t.Error("child line should contain ID")
		}
		if !strings.Contains(lines[1], "[open]") {
			t.Error("child line should contain status")
		}
		if !strings.Contains(lines[1], "Ticket A") {
			t.Error("child line should contain title")
		}
	}
}

// TestFullMode tests that --full flag shows all occurrences
func TestFullMode(t *testing.T) {
	// Diamond structure
	tickets := map[string]*ticket.Ticket{
		"a-1111": createTestTicket("a-1111", "Ticket A", ticket.StatusOpen, []string{}),
		"b-2222": createTestTicket("b-2222", "Ticket B", ticket.StatusOpen, []string{"a-1111"}),
		"c-3333": createTestTicket("c-3333", "Ticket C", ticket.StatusOpen, []string{"a-1111"}),
		"d-4444": createTestTicket("d-4444", "Ticket D", ticket.StatusOpen, []string{"b-2222", "c-3333"}),
	}

	tree := Build(tickets, "d-4444", true) // full=true

	output := captureOutput(func() {
		tree.Render()
	})

	// In full mode, A should appear twice (under both B and C)
	count := strings.Count(output, "a-1111")
	if count < 2 {
		t.Errorf("in full mode, A should appear at least twice, appeared %d times", count)
	}
}

// TestEmptyTree tests single ticket with no deps
func TestEmptyTree(t *testing.T) {
	tickets := map[string]*ticket.Ticket{
		"a-1111": createTestTicket("a-1111", "Single Ticket", ticket.StatusOpen, []string{}),
	}

	tree := Build(tickets, "a-1111", false)

	output := captureOutput(func() {
		tree.Render()
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should be exactly one line
	if len(lines) != 1 {
		t.Errorf("empty tree should have 1 line, got %d", len(lines))
	}

	// Format: {id} [{status}] {title}
	if !strings.Contains(lines[0], "a-1111") {
		t.Error("output should contain ID")
	}
	if !strings.Contains(lines[0], "[open]") {
		t.Error("output should contain status")
	}
	if !strings.Contains(lines[0], "Single Ticket") {
		t.Error("output should contain title")
	}
}

// TestTreeConnectors tests box-drawing characters
func TestTreeConnectors(t *testing.T) {
	tickets := map[string]*ticket.Ticket{
		"a-1111": createTestTicket("a-1111", "Ticket A", ticket.StatusOpen, []string{}),
		"b-2222": createTestTicket("b-2222", "Ticket B", ticket.StatusOpen, []string{}),
		"c-3333": createTestTicket("c-3333", "Ticket C", ticket.StatusOpen, []string{"a-1111", "b-2222"}),
	}

	tree := Build(tickets, "c-3333", false)

	output := captureOutput(func() {
		tree.Render()
	})

	// Should contain tree connectors
	if !strings.Contains(output, "├── ") && !strings.Contains(output, "└── ") {
		t.Error("output should contain tree connectors (├── or └──)")
	}

	// Last child should use └──
	lines := strings.Split(strings.TrimSpace(output), "\n")
	lastChildLine := lines[len(lines)-1]
	if !strings.Contains(lastChildLine, "└── ") {
		t.Error("last child should use └── connector")
	}
}

// TestMultiLevelTree tests deeper tree structure
func TestMultiLevelTree(t *testing.T) {
	tickets := map[string]*ticket.Ticket{
		"a-1111": createTestTicket("a-1111", "Ticket A", ticket.StatusClosed, []string{}),
		"b-2222": createTestTicket("b-2222", "Ticket B", ticket.StatusClosed, []string{"a-1111"}),
		"c-3333": createTestTicket("c-3333", "Ticket C", ticket.StatusInProgress, []string{"b-2222"}),
		"d-4444": createTestTicket("d-4444", "Ticket D", ticket.StatusOpen, []string{"c-3333"}),
	}

	tree := Build(tickets, "d-4444", false)

	output := captureOutput(func() {
		tree.Render()
	})

	// All tickets should appear
	for id := range tickets {
		if !strings.Contains(output, id) {
			t.Errorf("tree should contain %s", id)
		}
	}

	// Different statuses should be shown
	if !strings.Contains(output, "[open]") {
		t.Error("should show [open] status")
	}
	if !strings.Contains(output, "[in_progress]") {
		t.Error("should show [in_progress] status")
	}
	if !strings.Contains(output, "[closed]") {
		t.Error("should show [closed] status")
	}

	// Check indentation for nested structure
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 4 {
		t.Errorf("expected 4 levels, got %d lines", len(lines))
	}
}

// TestBuildWithMissingDeps tests handling of missing dependencies
func TestBuildWithMissingDeps(t *testing.T) {
	tickets := map[string]*ticket.Ticket{
		"b-2222": createTestTicket("b-2222", "Ticket B", ticket.StatusOpen, []string{"nonexistent"}),
	}

	tree := Build(tickets, "b-2222", false)

	// Should not panic
	output := captureOutput(func() {
		tree.Render()
	})

	// Should at least show the root
	if !strings.Contains(output, "b-2222") {
		t.Error("tree should show root even with missing deps")
	}
}

// TestStatusDisplay tests that different statuses are displayed correctly
func TestStatusDisplay(t *testing.T) {
	tickets := map[string]*ticket.Ticket{
		"a-1111": createTestTicket("a-1111", "Open Ticket", ticket.StatusOpen, []string{}),
		"b-2222": createTestTicket("b-2222", "InProgress Ticket", ticket.StatusInProgress, []string{}),
		"c-3333": createTestTicket("c-3333", "Closed Ticket", ticket.StatusClosed, []string{}),
		"d-4444": createTestTicket("d-4444", "Root", ticket.StatusOpen, []string{"a-1111", "b-2222", "c-3333"}),
	}

	tree := Build(tickets, "d-4444", false)

	output := captureOutput(func() {
		tree.Render()
	})

	// All status types should appear
	if !strings.Contains(output, "[open]") {
		t.Error("should display [open] status")
	}
	if !strings.Contains(output, "[in_progress]") {
		t.Error("should display [in_progress] status")
	}
	if !strings.Contains(output, "[closed]") {
		t.Error("should display [closed] status")
	}
}
