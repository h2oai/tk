package cmd

import (
	"fmt"

	"github.com/h2oai/tk/internal/tree"
	"github.com/spf13/cobra"
)

func init() { register(newReadyCmd) }

func newReadyCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:         "ready [epic]",
		Annotations: map[string]string{lockAnnotation: lockShared},
		Short:       "Print the highest ready leaf (exit 0 found, 1 nothing left, 2 blocked)",
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := app.LoadTree()
			if err != nil {
				return err
			}
			var scope string
			if len(args) == 1 {
				if scope, err = t.Resolve(args[0]); err != nil {
					return err
				}
			}
			res, err := t.Ready(scope)
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			switch res.Outcome {
			case tree.Found:
				tk := res.Ticket
				fmt.Fprintf(w, "%s %s %s\n", tk.ID, t.Position(tk.ID), titleWithType(tk))
				return nil
			case tree.Blocked:
				fmt.Fprintf(w, "blocked: %s\n", res.Block.Describe(res.Ticket))
				return &ExitError{Code: 2}
			}
			return &ExitError{Code: 1, Msg: "nothing left to do"}
		},
	}
}
