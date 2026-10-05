package store

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const fence = "---\n"

// flowList marshals as a YAML flow sequence ([a, b]) and is omitted when empty.
type flowList []string

func (l flowList) MarshalYAML() (any, error) {
	n := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, s := range l {
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: s})
	}
	return n, nil
}

type frontmatter struct {
	ID        string    `yaml:"id"`
	Status    Status    `yaml:"status"`
	Type      Type      `yaml:"type"`
	BlockedBy flowList  `yaml:"blocked-by,omitempty"`
	Children  flowList  `yaml:"children,omitempty"`
	Created   time.Time `yaml:"created"`
}

// Marshal serializes a ticket to its on-disk form:
//
//	---
//	<frontmatter>
//	---
//	# Title
//
//	Body
//
// The blank line after the title is present only when the body is non-empty;
// the body is otherwise written verbatim.
func Marshal(t *Ticket) ([]byte, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	ty := t.Type
	if ty == "" {
		ty = TypeTask
	}
	fm, err := yaml.Marshal(frontmatter{
		ID:        t.ID,
		Status:    t.Status,
		Type:      ty,
		BlockedBy: t.BlockedBy,
		Children:  t.Children,
		Created:   t.Created.UTC(),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal frontmatter: %w", err)
	}
	var b bytes.Buffer
	b.WriteString(fence)
	b.Write(fm)
	b.WriteString(fence)
	b.WriteString("# " + t.Title + "\n")
	if t.Body != "" {
		b.WriteString("\n" + t.Body)
	}
	return b.Bytes(), nil
}

// normalizeNewlines converts CRLF to LF so files touched by Windows editors or
// git autocrlf still parse. The next save rewrites them with LF.
func normalizeNewlines(data []byte) string {
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// Unmarshal parses the on-disk form produced by Marshal. CRLF line endings are
// accepted. A missing or zero created timestamp is an error.
func Unmarshal(data []byte) (*Ticket, error) { return UnmarshalAt(data, time.Time{}) }

// UnmarshalAt is Unmarshal, except that a missing or zero created timestamp
// is replaced by fallback.
func UnmarshalAt(data []byte, fallback time.Time) (*Ticket, error) {
	s := normalizeNewlines(data)
	rest, ok := strings.CutPrefix(s, fence)
	if !ok {
		return nil, errors.New("missing frontmatter")
	}
	var fmText, content string
	if strings.HasPrefix(rest, fence) {
		content = rest[len(fence):]
	} else {
		i := strings.Index(rest, "\n"+fence)
		if i < 0 {
			return nil, errors.New("unterminated frontmatter")
		}
		fmText = rest[:i+1]
		content = rest[i+1+len(fence):]
	}
	var fm frontmatter
	// Unknown keys are rejected, not dropped: a typo such as "blockd-by" or a
	// v1 field would otherwise vanish on the next save.
	dec := yaml.NewDecoder(strings.NewReader(fmText))
	dec.KnownFields(true)
	if err := dec.Decode(&fm); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	line, body, _ := strings.Cut(content, "\n")
	title, ok := strings.CutPrefix(line, "# ")
	if !ok {
		return nil, errors.New("missing '# Title' heading")
	}
	body = strings.TrimPrefix(body, "\n")
	if fm.Created.IsZero() {
		fm.Created = fallback
	}
	if fm.Type == "" {
		fm.Type = TypeTask
	}
	t := &Ticket{
		ID:        fm.ID,
		Status:    fm.Status,
		Type:      fm.Type,
		BlockedBy: fm.BlockedBy,
		Children:  fm.Children,
		Created:   fm.Created,
		Title:     title,
		Body:      body,
	}
	if err := t.Validate(); err != nil {
		return nil, err
	}
	return t, nil
}
