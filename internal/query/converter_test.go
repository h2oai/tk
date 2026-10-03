package query

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/h2oai/tk/internal/ticket"
)

// TestToJSON tests the ToJSON function
func TestToJSON(t *testing.T) {
	t.Run("basic ticket conversion", func(t *testing.T) {
		now := time.Now().UTC()
		tk := &ticket.Ticket{
			ID:       "test-1234",
			Status:   ticket.StatusOpen,
			Type:     ticket.TypeTask,
			Priority: 2,
			Created:  now,
			Title:    "Test Ticket",
			Body:     "Test body",
		}

		jsonStr, err := ToJSON(tk)
		if err != nil {
			t.Fatalf("ToJSON error: %v", err)
		}

		// Parse to verify structure
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Verify fields
		if result["id"] != "test-1234" {
			t.Errorf("id = %v, want test-1234", result["id"])
		}
		if result["status"] != "open" {
			t.Errorf("status = %v, want open", result["status"])
		}
		if result["type"] != "task" {
			t.Errorf("type = %v, want task", result["type"])
		}
		if result["priority"] != "2" {
			t.Errorf("priority = %v, want 2 (as string)", result["priority"])
		}
	})

	t.Run("all fields present", func(t *testing.T) {
		now := time.Now().UTC()
		tk := &ticket.Ticket{
			ID:          "full-5678",
			Status:      ticket.StatusInProgress,
			Type:        ticket.TypeBug,
			Priority:    1,
			Created:     now,
			Assignee:    "alice",
			ExternalRef: "BUG-123",
			Parent:      "parent-9012",
			Deps:        []string{"dep1", "dep2"},
			Links:       []string{"link1"},
			Title:       "Full Ticket",
			Body:        "Full body",
		}

		jsonStr, err := ToJSON(tk)
		if err != nil {
			t.Fatalf("ToJSON error: %v", err)
		}

		// Parse
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Verify all fields
		if result["id"] != "full-5678" {
			t.Errorf("id = %v, want full-5678", result["id"])
		}
		if result["status"] != "in_progress" {
			t.Errorf("status = %v, want in_progress", result["status"])
		}
		if result["type"] != "bug" {
			t.Errorf("type = %v, want bug", result["type"])
		}
		if result["priority"] != "1" {
			t.Errorf("priority = %v, want 1", result["priority"])
		}
		if result["assignee"] != "alice" {
			t.Errorf("assignee = %v, want alice", result["assignee"])
		}
		if result["external-ref"] != "BUG-123" {
			t.Errorf("external-ref = %v, want BUG-123", result["external-ref"])
		}
		if result["parent"] != "parent-9012" {
			t.Errorf("parent = %v, want parent-9012", result["parent"])
		}

		// Verify arrays
		deps, ok := result["deps"].([]interface{})
		if !ok {
			t.Fatal("deps is not an array")
		}
		if len(deps) != 2 {
			t.Errorf("deps length = %d, want 2", len(deps))
		}
		if deps[0] != "dep1" || deps[1] != "dep2" {
			t.Errorf("deps = %v, want [dep1, dep2]", deps)
		}

		links, ok := result["links"].([]interface{})
		if !ok {
			t.Fatal("links is not an array")
		}
		if len(links) != 1 {
			t.Errorf("links length = %d, want 1", len(links))
		}
		if links[0] != "link1" {
			t.Errorf("links = %v, want [link1]", links)
		}
	})

	t.Run("types are correct", func(t *testing.T) {
		now := time.Now().UTC()
		tk := &ticket.Ticket{
			ID:       "type-test",
			Status:   ticket.StatusClosed,
			Type:     ticket.TypeFeature,
			Priority: 3,
			Created:  now,
			Deps:     []string{},
			Links:    []string{},
		}

		jsonStr, err := ToJSON(tk)
		if err != nil {
			t.Fatalf("ToJSON error: %v", err)
		}

		// Parse
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Check types
		if _, ok := result["id"].(string); !ok {
			t.Error("id should be string")
		}
		if _, ok := result["status"].(string); !ok {
			t.Error("status should be string")
		}
		if _, ok := result["type"].(string); !ok {
			t.Error("type should be string")
		}
		if _, ok := result["priority"].(string); !ok {
			t.Error("priority should be string, not number")
		}
		if _, ok := result["created"].(string); !ok {
			t.Error("created should be string (ISO format)")
		}
		if _, ok := result["deps"].([]interface{}); !ok {
			t.Error("deps should be array")
		}
		if _, ok := result["links"].([]interface{}); !ok {
			t.Error("links should be array")
		}
	})

	t.Run("empty arrays are not nil", func(t *testing.T) {
		now := time.Now().UTC()
		tk := &ticket.Ticket{
			ID:       "empty-arrays",
			Status:   ticket.StatusOpen,
			Type:     ticket.TypeTask,
			Priority: 2,
			Created:  now,
			// Deps and Links are nil by default
		}

		jsonStr, err := ToJSON(tk)
		if err != nil {
			t.Fatalf("ToJSON error: %v", err)
		}

		// Should contain [] not null
		if strings.Contains(jsonStr, `"deps":null`) {
			t.Error("deps should be [] not null")
		}
		if strings.Contains(jsonStr, `"links":null`) {
			t.Error("links should be [] not null")
		}
		if !strings.Contains(jsonStr, `"deps":[]`) {
			t.Error("deps should be serialized as []")
		}
		if !strings.Contains(jsonStr, `"links":[]`) {
			t.Error("links should be serialized as []")
		}

		// Parse and verify
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		deps, ok := result["deps"].([]interface{})
		if !ok {
			t.Fatal("deps is not an array")
		}
		if len(deps) != 0 {
			t.Errorf("deps should be empty, got %d items", len(deps))
		}

		links, ok := result["links"].([]interface{})
		if !ok {
			t.Fatal("links is not an array")
		}
		if len(links) != 0 {
			t.Errorf("links should be empty, got %d items", len(links))
		}
	})

	t.Run("created timestamp format", func(t *testing.T) {
		// Use specific time for testing
		testTime := time.Date(2025, 1, 11, 10, 30, 45, 0, time.UTC)
		tk := &ticket.Ticket{
			ID:       "time-test",
			Status:   ticket.StatusOpen,
			Type:     ticket.TypeTask,
			Priority: 2,
			Created:  testTime,
		}

		jsonStr, err := ToJSON(tk)
		if err != nil {
			t.Fatalf("ToJSON error: %v", err)
		}

		// Should contain ISO format timestamp
		expectedTime := "2025-01-11T10:30:45Z"
		if !strings.Contains(jsonStr, expectedTime) {
			t.Errorf("expected timestamp %s in JSON: %s", expectedTime, jsonStr)
		}
	})

	t.Run("omitempty fields", func(t *testing.T) {
		now := time.Now().UTC()
		tk := &ticket.Ticket{
			ID:       "omit-test",
			Status:   ticket.StatusOpen,
			Type:     ticket.TypeTask,
			Priority: 2,
			Created:  now,
			// Assignee, ExternalRef, Parent omitted
		}

		jsonStr, err := ToJSON(tk)
		if err != nil {
			t.Fatalf("ToJSON error: %v", err)
		}

		// Parse
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Optional fields should either be absent or empty strings
		if val, ok := result["assignee"]; ok && val != "" {
			// If present, should be empty string
		}
		if val, ok := result["external-ref"]; ok && val != "" {
			// If present, should be empty string
		}
		if val, ok := result["parent"]; ok && val != "" {
			// If present, should be empty string
		}
	})

	t.Run("priority as string not number", func(t *testing.T) {
		now := time.Now().UTC()
		tk := &ticket.Ticket{
			ID:       "priority-test",
			Status:   ticket.StatusOpen,
			Type:     ticket.TypeTask,
			Priority: 0, // Test boundary value
			Created:  now,
		}

		jsonStr, err := ToJSON(tk)
		if err != nil {
			t.Fatalf("ToJSON error: %v", err)
		}

		// Should contain "0" as string, not as number
		if !strings.Contains(jsonStr, `"priority":"0"`) {
			t.Errorf("priority should be string \"0\", got: %s", jsonStr)
		}
	})
}

// TestFilter tests the Filter function
func TestFilter(t *testing.T) {
	t.Run("simple equality filter", func(t *testing.T) {
		jsonLines := []string{
			`{"id":"a","priority":"0"}`,
			`{"id":"b","priority":"2"}`,
			`{"id":"c","priority":"0"}`,
		}

		results, err := Filter(jsonLines, `.priority == "0"`)
		if err != nil {
			t.Fatalf("Filter error: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}

		// Verify correct items filtered
		if !strings.Contains(results[0], `"id":"a"`) {
			t.Error("first result should be ticket a")
		}
		if !strings.Contains(results[1], `"id":"c"`) {
			t.Error("second result should be ticket c")
		}
	})

	t.Run("filter with and condition", func(t *testing.T) {
		jsonLines := []string{
			`{"id":"a","priority":"0","status":"open"}`,
			`{"id":"b","priority":"0","status":"closed"}`,
			`{"id":"c","priority":"2","status":"open"}`,
		}

		results, err := Filter(jsonLines, `.priority == "0" and .status == "open"`)
		if err != nil {
			t.Fatalf("Filter error: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}

		if !strings.Contains(results[0], `"id":"a"`) {
			t.Error("result should be ticket a")
		}
	})

	t.Run("filter with or condition", func(t *testing.T) {
		jsonLines := []string{
			`{"id":"a","priority":"0"}`,
			`{"id":"b","priority":"2"}`,
			`{"id":"c","priority":"4"}`,
		}

		results, err := Filter(jsonLines, `.priority == "0" or .priority == "4"`)
		if err != nil {
			t.Fatalf("Filter error: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("filter with array length", func(t *testing.T) {
		jsonLines := []string{
			`{"id":"a","deps":[]}`,
			`{"id":"b","deps":["d1"]}`,
			`{"id":"c","deps":["d1","d2"]}`,
		}

		results, err := Filter(jsonLines, `(.deps | length) > 0`)
		if err != nil {
			t.Fatalf("Filter error: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("expected 2 results, got %d", len(results))
		}

		// Should not include ticket a (empty deps)
		for _, result := range results {
			if strings.Contains(result, `"id":"a"`) {
				t.Error("result should not include ticket a with empty deps")
			}
		}
	})

	t.Run("filter with explicit select", func(t *testing.T) {
		jsonLines := []string{
			`{"id":"a","priority":"0"}`,
			`{"id":"b","priority":"2"}`,
		}

		results, err := Filter(jsonLines, `select(.priority == "0")`)
		if err != nil {
			t.Fatalf("Filter error: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		jsonLines := []string{
			`{"id":"a","priority":"2"}`,
			`{"id":"b","priority":"2"}`,
		}

		results, err := Filter(jsonLines, `.priority == "0"`)
		if err != nil {
			t.Fatalf("Filter error: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("invalid JSON skipped", func(t *testing.T) {
		jsonLines := []string{
			`{"id":"a","priority":"0"}`,
			`invalid json`,
			`{"id":"c","priority":"0"}`,
		}

		// Should not error, just skip invalid lines
		results, err := Filter(jsonLines, `.priority == "0"`)
		if err != nil {
			t.Fatalf("Filter should not error on invalid JSON: %v", err)
		}

		// Should get valid results
		if len(results) != 2 {
			t.Errorf("expected 2 results (skipping invalid), got %d", len(results))
		}
	})

	t.Run("invalid filter returns error", func(t *testing.T) {
		jsonLines := []string{
			`{"id":"a"}`,
		}

		_, err := Filter(jsonLines, `invalid filter !@#`)
		if err == nil {
			t.Error("expected error for invalid filter")
		}
	})

	t.Run("filter with nested field access", func(t *testing.T) {
		jsonLines := []string{
			`{"id":"a","type":"bug"}`,
			`{"id":"b","type":"feature"}`,
		}

		results, err := Filter(jsonLines, `.type == "bug"`)
		if err != nil {
			t.Fatalf("Filter error: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("expected 1 result, got %d", len(results))
		}

		if !strings.Contains(results[0], `"id":"a"`) {
			t.Error("result should be ticket a")
		}
	})

	t.Run("empty input returns empty", func(t *testing.T) {
		jsonLines := []string{}

		results, err := Filter(jsonLines, `.priority == "0"`)
		if err != nil {
			t.Fatalf("Filter error: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("expected 0 results for empty input, got %d", len(results))
		}
	})
}

// TestRoundTrip tests converting ticket to JSON and back
func TestRoundTrip(t *testing.T) {
	t.Run("round trip preserves data", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		original := &ticket.Ticket{
			ID:          "round-trip",
			Status:      ticket.StatusInProgress,
			Type:        ticket.TypeFeature,
			Priority:    1,
			Created:     now,
			Assignee:    "bob",
			ExternalRef: "FEAT-456",
			Parent:      "parent-123",
			Deps:        []string{"dep1", "dep2"},
			Links:       []string{"link1"},
			Title:       "Round Trip Test",
			Body:        "Test body content",
		}

		// Convert to JSON
		jsonStr, err := ToJSON(original)
		if err != nil {
			t.Fatalf("ToJSON error: %v", err)
		}

		// Parse back
		var parsed TicketJSON
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Verify all fields match
		if parsed.ID != original.ID {
			t.Errorf("ID mismatch: %v != %v", parsed.ID, original.ID)
		}
		if parsed.Status != string(original.Status) {
			t.Errorf("Status mismatch: %v != %v", parsed.Status, original.Status)
		}
		if parsed.Type != string(original.Type) {
			t.Errorf("Type mismatch: %v != %v", parsed.Type, original.Type)
		}
		if parsed.Priority != "1" {
			t.Errorf("Priority mismatch: %v != 1", parsed.Priority)
		}
		if parsed.Assignee != original.Assignee {
			t.Errorf("Assignee mismatch: %v != %v", parsed.Assignee, original.Assignee)
		}
		if parsed.ExternalRef != original.ExternalRef {
			t.Errorf("ExternalRef mismatch: %v != %v", parsed.ExternalRef, original.ExternalRef)
		}
		if parsed.Parent != original.Parent {
			t.Errorf("Parent mismatch: %v != %v", parsed.Parent, original.Parent)
		}

		// Verify arrays
		if len(parsed.Deps) != len(original.Deps) {
			t.Errorf("Deps length mismatch: %d != %d", len(parsed.Deps), len(original.Deps))
		}
		for i, dep := range parsed.Deps {
			if dep != original.Deps[i] {
				t.Errorf("Dep[%d] mismatch: %v != %v", i, dep, original.Deps[i])
			}
		}

		if len(parsed.Links) != len(original.Links) {
			t.Errorf("Links length mismatch: %d != %d", len(parsed.Links), len(original.Links))
		}
		for i, link := range parsed.Links {
			if link != original.Links[i] {
				t.Errorf("Link[%d] mismatch: %v != %v", i, link, original.Links[i])
			}
		}

		// Verify timestamp format
		expectedTime := now.Format("2006-01-02T15:04:05Z")
		if parsed.Created != expectedTime {
			t.Errorf("Created timestamp mismatch: %v != %v", parsed.Created, expectedTime)
		}
	})

	t.Run("convert to JSON and back preserves structure", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		original := &ticket.Ticket{
			ID:       "structure-test",
			Status:   ticket.StatusOpen,
			Type:     ticket.TypeTask,
			Priority: 2,
			Created:  now,
			Deps:     []string{},
			Links:    []string{},
		}

		// First conversion
		json1, err := ToJSON(original)
		if err != nil {
			t.Fatalf("first ToJSON error: %v", err)
		}

		// Parse and convert again
		var parsed TicketJSON
		if err := json.Unmarshal([]byte(json1), &parsed); err != nil {
			t.Fatalf("parse error: %v", err)
		}

		// Recreate ticket from parsed JSON
		roundtrip := &ticket.Ticket{
			ID:          parsed.ID,
			Status:      ticket.Status(parsed.Status),
			Type:        ticket.Type(parsed.Type),
			Priority:    2, // Would need conversion from string
			Created:     now,
			Assignee:    parsed.Assignee,
			ExternalRef: parsed.ExternalRef,
			Parent:      parsed.Parent,
			Deps:        parsed.Deps,
			Links:       parsed.Links,
		}

		// Second conversion
		json2, err := ToJSON(roundtrip)
		if err != nil {
			t.Fatalf("second ToJSON error: %v", err)
		}

		// JSON outputs should be identical
		if json1 != json2 {
			t.Errorf("JSON outputs differ:\n%s\n!=\n%s", json1, json2)
		}
	})
}
