package cmd

import (
	"fmt"
	"io"

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
			w := cmd.OutOrStdout()
			id, err := readyID(w, t, scope)
			if err != nil {
				return err
			}
			fmt.Fprintf(w, "%s %s %s\n", id, t.Position(id), titleWithType(t.Get(id)))
			return nil
		},
	}
}

// readyID returns the highest ready leaf under scope. When there is none it
// returns ready's exit error, printing the blocker to w first if it is blocked.
func readyID(w io.Writer, t *tree.Tree, scope string) (string, error) {
	res, err := t.Ready(scope)
	if err != nil {
		return "", err
	}
	switch res.Outcome {
	case tree.Found:
		return res.Ticket.ID, nil
	case tree.Blocked:
		fmt.Fprintf(w, "blocked: %s\n", res.Block.Describe(res.Ticket))
		return "", &ExitError{Code: 2}
	}
	return "", &ExitError{Code: 1, Msg: "nothing left to do"}
}
