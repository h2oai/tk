package store

import (
	"crypto/rand"
	"strings"
)

// q and x are excluded from consonants since they read awkwardly in the
// consonant-vowel pattern. The digit 0 is excluded to avoid confusion with o.
// sonorants open the consonant cluster of a CVCCV stem so it stays sayable.
const (
	consonants = "bcdfghjklmnprstvwyz"
	vowels     = "aeiou"
	sonorants  = "lmnrs"
	digits     = "123456789"
)

// stemPatterns are the three shapes the five letters of an ID's stem may take.
var stemPatterns = [3][5]string{
	{consonants, vowels, consonants, vowels, consonants}, // CVCVC, e.g. "fanir"
	{vowels, consonants, vowels, consonants, vowels},     // VCVCV, e.g. "igoro"
	{consonants, vowels, sonorants, consonants, vowels},  // CVCCV, e.g. "banto"
}

// GenerateID returns a short pronounceable random ID: a CVCVC, VCVCV or CVCCV
// stem followed by a digit 1-9 (e.g. "fanir7", "igoro8", "banto3"). The digit
// guarantees an ID is never a real word.
//
// Every shape has two consonants and two vowels in common; the fifth letter
// is a consonant (CVCVC), a vowel (VCVCV) or a sonorant opening the cluster
// (CVCCV), so the shapes hold stems in the ratio consonants:vowels:sonorants
// (19:5:5). Picking the shape with that weight makes every possible ID
// equally likely.
func GenerateID() string {
	var pattern [5]string
	switch r := randomIndex(len(consonants) + len(vowels) + len(sonorants)); {
	case r < len(consonants):
		pattern = stemPatterns[0]
	case r < len(consonants)+len(vowels):
		pattern = stemPatterns[1]
	default:
		pattern = stemPatterns[2]
	}
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
