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
	digits     = "123456789"
)

// stemPatterns are the two shapes the five letters of an ID's stem may take.
// One is picked at random for each ID, then a digit is appended. Both shapes
// are five letters and three syllables, so IDs stay short and pronounceable.
var stemPatterns = [2][5]string{
	{consonants, vowels, consonants, vowels, consonants}, // CVCVC, e.g. "fanir"
	{vowels, consonants, vowels, consonants, vowels},     // VCVCV, e.g. "igoro"
}

// GenerateID generates a new ticket ID: a short, pronounceable random string
// following either a consonant-vowel-consonant-vowel-consonant pattern
// (e.g. "fanir7") or a vowel-consonant-vowel-consonant-vowel pattern
// (e.g. "igoro8"), followed by a digit. The trailing digit guarantees an ID is
// never a real word, so IDs stay easy to grep. This gives
// (len(consonants)^3*len(vowels)^2 + len(vowels)^3*len(consonants)^2) *
// len(digits) possible IDs.
func GenerateID() string {
	return randomPronounceable() + string(digits[randomIndex(len(digits))])
}

// randomPronounceable generates a random 5-character string following one of
// stemPatterns (e.g. "fanir", "lovet", "igoro"), chosen with cryptographic
// randomness.
func randomPronounceable() string {
	pattern := stemPatterns[randomIndex(len(stemPatterns))]
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
