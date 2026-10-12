package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/h2oai/tk/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	register(newArchiveCmd)
	register(newUnarchiveCmd)
}

func newArchiveCmd(app *App) *cobra.Command {
	var force, all, dryRun bool
	c := &cobra.Command{
		Use:   "archive <id> | --all",
		Short: "Mark a closed subtree (or, with --all, every closed subtree) as archived",
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
			var changed []*store.Ticket
			if all {
				t, err := app.LoadTree()
				if err != nil {
					return err
				}
				if changed, err = t.ArchiveClosed(dryRun); err != nil {
					return err
				}
			} else {
				t, ids, err := app.LoadResolved(args[0])
				if err != nil {
					return err
				}
				if changed, err = t.Archive(ids[0], force, dryRun); err != nil {
					return err
				}
			}
			printArchived(cmd, changed, "archived", dryRun)
			return nil
		},
	}
	c.Flags().BoolVar(&force, "force", false, "close the whole subtree first")
	c.Flags().BoolVar(&all, "all", false, "archive every subtree whose tickets are all closed")
	c.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "print what would be archived without changing anything")
	return c
}

func newUnarchiveCmd(app *App) *cobra.Command {
	var dryRun bool
	c := &cobra.Command{
		Use:   "unarchive <id>",
		Short: "Clear the archived flag on a subtree and its archived ancestors",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, ids, err := app.LoadResolved(args[0])
			if err != nil {
				return err
			}
			changed, err := t.Unarchive(ids[0], dryRun)
			if err != nil {
				return err
			}
			printArchived(cmd, changed, "unarchived", dryRun)
			return nil
		},
	}
	c.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "print what would be unarchived without changing anything")
	return c
}

// printArchived prints one `<id> <verb> "<title>"` line per ticket, with
// verb "archived" or "unarchived"; a dry run prints `would archive` or
// `would unarchive` instead.
func printArchived(cmd *cobra.Command, tks []*store.Ticket, verb string, dryRun bool) {
	if dryRun {
		verb = "would " + strings.TrimSuffix(verb, "d")
	}
	for _, tk := range tks {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s %q\n", tk.ID, verb, tk.Title)
	}
}
