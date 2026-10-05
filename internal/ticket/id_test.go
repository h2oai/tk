package ticket

import (
	"regexp"
	"testing"
)

// 1.2 ID Generation Tests

// idPatterns are the two valid ID shapes: a CVCVC stem or a VCVCV stem, each
// followed by a digit.
var idPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^[` + consonants + `][` + vowels + `][` + consonants + `][` + vowels + `][` + consonants + `][` + digits + `]$`),
	regexp.MustCompile(`^[` + vowels + `][` + consonants + `][` + vowels + `][` + consonants + `][` + vowels + `][` + digits + `]$`),
}

// TestGenerateIDFormat tests that IDs are a CVCVC or VCVCV stem plus a digit
func TestGenerateIDFormat(t *testing.T) {
	for i := 0; i < 100; i++ {
		id := GenerateID()
		valid := false
		for _, p := range idPatterns {
			if p.MatchString(id) {
				valid = true
				break
			}
		}
		if !valid {
			t.Errorf("ID %q should follow the CVCVC-digit or VCVCV-digit pattern", id)
		}
	}
}

// TestGenerateIDCoversBothShapes tests that both stem shapes are produced
func TestGenerateIDCoversBothShapes(t *testing.T) {
	counts := make([]int, len(idPatterns))
	const n = 200
	for i := 0; i < n; i++ {
		id := GenerateID()
		for j, p := range idPatterns {
			if p.MatchString(id) {
				counts[j]++
			}
		}
	}
	for j, c := range counts {
		if c == 0 {
			t.Errorf("stem shape %d never generated in %d IDs", j, n)
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
