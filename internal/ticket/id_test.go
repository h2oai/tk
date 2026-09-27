package ticket

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 1.2 ID Generation Tests

// splitID splits an ID into its prefix and pronounceable suffix.
// Format: {prefix}-{suffix}
func splitID(id string) (prefix string, suffix string) {
	idx := strings.LastIndex(id, "-")
	if idx < 0 {
		return "", ""
	}
	return id[:idx], id[idx+1:]
}

// TestPrefixExtraction tests prefix extraction from various directory names
func TestPrefixExtraction(t *testing.T) {
	tests := []struct {
		name           string
		dirName        string
		expectedPrefix string // prefix part only, before the suffix
	}{
		{"single segment", "/path/to/myproject", "myp"},
		{"hyphenated", "/path/to/my-ticket-keeper", "mtk"},
		{"underscored", "/path/to/my_ticket_keeper", "mtk"},
		{"mixed hyphen and underscore", "/path/to/my-ticket_keeper", "mtk"},
		{"single char", "/path/to/a", "a"},
		{"numeric start", "/path/to/123project", "123"},
		{"three segments", "/path/to/go-tk", "got"},
		{"gotk", "/path/to/gotk", "got"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := GenerateID(tt.dirName)
			prefix, suffix := splitID(id)
			if prefix == "" || suffix == "" {
				t.Fatalf("Expected ID format {prefix}-{suffix}, got %q", id)
			}
			if prefix != tt.expectedPrefix {
				t.Errorf("GenerateID(%q) prefix = %q, expected %q (full ID: %s)",
					filepath.Base(tt.dirName), prefix, tt.expectedPrefix, id)
			}
		})
	}
}

// TestPronounceableSuffixGeneration tests that the suffix portion of the ID
// is correctly formatted as a consonant-vowel-consonant-vowel-consonant string
func TestPronounceableSuffixGeneration(t *testing.T) {
	tests := []struct {
		name    string
		dirName string
	}{
		{"simple", "/path/to/project"},
		{"complex", "/path/to/my-complex-project_name"},
	}

	suffixPattern := regexp.MustCompile(`^[` + consonants + `][` + vowels + `][` + consonants + `][` + vowels + `][` + consonants + `]$`)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := GenerateID(tt.dirName)
			_, suffix := splitID(id)
			if suffix == "" {
				t.Fatalf("Expected ID format {prefix}-{suffix}, got %q", id)
			}

			if !suffixPattern.MatchString(suffix) {
				t.Errorf("Suffix %q should follow the consonant-vowel-consonant-vowel-consonant pattern (ID: %s)", suffix, id)
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

			// Should match pattern: {prefix}-{suffix}
			pattern := regexp.MustCompile(`^[a-z0-9]+-[a-z]{5}$`)
			if !pattern.MatchString(id) {
				t.Errorf("ID %q does not match expected format {prefix}-{suffix}", id)
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
		{"single word lowercase", "/path/to/myproject", "myp"},
		{"single word uppercase", "/path/to/MyProject", "myp"}, // Expecting lowercase
		{"two words hyphen", "/path/to/my-project", "myp"},
		{"three words hyphen", "/path/to/my-new-project", "mnp"},
		{"two words underscore", "/path/to/my_project", "myp"},
		{"three words underscore", "/path/to/my_new_project", "mnp"},
		{"mixed separators", "/path/to/my-new_project", "mnp"},
		{"single letter", "/path/to/x", "x"},
		{"numbers", "/path/to/123", "123"},
		{"word with numbers", "/path/to/proj123", "pro"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := GenerateID(tt.dirName)
			prefix, _ := splitID(id)

			if prefix != tt.expectedPrefix {
				t.Errorf("GenerateID(%q) prefix = %q, expected %q",
					filepath.Base(tt.dirName), prefix, tt.expectedPrefix)
			}
		})
	}
}

// TestSuffixAppendedCorrectly verifies the suffix is properly appended
func TestSuffixAppendedCorrectly(t *testing.T) {
	dirName := "/path/to/myproject"
	id := GenerateID(dirName)

	prefix, suffix := splitID(id)
	if prefix == "" || suffix == "" {
		t.Fatalf("ID should have both a prefix and a suffix part: %q", id)
	}

	if !strings.HasSuffix(id, suffix) {
		t.Errorf("ID should end with suffix, ID=%q, suffix=%q", id, suffix)
	}
	if len(suffix) != 5 {
		t.Errorf("Suffix should be 5 characters long, got %q (len %d)", suffix, len(suffix))
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

	suffixPattern := regexp.MustCompile(`^[` + consonants + `][` + vowels + `][` + consonants + `][` + vowels + `][` + consonants + `]$`)

	// Generate multiple IDs
	ids := make([]string, 10)
	for i := 0; i < 10; i++ {
		ids[i] = GenerateID(dirName)
	}

	// All should have the same prefix
	var commonPrefix string
	for i, id := range ids {
		prefix, suffix := splitID(id)
		if prefix == "" || suffix == "" {
			t.Fatalf("Invalid ID format: %q", id)
		}

		if i == 0 {
			commonPrefix = prefix
		} else {
			if prefix != commonPrefix {
				t.Errorf("Inconsistent prefix: got %q, expected %q", prefix, commonPrefix)
			}
		}

		// Suffix should follow the pronounceable pattern
		if !suffixPattern.MatchString(suffix) {
			t.Errorf("Suffix %q should follow the consonant-vowel-consonant-vowel-consonant pattern: %q", suffix, id)
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
			prefix, _ := splitID(id)

			if prefix != strings.ToLower(prefix) {
				t.Errorf("Prefix should be lowercase: got %q", prefix)
			}
		})
	}
}

// TestSuffixLowercase verifies that the suffix is lowercase
func TestSuffixLowercase(t *testing.T) {
	for i := 0; i < 20; i++ {
		id := GenerateID("/path/to/project")
		if id != strings.ToLower(id) {
			t.Errorf("ID should be fully lowercase: got %q", id)
		}
	}
}
