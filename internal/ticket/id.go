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
// Format: {prefix}-{adjective}-{noun}, e.g. "g-baking-badger".
// The prefix is extracted from the directory name by taking the first letter
// of each hyphen/underscore-separated segment, or the first 3 chars as fallback.
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

	// Fallback to first 3 chars if no segments produced a prefix
	if prefix == "" {
		runes := []rune(dirName)
		if len(runes) > 3 {
			prefix = string(runes[:3])
		} else {
			prefix = dirName
		}
	}

	return fmt.Sprintf("%s-%s-%s", strings.ToLower(prefix), randomAdjective(), randomNoun())
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
