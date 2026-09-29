package ticket

import (
	"regexp"
	"testing"
)

// 1.2 ID Generation Tests

var idPattern = regexp.MustCompile(`^[` + consonants + `][` + vowels + `][` + consonants + `][` + vowels + `][` + consonants + `][` + digits + `]$`)

// TestGenerateIDFormat tests that IDs are a consonant-vowel-consonant-vowel-consonant string plus a digit
func TestGenerateIDFormat(t *testing.T) {
	for i := 0; i < 100; i++ {
		id := GenerateID()
		if !idPattern.MatchString(id) {
			t.Errorf("ID %q should follow the consonant-vowel-consonant-vowel-consonant-digit pattern", id)
		}
	}
}

// TestGenerateIDVaries tests that generated IDs are random
func TestGenerateIDVaries(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		seen[GenerateID()] = true
	}
	if len(seen) < 40 {
		t.Errorf("expected mostly unique IDs, got %d unique out of 50", len(seen))
	}
}
