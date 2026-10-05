package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() { register(newRmCmd); register(newFsckCmd) }

func newRmCmd(app *App) *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   "rm <id>",
		Short: "Delete a ticket (refuses if it has children or blocks others; --force overrides)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, ids, err := app.LoadResolved(args[0])
			if err != nil {
				return err
			}
			gone, err := t.Remove(ids[0], force)
			if err != nil {
				return err
			}
			for _, id := range gone {
				fmt.Fprintln(cmd.OutOrStdout(), id)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&force, "force", false, "delete the whole subtree and detach it from other tickets' blockers")
	return c
}

func newFsckCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "fsck",
		Short: "Verify the integrity of the tickets directory (exit 1 on problems)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := app.LoadTree()
			if err != nil {
				return err
			}
			problems := t.Fsck()
			w := cmd.OutOrStdout()
			if len(problems) == 0 {
				fmt.Fprintln(w, "ok")
				return nil
			}
			for _, p := range problems {
				fmt.Fprintln(w, p)
			}
			return &ExitError{Code: 1}
		},
	}
}
