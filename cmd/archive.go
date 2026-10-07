package cmd

import (
	"errors"
	"fmt"

	"github.com/h2oai/tk/internal/store"
	"github.com/h2oai/tk/internal/tree"
	"github.com/spf13/cobra"
)

func init() { register(newArchiveCmd) }

func newArchiveCmd(app *App) *cobra.Command {
	var force, all, dryRun bool
	c := &cobra.Command{
		Use:   "archive <id> | --all",
		Short: "Move a closed subtree (or, with --all, every closed subtree) into the archive directory",
		Args: func(cmd *cobra.Command, args []string) error {
			if all {
				if force {
					return errors.New("--all and --force cannot be combined")
				}
				return cobra.NoArgs(cmd, args)
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var moved []*store.Ticket
			if all {
				t, err := app.LoadTree()
				if err != nil {
					return err
				}
				var skips []tree.Skip
				moved, skips, err = t.ArchiveClosed(dryRun)
				if err != nil {
					return err
				}
				for _, s := range skips {
					fmt.Fprintf(cmd.ErrOrStderr(), "skipped %s: %s\n", s.ID, s)
				}
			} else {
				t, ids, err := app.LoadResolved(args[0])
				if err != nil {
					return err
				}
				if moved, err = t.Archive(ids[0], force, dryRun); err != nil {
					return err
				}
			}
			verb := "archived"
			if dryRun {
				verb = "would archive"
			}
			for _, tk := range moved {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s %q\n", tk.ID, verb, tk.Title)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&force, "force", false, "close the whole subtree first")
	c.Flags().BoolVar(&all, "all", false, "archive every subtree whose tickets are all closed, skipping those linked to live tickets")
	c.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "print what would be archived without changing anything")
	return c
}
