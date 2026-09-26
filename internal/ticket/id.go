package ticket

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

// GenerateID generates a new ticket ID based on the current directory name
// and a word pair (adjective-noun) chosen with cryptographic randomness.
// Format: {prefix}-{adjective}-{noun}, e.g. "got-baking-badger".
// The prefix is extracted from the directory name by taking the first letter
// of each hyphen/underscore-separated segment; if that yields fewer than 3
// characters, the first 3 alphanumeric characters of the directory name are
// used instead (so separators never leak into the prefix).
// The word pair is drawn from the nouns and adjectives lists in words.go, giving
// len(adjectives) * len(nouns) possible IDs per prefix.
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

	return fmt.Sprintf("%s-%s-%s", strings.ToLower(prefix), randomAdjective(), randomNoun())
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

// randomAdjective returns a random adjective from the adjectives list.
func randomAdjective() string {
	return randomWord(adjectives)
}

// randomNoun returns a random noun from the nouns list.
func randomNoun() string {
	return randomWord(nouns)
}

// randomWord picks a random word using crypto/rand.
func randomWord(words []string) string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Fallback to first word (extremely unlikely)
		return words[0]
	}
	n := binary.BigEndian.Uint32(b[:])
	return words[n%uint32(len(words))]
}
