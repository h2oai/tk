package cmd

import (
	"github.com/h2oai/tk/internal/tui"
	"github.com/spf13/cobra"
)

func init() { register(newTuiCmd) }

func newTuiCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Reorder tickets interactively (reorder only; no content edits)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := app.LoadTree()
			if err != nil {
				return err
			}
			return tui.Run(t)
		},
	}
}
