package store

import (
	"crypto/rand"
	"strings"
)

// q and x are excluded from consonants since they read awkwardly in the
// consonant-vowel pattern. The digit 0 is excluded to avoid confusion with o.
const (
	consonants = "bcdfghjklmnprstvwyz"
	vowels     = "aeiou"
	digits     = "123456789"
)

// stemPatterns are the two shapes the five letters of an ID's stem may take.
var stemPatterns = [2][5]string{
	{consonants, vowels, consonants, vowels, consonants}, // CVCVC, e.g. "fanir"
	{vowels, consonants, vowels, consonants, vowels},     // VCVCV, e.g. "igoro"
}

// GenerateID returns a short pronounceable random ID: a CVCVC or VCVCV stem
// followed by a digit 1-9 (e.g. "fanir7", "igoro8"). The digit guarantees an
// ID is never a real word.
func GenerateID() string {
	pattern := stemPatterns[randomIndex(len(stemPatterns))]
	var b strings.Builder
	for _, set := range pattern {
		b.WriteByte(set[randomIndex(len(set))])
	}
	b.WriteByte(digits[randomIndex(len(digits))])
	return b.String()
}

// randomIndex returns a random index in [0, n) using rejection sampling to
// avoid modulo bias.
func randomIndex(n int) int {
	limit := 256 - 256%n
	var b [1]byte
	for {
		if _, err := rand.Read(b[:]); err != nil {
			panic(err)
		}
		if int(b[0]) < limit {
			return int(b[0]) % n
		}
	}
}
