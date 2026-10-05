package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/h2oai/tk/internal/store"
	"github.com/h2oai/tk/internal/tree"
)

// ExitError makes a command finish with a specific exit code. An empty Msg
// means the command already printed everything it wanted to.
type ExitError struct {
	Code int
	Msg  string
}

func (e *ExitError) Error() string { return e.Msg }

// Store returns the store for the --dir directory.
func (a *App) Store() *store.Store { return store.New(a.Dir) }

// LoadTree loads the tree, with a friendly error if the directory is missing.
func (a *App) LoadTree() (*tree.Tree, error) {
	if _, err := os.Stat(a.Dir); errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("no tickets directory %q (create a ticket with `tk new` first)", a.Dir)
	}
	return tree.Load(a.Store())
}

// ResolveID resolves a full or partial id against the loaded tree.
func ResolveID(t *tree.Tree, partial string) (string, error) { return t.Resolve(partial) }

// LoadResolved loads the tree and resolves each partial id, in order.
func (a *App) LoadResolved(partials ...string) (*tree.Tree, []string, error) {
	t, err := a.LoadTree()
	if err != nil {
		return nil, nil, err
	}
	ids := make([]string, len(partials))
	for i, p := range partials {
		if ids[i], err = t.Resolve(p); err != nil {
			return nil, nil, err
		}
	}
	return t, ids, nil
}

// now returns the current UTC time truncated to seconds.
func now() time.Time { return time.Now().UTC().Truncate(time.Second) }

// marker is the status glyph used by ls and show.
func marker(s store.Status) string {
	switch s {
	case store.StatusClosed:
		return "[x]"
	case store.StatusInProgress:
		return "[~]"
	}
	return "[ ]"
}
