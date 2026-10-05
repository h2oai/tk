// Package store reads and writes tickets as markdown files with YAML
// frontmatter, and resolves partial ticket IDs.
package store

import (
	"fmt"
	"strings"
	"time"
)

// Status is the lifecycle state of a ticket.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusClosed     Status = "closed"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusOpen, StatusInProgress, StatusClosed:
		return true
	}
	return false
}

// ParseStatus converts a string to a Status, rejecting unknown values.
func ParseStatus(s string) (Status, error) {
	st := Status(s)
	if !st.Valid() {
		return "", fmt.Errorf("invalid status %q (want open, in_progress or closed)", s)
	}
	return st, nil
}

// Type is the kind of work a ticket describes. It is pure metadata.
type Type string

const (
	TypeTask    Type = "task"
	TypeBug     Type = "bug"
	TypeFeature Type = "feature"
	TypeChore   Type = "chore"
)

// TypeNames lists the valid types, for messages.
const TypeNames = "task, bug, feature, chore"

// Valid reports whether t is a known type.
func (t Type) Valid() bool {
	switch t {
	case TypeTask, TypeBug, TypeFeature, TypeChore:
		return true
	}
	return false
}

// ParseType converts a string to a Type, case-insensitively, rejecting
// unknown values.
func ParseType(s string) (Type, error) {
	ty := Type(strings.ToLower(strings.TrimSpace(s)))
	if !ty.Valid() {
		return "", fmt.Errorf("invalid type %q (want %s)", s, TypeNames)
	}
	return ty, nil
}

// Ticket is a unit of work. Hierarchy is stored only as the ordered Children
// list; the parent is derived by scanning.
type Ticket struct {
	ID        string
	Status    Status
	Type      Type
	BlockedBy []string
	Children  []string
	Created   time.Time
	Title     string
	Body      string
}

// Validate checks the invariants required to serialize a ticket.
func (t *Ticket) Validate() error {
	if !validID(t.ID) {
		return fmt.Errorf("invalid ticket id %q", t.ID)
	}
	if !t.Status.Valid() {
		return fmt.Errorf("ticket %s: invalid status %q", t.ID, t.Status)
	}
	if !t.Type.Valid() {
		return fmt.Errorf("ticket %s: invalid type %q (want %s)", t.ID, t.Type, TypeNames)
	}
	if t.Created.IsZero() {
		return fmt.Errorf("ticket %s: missing created timestamp", t.ID)
	}
	if strings.TrimSpace(t.Title) == "" {
		return fmt.Errorf("ticket %s: title must not be empty", t.ID)
	}
	if strings.ContainsAny(t.Title, "\r\n") {
		return fmt.Errorf("ticket %s: title must be a single line", t.ID)
	}
	return nil
}

// validID reports whether id is safe to use as a file name.
func validID(id string) bool {
	if id == "" || id == rootName {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}
