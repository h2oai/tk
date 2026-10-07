// Package edit edits a ticket's file in $EDITOR. The editor can stay open for
// a long time, so the store lock is held only while reading the ticket (Start)
// and again while saving the result (Finish), and Finish refuses to save over
// a change made by someone else in between.
package edit

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/h2oai/tk/internal/store"
	"github.com/h2oai/tk/internal/tree"
)

// Locker takes the store lock (exclusive or shared) and returns its release
// function. A nil Locker skips locking.
type Locker func(exclusive bool) (unlock func(), err error)

// Session is one edit: a copy of the ticket's file in a temp file for the
// editor.
type Session struct {
	ID   string
	Path string // temp file the editor opens

	t       *tree.Tree
	orig    []byte
	created time.Time
}

// Editor returns $EDITOR, or an error if it is not set.
func Editor() (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		return "", errors.New("$EDITOR is not set")
	}
	return editor, nil
}

func locked(lock Locker, exclusive bool, fn func() error) error {
	if lock != nil {
		unlock, err := lock(exclusive)
		if err != nil {
			return err
		}
		defer unlock()
	}
	return fn()
}

// Start reloads t under a shared lock and copies id's file to a temp file.
func Start(t *tree.Tree, id string, lock Locker) (*Session, error) {
	s := &Session{ID: id, t: t}
	err := locked(lock, false, func() error {
		if err := t.Reload(); err != nil {
			return err
		}
		tk := t.Get(id)
		if tk == nil {
			return fmt.Errorf("%s no longer exists", id)
		}
		s.created = tk.Created
		var err error
		s.orig, err = t.Raw(id)
		return err
	})
	if err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp("", "tk-"+id+"-*.md")
	if err != nil {
		return nil, err
	}
	s.Path = tmp.Name()
	_, werr := tmp.Write(s.orig)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		s.Discard()
		return nil, werr
	}
	return s, nil
}

// Command returns the command that runs editor on the temp file. editor is
// run by the shell, so it may carry arguments ("code --wait").
func (s *Session) Command(editor string) *exec.Cmd {
	return exec.Command("sh", "-c", editor+` "$1"`, "sh", s.Path)
}

// Discard removes the temp file. It is safe to call more than once.
func (s *Session) Discard() { os.Remove(s.Path) }

// Finish validates the edited temp file and saves it under an exclusive
// lock, then removes the temp file. It reports whether anything was written:
// a file saved unchanged writes nothing. A bad edit, or a ticket changed on
// disk while the editor was open, saves nothing.
func (s *Session) Finish(lock Locker) (bool, error) {
	defer s.Discard()
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return false, err
	}
	if bytes.Equal(data, s.orig) {
		return false, nil
	}
	// Validate before touching the real file, so a bad edit is rejected.
	// Deleting the created line keeps the ticket's existing timestamp.
	tk, err := store.UnmarshalAt(data, s.created)
	if err != nil {
		return false, fmt.Errorf("edited ticket is invalid, nothing saved: %w", err)
	}
	if tk.ID != s.ID {
		return false, fmt.Errorf("edited ticket is invalid, nothing saved: id changed to %q", tk.ID)
	}
	if err := tk.Validate(); err != nil {
		return false, fmt.Errorf("edited ticket is invalid, nothing saved: %w", err)
	}
	err = locked(lock, true, func() error {
		// Reload and make sure nobody changed the ticket while the editor
		// was open; saving over their change would silently lose it.
		if err := s.t.Reload(); err != nil {
			return err
		}
		if cur, err := s.t.Raw(s.ID); err != nil || !bytes.Equal(cur, s.orig) {
			return fmt.Errorf("%s changed while editing, nothing saved", s.ID)
		}
		// Replace rolls back on failure, so a refused edit leaves the
		// ticket as it was.
		if err := s.t.Replace(tk); err != nil {
			return fmt.Errorf("%w, nothing saved", err)
		}
		return nil
	})
	return err == nil, err
}
