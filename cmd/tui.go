package cmd

import (
	"github.com/h2oai/tk/internal/tui"
	"github.com/spf13/cobra"
)

func init() { register(newTuiCmd) }

func newTuiCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:         "tui",
		Annotations: map[string]string{lockAnnotation: lockNone},
		Short:       "Reorder tickets interactively, view them and edit them in $EDITOR",
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := app.LoadTree()
			if err != nil {
				return err
			}
			// The TUI is long-lived, so it locks per keypress, not per session.
			return tui.Run(t, app.Lock)
		},
	}
}
