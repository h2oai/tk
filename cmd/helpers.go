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

// placeFromFlags builds a Place from --at/--before/--after, resolving anchors.
func placeFromFlags(t *tree.Tree, at int, before, after string) (tree.Place, error) {
	place := tree.Place{Index: at}
	var err error
	if before != "" {
		if place.Before, err = t.Resolve(before); err != nil {
			return place, err
		}
	}
	if after != "" {
		if place.After, err = t.Resolve(after); err != nil {
			return place, err
		}
	}
	return place, nil
}

// titleWithType prefixes the title with its [type] tag for ls and ready.
func titleWithType(tk *store.Ticket) string {
	return "[" + string(tk.Type) + "] " + tk.Title
}
