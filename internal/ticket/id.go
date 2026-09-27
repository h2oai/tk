package ticket

import (
	"crypto/rand"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// consonants and vowels used to build a pronounceable suffix. q and x are
// excluded from consonants since they read awkwardly in the consonant-vowel
// pattern used below (e.g. "qa", "xu").
const (
	consonants = "bcdfghjklmnprstvwyz"
	vowels     = "aeiou"
)

// GenerateID generates a new ticket ID based on the current directory name
// and a short, pronounceable random suffix.
// Format: {prefix}-{suffix}, e.g. "got-fanix".
// The prefix is extracted from the directory name by taking the first letter
// of each hyphen/underscore-separated segment; if that yields fewer than 3
// characters, the first 3 alphanumeric characters of the directory name are
// used instead (so separators never leak into the prefix).
// The suffix follows a consonant-vowel-consonant-vowel-consonant pattern,
// giving len(consonants)^3 * len(vowels)^2 possible suffixes per prefix.
func GenerateID(cwd string) string {
	dirName := filepath.Base(cwd)

	// Extract first letter of each segment (split by hyphen or underscore)
	segments := strings.FieldsFunc(dirName, func(r rune) bool {
		return r == '-' || r == '_'
	})

	var prefix string
	for _, seg := range segments {
		if len(seg) > 0 {
			// Take first rune to handle unicode correctly
			for _, r := range seg {
				if unicode.IsLetter(r) || unicode.IsDigit(r) {
					prefix += string(r)
					break
				}
			}
		}
	}

	// Fallback to the first 3 alphanumeric characters of the dir name if fewer
	// than 3 prefix characters were produced, so separators never leak into the
	// prefix (e.g. "go-tk" -> "got", not "go-").
	if len([]rune(prefix)) < 3 {
		prefix = firstAlphanumeric(dirName, 3)
	}

	return fmt.Sprintf("%s-%s", strings.ToLower(prefix), randomPronounceable())
}

// firstAlphanumeric returns up to n alphanumeric runes from s, ignoring any
// other characters. If s contains no alphanumeric runes, s is returned as-is.
func firstAlphanumeric(s string, n int) string {
	var b strings.Builder
	count := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			count++
			if count >= n {
				break
			}
		}
	}
	if b.Len() == 0 {
		return s
	}
	return b.String()
}

// randomPronounceable generates a random 5-character string following a
// consonant-vowel-consonant-vowel-consonant pattern (e.g. "fanix", "lovex"),
// chosen with cryptographic randomness.
func randomPronounceable() string {
	pattern := [5]string{consonants, vowels, consonants, vowels, consonants}
	var b strings.Builder
	for _, set := range pattern {
		b.WriteByte(set[randomIndex(len(set))])
	}
	return b.String()
}

// randomIndex returns a cryptographically random index in [0, n).
func randomIndex(n int) int {
	var b [1]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback to first index (extremely unlikely)
		return 0
	}
	return int(b[0]) % n
}
