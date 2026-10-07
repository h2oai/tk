package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	rootName   = "ROOT"
	ext        = ".md"
	archiveDir = "archive"
)

// ErrNotFound is returned (wrapped) when a ticket does not exist.
var ErrNotFound = errors.New("ticket not found")

// Store is a directory of ticket files plus ROOT.md.
type Store struct {
	Dir string
}

// New returns a Store rooted at dir. It does not touch the filesystem.
func New(dir string) *Store { return &Store{Dir: dir} }

// Init creates the directory and an empty ROOT.md if they are missing.
func (s *Store) Init() error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", s.Dir, err)
	}
	if _, err := os.Stat(s.rootPath()); errors.Is(err, fs.ErrNotExist) {
		return s.SaveRoots(nil)
	} else if err != nil {
		return fmt.Errorf("stat ROOT.md: %w", err)
	}
	return nil
}

func (s *Store) path(id string) string { return filepath.Join(s.Dir, id+ext) }

func (s *Store) rootPath() string { return filepath.Join(s.Dir, rootName+ext) }

// Load reads the ticket with exactly the given id.
func (s *Store) Load(id string) (*Ticket, error) {
	if !validID(id) {
		return nil, fmt.Errorf("invalid ticket id %q", id)
	}
	data, err := os.ReadFile(s.path(id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s: %w", id, ErrNotFound)
	} else if err != nil {
		return nil, fmt.Errorf("read %s: %w", id, err)
	}
	// A ticket without created (hand-written or generated) gets the file's
	// modification time, which is persisted on its next save.
	var mtime time.Time
	if fi, err := os.Stat(s.path(id)); err == nil {
		mtime = fi.ModTime().UTC().Truncate(time.Second)
	}
	t, err := UnmarshalAt(data, mtime)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", id, err)
	}
	if t.ID != id {
		return nil, fmt.Errorf("parse %s: id in frontmatter is %q", id, t.ID)
	}
	return t, nil
}

// ReadRaw returns the ticket file's bytes exactly as they are on disk.
func (s *Store) ReadRaw(id string) ([]byte, error) {
	if !validID(id) {
		return nil, fmt.Errorf("invalid ticket id %q", id)
	}
	return os.ReadFile(s.path(id))
}

// Save atomically writes the ticket (temp file, then rename).
func (s *Store) Save(t *Ticket) error {
	data, err := Marshal(t)
	if err != nil {
		return err
	}
	if err := writeAtomic(s.path(t.ID), data); err != nil {
		return fmt.Errorf("save %s: %w", t.ID, err)
	}
	return nil
}

// Delete removes the ticket file.
func (s *Store) Delete(id string) error {
	if !validID(id) {
		return fmt.Errorf("invalid ticket id %q", id)
	}
	if err := os.Remove(s.path(id)); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s: %w", id, ErrNotFound)
	} else if err != nil {
		return fmt.Errorf("delete %s: %w", id, err)
	}
	return nil
}

// ArchiveDir is the directory archived tickets are moved into. Nothing reads
// it back.
func (s *Store) ArchiveDir() string { return filepath.Join(s.Dir, archiveDir) }

// Archive moves the ticket file byte for byte into ArchiveDir, creating it if
// needed. If <id>.md is taken there, the file gets the shortest free numeric
// suffix (<id>-2.md, <id>-3.md, ...). It returns the archived file's path.
// Callers hold the store lock, so checking for a free name then renaming is
// not racy.
func (s *Store) Archive(id string) (string, error) {
	if !validID(id) {
		return "", fmt.Errorf("invalid ticket id %q", id)
	}
	if _, err := os.Stat(s.path(id)); errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("%s: %w", id, ErrNotFound)
	} else if err != nil {
		return "", fmt.Errorf("archive %s: %w", id, err)
	}
	dir := s.ArchiveDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	dst := filepath.Join(dir, id+ext)
	for n := 2; ; n++ {
		if _, err := os.Lstat(dst); errors.Is(err, fs.ErrNotExist) {
			break
		} else if err != nil {
			return "", fmt.Errorf("archive %s: %w", id, err)
		}
		dst = filepath.Join(dir, fmt.Sprintf("%s-%d%s", id, n, ext))
	}
	if err := os.Rename(s.path(id), dst); err != nil {
		return "", fmt.Errorf("archive %s: %w", id, err)
	}
	return dst, nil
}

// IDs returns all ticket ids, sorted.
func (s *Store) IDs() ([]string, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", s.Dir, err)
	}
	var ids []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ext) {
			continue
		}
		id := strings.TrimSuffix(name, ext)
		if validID(id) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// ResolveID maps a full or partial id to a full id. An exact match wins;
// otherwise exactly one id must contain the input as a substring.
func ResolveID(ids []string, partial string) (string, error) {
	if partial == "" {
		return "", errors.New("empty ticket id")
	}
	var matches []string
	for _, id := range ids {
		if id == partial {
			return id, nil
		}
		if strings.Contains(id, partial) {
			matches = append(matches, id)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("%q: %w", partial, ErrNotFound)
	case 1:
		return matches[0], nil
	}
	return "", fmt.Errorf("ambiguous id %q matches %s", partial, strings.Join(matches, ", "))
}

// ROOT.md holds the ordered root ticket ids as YAML frontmatter only:
//
//	---
//	roots: [fanir7, lovet2]
//	---
type rootFile struct {
	Roots flowList `yaml:"roots"`
}

// LoadRoots returns the ordered root ids. A missing ROOT.md means no roots.
func (s *Store) LoadRoots() ([]string, error) {
	data, err := os.ReadFile(s.rootPath())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("read ROOT.md: %w", err)
	}
	text, ok := strings.CutPrefix(normalizeNewlines(data), fence)
	if !ok {
		return nil, errors.New("parse ROOT.md: missing frontmatter")
	}
	i := strings.Index(text, fence)
	if i < 0 {
		return nil, errors.New("parse ROOT.md: unterminated frontmatter")
	}
	var rf rootFile
	if err := yaml.Unmarshal([]byte(text[:i]), &rf); err != nil {
		return nil, fmt.Errorf("parse ROOT.md: %w", err)
	}
	if len(rf.Roots) == 0 {
		return nil, nil
	}
	return rf.Roots, nil
}

// SaveRoots atomically writes the ordered root ids.
func (s *Store) SaveRoots(roots []string) error {
	for _, id := range roots {
		if !validID(id) {
			return fmt.Errorf("invalid root id %q", id)
		}
	}
	body, err := yaml.Marshal(rootFile{Roots: roots})
	if err != nil {
		return fmt.Errorf("marshal ROOT.md: %w", err)
	}
	data := append(append([]byte(fence), body...), fence...)
	if err := writeAtomic(s.rootPath(), data); err != nil {
		return fmt.Errorf("save ROOT.md: %w", err)
	}
	return nil
}

// writeAtomic writes data to a temp file in the same directory, then renames
// it over path.
func writeAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp, 0o644)
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}
