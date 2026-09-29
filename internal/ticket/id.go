package ticket

import (
	"crypto/rand"
	"strings"
)

// consonants, vowels and digits used to build an ID. q and x are
// excluded from consonants since they read awkwardly in the consonant-vowel
// pattern used below (e.g. "qa", "xu").
const (
	consonants = "bcdfghjklmnprstvwyz"
	vowels     = "aeiou"
	digits     = "0123456789"
)

// GenerateID generates a new ticket ID: a short, pronounceable random string
// following a consonant-vowel-consonant-vowel-consonant pattern, followed by a
// digit (e.g. "fanir7"). The trailing digit guarantees an ID is never a real
// word, so IDs stay easy to grep. This gives
// len(consonants)^3 * len(vowels)^2 * len(digits) possible IDs.
func GenerateID() string {
	return randomPronounceable() + string(digits[randomIndex(len(digits))])
}

// randomPronounceable generates a random 5-character string following a
// consonant-vowel-consonant-vowel-consonant pattern (e.g. "fanir", "lovet"),
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
