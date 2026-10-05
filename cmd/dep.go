package cmd

import (
	"errors"
	"fmt"

	"github.com/h2oai/tk/internal/tree"
	"github.com/spf13/cobra"
)

func init() {
	register(func(a *App) *cobra.Command {
		return depCmd(a, "dep <id> <blocker>", "Make a ticket wait on a blocker", "now waits on", (*tree.Tree).AddDep)
	})
	register(func(a *App) *cobra.Command {
		return depCmd(a, "undep <id> <blocker>", "Remove a blocker from a ticket", "no longer waits on", (*tree.Tree).RemoveDep)
	})
}

func depErr(err error) error {
	switch {
	case errors.Is(err, tree.ErrSelfDep):
		return fmt.Errorf("a ticket cannot wait on itself: %w", err)
	case errors.Is(err, tree.ErrRelatedDep):
		return fmt.Errorf("a ticket cannot wait on its own ancestor or descendant: %w", err)
	case errors.Is(err, tree.ErrDepCycle):
		return fmt.Errorf("that dependency would create a cycle: %w", err)
	}
	return err
}

func depCmd(app *App, use, short, verb string, do func(*tree.Tree, string, string) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := app.LoadTree()
			if err != nil {
				return err
			}
			id, err := t.Resolve(args[0])
			if err != nil {
				return err
			}
			blocker, err := t.Resolve(args[1])
			if err != nil && verb == "now waits on" {
				return err
			}
			if err != nil {
				// undep may name a dangling blocker that no longer resolves.
				blocker = args[1]
			}
			if err := do(t, id, blocker); err != nil {
				return depErr(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", id, verb, blocker)
			return nil
		},
	}
}
