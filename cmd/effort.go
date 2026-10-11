package cmd

import (
	"fmt"

	"github.com/h2oai/tk/internal/store"
	"github.com/spf13/cobra"
)

func init() { register(newEffortCmd) }

func newEffortCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "effort <id> <easy|normal|hard>",
		Short: "Set a ticket's expected effort",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			eff, err := store.ParseEffort(args[1])
			if err != nil {
				return err
			}
			t, ids, err := app.LoadResolved(args[0])
			if err != nil {
				return err
			}
			changed, err := t.SetEffort(ids[0], eff)
			if err != nil {
				return err
			}
			if changed {
				tk := t.Get(ids[0])
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s %q\n", tk.ID, tk.Effort, tk.Title)
			}
			return nil
		},
	}
}
