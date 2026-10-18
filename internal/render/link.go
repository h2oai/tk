package render

import (
	"regexp"
	"slices"
	"strings"
)

var (
	// word matches a run of id characters (see store.validID); a run is linked
	// only when it is a whole known id, so `fanir7x` or `fanir7-y` stay plain.
	word = regexp.MustCompile(`[A-Za-z0-9_-]+`)
	// bareURL matches a URL written without <>, as GitHub autolinks it.
	bareURL = regexp.MustCompile(`^(?:[A-Za-z][A-Za-z0-9+.-]*://|www\.)[^\s<>]*`)
	// refDef matches a link reference definition, `[name]: url`.
	refDef = regexp.MustCompile(`^ {0,3}\[[^\]]+\]:`)
	// fenceLine matches a code fence line: the marker run, then the rest.
	fenceLine = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")
)

// Linkify rewrites every whole word of md for which href returns a target
// into a `[word](target)` link; href returns "" for words that are not ids.
// It leaves fenced code blocks, inline code spans, links, images, autolinks,
// HTML tags, bare URLs and reference definitions as they are. Constructs are
// recognised within a single line.
func Linkify(md string, href func(word string) string) string {
	var b strings.Builder
	var f fence
	for _, line := range strings.SplitAfter(md, "\n") {
		if f.step(line) || refDef.MatchString(line) {
			b.WriteString(line)
			continue
		}
		start := 0
		for i := 0; i < len(line); {
			n := opaque(line, i)
			if n == 0 {
				i++
				continue
			}
			b.WriteString(linkWords(line[start:i], href))
			b.WriteString(line[i : i+n])
			i += n
			start = i
		}
		b.WriteString(linkWords(line[start:], href))
	}
	return b.String()
}

// Mentions returns the distinct words of md for which known is true, in order
// of first appearance, under the same rules as Linkify: a mention is a word
// Linkify would link.
func Mentions(md string, known func(word string) bool) []string {
	var out []string
	Linkify(md, func(w string) string {
		if known(w) && !slices.Contains(out, w) {
			out = append(out, w)
		}
		return ""
	})
	return out
}

func linkWords(s string, href func(string) string) string {
	return word.ReplaceAllStringFunc(s, func(w string) string {
		if h := href(w); h != "" {
			return "[" + w + "](" + h + ")"
		}
		return w
	})
}

// fence tracks whether a line sequence is inside a fenced code block.
type fence struct{ marker string }

// step reports whether line belongs to a fenced code block, fence lines
// included, and updates the state.
func (f *fence) step(line string) bool {
	m := fenceLine.FindStringSubmatch(strings.TrimRight(line, "\r\n"))
	if f.marker != "" {
		if m != nil && m[1][0] == f.marker[0] && len(m[1]) >= len(f.marker) && strings.TrimSpace(m[2]) == "" {
			f.marker = ""
		}
		return true
	}
	// A backtick fence's info string cannot contain a backtick.
	if m != nil && (m[1][0] == '~' || !strings.Contains(m[2], "`")) {
		f.marker = m[1]
		return true
	}
	return false
}

// opaque returns the length of the construct starting at s[i] whose text must
// not be rewritten, or 0 if s[i] is ordinary text.
func opaque(s string, i int) int {
	switch c := s[i]; {
	case c == '\\':
		if i+1 < len(s) && strings.IndexByte(punct, s[i+1]) >= 0 {
			return 2
		}
	case c == '`':
		n := run(s, i, '`')
		if end := closeTicks(s, i+n, n); end >= 0 {
			return end - i
		}
		return n
	case c == '!' && i+1 < len(s) && s[i+1] == '[':
		if n := link(s, i+1); n > 0 {
			return n + 1
		}
	case c == '[':
		return link(s, i)
	case c == '<':
		if j := strings.IndexByte(s[i:], '>'); j > 1 && tagStart(s[i+1]) {
			return j + 1
		}
	case isLetter(c) && (i == 0 || !isWord(s[i-1])):
		return len(bareURL.FindString(s[i:]))
	}
	return 0
}

const punct = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"

// run returns the length of the run of c starting at s[i].
func run(s string, i int, c byte) int {
	n := 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

// closeTicks returns the index just past the first run of exactly n backticks
// at or after j, or -1.
func closeTicks(s string, j, n int) int {
	for j < len(s) {
		if s[j] != '`' {
			j++
			continue
		}
		m := run(s, j, '`')
		if m == n {
			return j + m
		}
		j += m
	}
	return -1
}

// link returns the length of `[text](dest)` or `[text][ref]` starting at
// s[i] == '[', or 0 if there is none.
func link(s string, i int) int {
	j := matching(s, i, '[', ']')
	if j < 0 || j+1 >= len(s) {
		return 0
	}
	switch s[j+1] {
	case '(':
		if k := matching(s, j+1, '(', ')'); k >= 0 {
			return k + 1 - i
		}
	case '[':
		if k := matching(s, j+1, '[', ']'); k >= 0 {
			return k + 1 - i
		}
	}
	return 0
}

// matching returns the index of the bracket closing s[i], skipping
// backslash escapes, or -1.
func matching(s string, i int, open, close byte) int {
	depth := 0
	for j := i; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case open:
			depth++
		case close:
			if depth--; depth == 0 {
				return j
			}
		}
	}
	return -1
}

func tagStart(c byte) bool { return isLetter(c) || c == '/' || c == '!' || c == '?' }

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func isWord(c byte) bool { return isLetter(c) || c >= '0' && c <= '9' || c == '_' || c == '-' }
