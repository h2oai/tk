package cmd

import (
	"fmt"
	"os"

	"github.com/h2oai/tk/internal/edit"
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
			editor, err := edit.Editor()
			if err != nil {
				return err
			}
			// The editor can stay open for a long time, so the lock is held only
			// while reading the ticket and again while saving the result.
			unlock, err := app.Lock(false)
			if err != nil {
				return err
			}
			t, ids, err := app.LoadResolved(args[0])
			unlock()
			if err != nil {
				return err
			}
			s, err := edit.Start(t, ids[0], app.Lock)
			if err != nil {
				return err
			}
			defer s.Discard()
			c := s.Command(editor)
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, cmd.OutOrStdout(), cmd.ErrOrStderr()
			if err := c.Run(); err != nil {
				return fmt.Errorf("editor: %w", err)
			}
			changed, err := s.Finish(app.Lock)
			if err != nil {
				return err
			}
			if changed {
				fmt.Fprintf(cmd.OutOrStdout(), "edited %s\n", s.ID)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "unchanged %s\n", s.ID)
			}
			return nil
		},
	}
}
