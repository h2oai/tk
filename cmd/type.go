package cmd

import (
	"fmt"

	"github.com/h2oai/tk/internal/store"
	"github.com/spf13/cobra"
)

func init() { register(newTypeCmd) }

func newTypeCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "type <id> <" + "task|bug|feature|chore" + ">",
		Short: "Set a ticket's type",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ty, err := store.ParseType(args[1])
			if err != nil {
				return err
			}
			t, ids, err := app.LoadResolved(args[0])
			if err != nil {
				return err
			}
			changed, err := t.SetType(ids[0], ty)
			if err != nil {
				return err
			}
			if changed {
				tk := t.Get(ids[0])
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s %q\n", tk.ID, tk.Type, tk.Title)
			}
			return nil
		},
	}
}
