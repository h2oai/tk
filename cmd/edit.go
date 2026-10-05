package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/h2oai/tk/internal/store"
	"github.com/spf13/cobra"
)

func init() { register(newEditCmd) }

func newEditCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:         "edit <id>",
		Annotations: map[string]string{lockAnnotation: lockNone},
		Short:       "Edit a ticket in $EDITOR",
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			editor := os.Getenv("EDITOR")
			if editor == "" {
				return fmt.Errorf("$EDITOR is not set")
			}
			// The editor can stay open for a long time, so the lock is held only
			// while reading the ticket and again while saving the result.
			unlock, err := app.Lock(false)
			if err != nil {
				return err
			}
			t, ids, err := app.LoadResolved(args[0])
			if err != nil {
				unlock()
				return err
			}
			id := ids[0]
			path := filepath.Join(app.Dir, id+".md")
			orig, err := os.ReadFile(path)
			unlock()
			if err != nil {
				return err
			}
			tmp, err := os.CreateTemp("", "tk-"+id+"-*.md")
			if err != nil {
				return err
			}
			defer os.Remove(tmp.Name())
			_, werr := tmp.Write(orig)
			if cerr := tmp.Close(); werr == nil {
				werr = cerr
			}
			if werr != nil {
				return werr
			}
			c := exec.Command("sh", "-c", editor+` "$1"`, "sh", tmp.Name())
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, cmd.OutOrStdout(), cmd.ErrOrStderr()
			if err := c.Run(); err != nil {
				return fmt.Errorf("editor: %w", err)
			}
			data, err := os.ReadFile(tmp.Name())
			if err != nil {
				return err
			}
			// Validate before touching the real file, so a bad edit is rejected.
			// Deleting the created line keeps the ticket's existing timestamp.
			tk, err := store.UnmarshalAt(data, t.Get(id).Created)
			if err != nil {
				return fmt.Errorf("edited ticket is invalid, nothing saved: %w", err)
			}
			if tk.ID != id {
				return fmt.Errorf("edited ticket is invalid, nothing saved: id changed to %q", tk.ID)
			}
			if err := tk.Validate(); err != nil {
				return fmt.Errorf("edited ticket is invalid, nothing saved: %w", err)
			}
			unlock, err = app.Lock(true)
			if err != nil {
				return err
			}
			defer unlock()
			// Reload and make sure nobody changed the ticket while the editor
			// was open; saving over their change would silently lose it.
			if err := t.Reload(); err != nil {
				return err
			}
			if cur, err := os.ReadFile(path); err != nil || !bytes.Equal(cur, orig) {
				return fmt.Errorf("%s changed while editing, nothing saved", id)
			}
			if err := t.Replace(tk); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "edited %s\n", id)
			return nil
		},
	}
}
