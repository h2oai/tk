package cmd

import (
	"fmt"
	"slices"

	"github.com/h2oai/tk/internal/tree"
	"github.com/spf13/cobra"
)

func init() {
	register(func(a *App) *cobra.Command {
		return &cobra.Command{
			Use:   "dep <id> <blocker>",
			Short: "Make a ticket wait on a blocker",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				t, ids, err := a.LoadResolved(args...)
				if err != nil {
					return err
				}
				before := t.Misorders()
				if err := t.AddDep(ids[0], ids[1]); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s now waits on %s\n", ids[0], ids[1])
				warnMisorders(cmd, t, ids[0], before)
				return nil
			},
		}
	})
	register(func(a *App) *cobra.Command {
		return &cobra.Command{
			Use:   "undep <id> <blocker>",
			Short: "Remove a blocker from a ticket",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				t, err := a.LoadTree()
				if err != nil {
					return err
				}
				id, err := t.Resolve(args[0])
				if err != nil {
					return err
				}
				blocker, err := resolveBlocker(t, id, args[1])
				if err != nil {
					return err
				}
				if err := t.RemoveDep(id, blocker); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s no longer waits on %s\n", id, blocker)
				return nil
			},
		}
	})
}

// resolveBlocker finds which of id's blockers arg names. A blocker that no
// longer exists (dangling) can only be named by its exact id. Anything else
// that does not resolve to a current blocker of id is an error.
func resolveBlocker(t *tree.Tree, id, arg string) (string, error) {
	blockedBy := t.Get(id).BlockedBy
	blocker, err := t.Resolve(arg)
	if err != nil {
		if slices.Contains(blockedBy, arg) {
			return arg, nil
		}
		return "", err
	}
	if !slices.Contains(blockedBy, blocker) {
		return "", fmt.Errorf("%s is not blocked by %s", id, blocker)
	}
	return blocker, nil
}
