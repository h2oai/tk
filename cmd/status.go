package cmd

import (
	"fmt"

	"github.com/h2oai/tk/internal/tree"
	"github.com/spf13/cobra"
)

func init() {
	register(func(a *App) *cobra.Command {
		return statusCmd(a, "start", "Set a ticket to in_progress", func(t *tree.Tree, id string, _ bool) ([]string, error) {
			return t.Start(id)
		}, false)
	})
	register(func(a *App) *cobra.Command {
		return statusCmd(a, "close", "Close a ticket (--force also closes descendants)", func(t *tree.Tree, id string, force bool) ([]string, error) {
			return t.Close(id, force)
		}, true)
	})
	register(func(a *App) *cobra.Command {
		return statusCmd(a, "reopen", "Set a ticket back to open", func(t *tree.Tree, id string, _ bool) ([]string, error) {
			return t.Reopen(id)
		}, false)
	})
}

func statusCmd(app *App, name, short string, do func(*tree.Tree, string, bool) ([]string, error), withForce bool) *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   name + " <id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, ids, err := app.LoadResolved(args[0])
			if err != nil {
				return err
			}
			changed, err := do(t, ids[0], force)
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			if len(changed) == 0 {
				fmt.Fprintf(w, "%s unchanged (%s)\n", ids[0], t.Get(ids[0]).Status)
			}
			for _, id := range changed {
				tk := t.Get(id)
				fmt.Fprintf(w, "%s %s %q\n", id, tk.Status, tk.Title)
			}
			return nil
		},
	}
	if withForce {
		c.Flags().BoolVar(&force, "force", false, "also close all descendants")
	}
	return c
}
