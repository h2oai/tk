package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() { register(newArchiveCmd) }

func newArchiveCmd(app *App) *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   "archive <id>",
		Short: "Move a closed subtree into the archive directory (--force closes it first)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, ids, err := app.LoadResolved(args[0])
			if err != nil {
				return err
			}
			moved, err := t.Archive(ids[0], force)
			if err != nil {
				return err
			}
			for _, tk := range moved {
				fmt.Fprintf(cmd.OutOrStdout(), "%s archived %q\n", tk.ID, tk.Title)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&force, "force", false, "close the whole subtree first")
	return c
}
