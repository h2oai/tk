package cmd

import (
	"errors"
	"fmt"

	"github.com/h2oai/tk/internal/tree"
	"github.com/spf13/cobra"
)

func init() {
	register(newMvCmd)
	for _, d := range []struct {
		name, short string
		do          func(*tree.Tree, string) error
	}{
		{"up", "Move a ticket one place earlier among its siblings", (*tree.Tree).Up},
		{"down", "Move a ticket one place later among its siblings", (*tree.Tree).Down},
		{"top", "Move a ticket to the first place among its siblings", (*tree.Tree).Top},
		{"bottom", "Move a ticket to the last place among its siblings", (*tree.Tree).Bottom},
	} {
		register(func(a *App) *cobra.Command { return newShiftCmd(a, d.name, d.short, d.do) })
	}
}

func printPlace(cmd *cobra.Command, t *tree.Tree, id string) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", id, t.Position(id), t.Get(id).Title)
}

// warnMisorders prints, on stderr, the blocker misorders a change to id created.
func warnMisorders(cmd *cobra.Command, t *tree.Tree, id string, before []tree.Misorder) {
	for _, m := range t.MisordersSince(before, id) {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", m)
	}
}

func newMvCmd(app *App) *cobra.Command {
	var under, before, after string
	var at int
	var root bool
	c := &cobra.Command{
		Use:   "mv <id>",
		Short: "Reparent and/or reposition a ticket",
		Long: `Reparent and/or reposition a ticket.

The new parent is --under P, or the top level with --root. With neither, it is
the parent of the --before/--after anchor, or else the current parent (so
--at alone only reorders). --at is a 1-based position among the new siblings;
without any position the ticket is appended last.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if root && under != "" {
				return errors.New("use only one of --root and --under")
			}
			if !root && under == "" && at == 0 && before == "" && after == "" {
				return errors.New("nothing to do: give --under, --root, --at, --before or --after")
			}
			t, ids, err := app.LoadResolved(args[0])
			if err != nil {
				return err
			}
			id := ids[0]
			place, err := placeFromFlags(t, at, before, after)
			if err != nil {
				return err
			}
			var parent string
			switch {
			case root:
			case under != "":
				if parent, err = t.Resolve(under); err != nil {
					return err
				}
			case place.Before != "":
				parent = t.Parent(place.Before)
			case place.After != "":
				parent = t.Parent(place.After)
			default:
				parent = t.Parent(id)
			}
			before := t.Misorders()
			if err := t.Move(id, parent, place); err != nil {
				return err
			}
			printPlace(cmd, t, id)
			warnMisorders(cmd, t, id, before)
			return nil
		},
	}
	c.Flags().StringVar(&under, "under", "", "new parent ticket")
	c.Flags().BoolVar(&root, "root", false, "move to the top level")
	c.Flags().IntVar(&at, "at", 0, "1-based position among the new siblings (default: last)")
	c.Flags().StringVar(&before, "before", "", "place before this sibling")
	c.Flags().StringVar(&after, "after", "", "place after this sibling")
	return c
}

func newShiftCmd(app *App, name, short string, do func(*tree.Tree, string) error) *cobra.Command {
	return &cobra.Command{
		Use:   name + " <id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, ids, err := app.LoadResolved(args[0])
			if err != nil {
				return err
			}
			before := t.Misorders()
			if err := do(t, ids[0]); err != nil {
				return err
			}
			printPlace(cmd, t, ids[0])
			warnMisorders(cmd, t, ids[0], before)
			return nil
		},
	}
}
