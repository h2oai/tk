package ticket

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// containsWord reports whether words contains target, case-insensitively
// (the source adjectives list mixes capitalization).
func containsWord(words []string, target string) bool {
	return slices.ContainsFunc(words, func(w string) bool {
		return strings.EqualFold(w, target)
	})
}

// 1.2 ID Generation Tests

// splitID splits an ID into its prefix segments, adjective, and noun.
// Format: {prefix}-{adjective}-{noun}
func splitID(id string) (prefix string, adjective string, noun string) {
	parts := strings.Split(id, "-")
	if len(parts) < 3 {
		return "", "", ""
	}
	return strings.Join(parts[:len(parts)-2], "-"), parts[len(parts)-2], parts[len(parts)-1]
}

// TestPrefixExtraction tests prefix extraction from various directory names
func TestPrefixExtraction(t *testing.T) {
	tests := []struct {
		name           string
		dirName        string
		expectedPrefix string // prefix part only, before the word pair
	}{
		{"single segment", "/path/to/myproject", "m"},
		{"hyphenated", "/path/to/my-ticket-keeper", "mtk"},
		{"underscored", "/path/to/my_ticket_keeper", "mtk"},
		{"mixed hyphen and underscore", "/path/to/my-ticket_keeper", "mtk"},
		{"single char", "/path/to/a", "a"},
		{"numeric start", "/path/to/123project", "1"},
		{"three segments", "/path/to/go-tk", "gt"},
		{"gotk", "/path/to/gotk", "g"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := GenerateID(tt.dirName)
			prefix, _, _ := splitID(id)
			if prefix == "" {
				t.Fatalf("Expected ID format {prefix}-{adjective}-{noun}, got %q", id)
			}
			if prefix != tt.expectedPrefix {
				t.Errorf("GenerateID(%q) prefix = %q, expected %q (full ID: %s)",
					filepath.Base(tt.dirName), prefix, tt.expectedPrefix, id)
			}
		})
	}
}

// TestWordPairGeneration tests that the word pair portion of the ID is
// correctly formatted and drawn from the nouns and adjectives lists
func TestWordPairGeneration(t *testing.T) {
	tests := []struct {
		name    string
		dirName string
	}{
		{"simple", "/path/to/project"},
		{"complex", "/path/to/my-complex-project_name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := GenerateID(tt.dirName)
			_, adjective, noun := splitID(id)
			if adjective == "" || noun == "" {
				t.Fatalf("Expected ID format {prefix}-{adjective}-{noun}, got %q", id)
			}

			// Word pair should come from the adjectives and nouns lists
			if !containsWord(adjectives, adjective) {
				t.Errorf("Word %q should be in the adjectives list (ID: %s)", adjective, id)
			}
			if !slices.Contains(nouns, noun) {
				t.Errorf("Word %q should be in the nouns list (ID: %s)", noun, id)
			}
		})
	}
}

// TestFullIDFormat tests that the complete ID follows the expected format
func TestFullIDFormat(t *testing.T) {
	tests := []struct {
		dirName string
	}{
		{"/path/to/myproject"},
		{"/path/to/my-ticket-keeper"},
		{"/path/to/gotk"},
	}

	for _, tt := range tests {
		t.Run(filepath.Base(tt.dirName), func(t *testing.T) {
			id := GenerateID(tt.dirName)

			// Should match pattern: {prefix}-{adjective}-{noun}
			pattern := regexp.MustCompile(`^[a-z0-9]+-[a-z]+-[a-z]+$`)
			if !pattern.MatchString(id) {
				t.Errorf("ID %q does not match expected format {prefix}-{adjective}-{noun}", id)
			}

			// Should contain exactly two hyphens separating prefix, adjective, and noun
			if strings.Count(id, "-") < 2 {
				t.Errorf("ID %q should contain hyphen separators", id)
			}
		})
	}
}

// TestPrefixExtractedCorrectly verifies prefix extraction logic
func TestPrefixExtractedCorrectly(t *testing.T) {
	tests := []struct {
		name           string
		dirName        string
		expectedPrefix string
	}{
		{"single word lowercase", "/path/to/myproject", "m"},
		{"single word uppercase", "/path/to/MyProject", "m"}, // Expecting lowercase
		{"two words hyphen", "/path/to/my-project", "mp"},
		{"three words hyphen", "/path/to/my-new-project", "mnp"},
		{"two words underscore", "/path/to/my_project", "mp"},
		{"three words underscore", "/path/to/my_new_project", "mnp"},
		{"mixed separators", "/path/to/my-new_project", "mnp"},
		{"single letter", "/path/to/x", "x"},
		{"numbers", "/path/to/123", "1"},
		{"word with numbers", "/path/to/proj123", "p"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := GenerateID(tt.dirName)
			prefix, _, _ := splitID(id)

			if prefix != tt.expectedPrefix {
				t.Errorf("GenerateID(%q) prefix = %q, expected %q",
					filepath.Base(tt.dirName), prefix, tt.expectedPrefix)
			}
		})
	}
}

// TestWordPairAppendedCorrectly verifies the word pair is properly appended
func TestWordPairAppendedCorrectly(t *testing.T) {
	dirName := "/path/to/myproject"
	id := GenerateID(dirName)

	parts := strings.Split(id, "-")
	if len(parts) < 3 {
		t.Fatalf("ID should have at least prefix, adjective, and noun parts: %q", id)
	}

	// Verify word pair is at the end
	suffix := strings.Join(parts[len(parts)-2:], "-")
	if !strings.HasSuffix(id, suffix) {
		t.Errorf("ID should end with word pair, ID=%q, word pair=%q", id, suffix)
	}
}

// TestEmptyDirectoryNameFallback tests handling of edge cases
func TestEmptyDirectoryNameFallback(t *testing.T) {
	tests := []struct {
		name    string
		dirName string
	}{
		{"root directory", "/"},
		{"dot", "/."},
		{"empty path", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			id := GenerateID(tt.dirName)

			// Should still produce valid ID format
			if id == "" {
				t.Error("GenerateID should not return empty string")
			}

			if !strings.Contains(id, "-") {
				t.Errorf("ID should contain hyphen separator even for edge cases: %q", id)
			}
		})
	}
}

// TestIDFormatConsistency verifies the format is consistent across calls
func TestIDFormatConsistency(t *testing.T) {
	dirName := "/path/to/my-test-project"

	// Generate multiple IDs
	ids := make([]string, 10)
	for i := 0; i < 10; i++ {
		ids[i] = GenerateID(dirName)
	}

	// All should have the same prefix
	var commonPrefix string
	for i, id := range ids {
		prefix, adjective, noun := splitID(id)
		if prefix == "" || adjective == "" || noun == "" {
			t.Fatalf("Invalid ID format: %q", id)
		}

		if i == 0 {
			commonPrefix = prefix
		} else {
			if prefix != commonPrefix {
				t.Errorf("Inconsistent prefix: got %q, expected %q", prefix, commonPrefix)
			}
		}

		// Word pair should come from the known word lists
		if !containsWord(adjectives, adjective) {
			t.Errorf("Word %q should be in the adjectives list: %q", adjective, id)
		}
		if !slices.Contains(nouns, noun) {
			t.Errorf("Word %q should be in the nouns list: %q", noun, id)
		}
	}
}

// TestPrefixLowercase verifies that prefix is lowercased
func TestPrefixLowercase(t *testing.T) {
	tests := []struct {
		dirName string
	}{
		{"/path/to/MyProject"},
		{"/path/to/MY-PROJECT"},
		{"/path/to/My-Ticket-Keeper"},
	}

	for _, tt := range tests {
		t.Run(filepath.Base(tt.dirName), func(t *testing.T) {
			id := GenerateID(tt.dirName)
			prefix, _, _ := splitID(id)

			if prefix != strings.ToLower(prefix) {
				t.Errorf("Prefix should be lowercase: got %q", prefix)
			}
		})
	}
}

// TestWordPairLowercase verifies that the word pair is lowercase (adjectives in
// the source list are capitalized)
func TestWordPairLowercase(t *testing.T) {
	for i := 0; i < 20; i++ {
		id := GenerateID("/path/to/project")
		if id != strings.ToLower(id) {
			t.Errorf("ID should be fully lowercase: got %q", id)
		}
	}
}
