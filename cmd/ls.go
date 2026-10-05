package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/h2oai/tk/internal/store"
	"github.com/h2oai/tk/internal/tree"
	"github.com/spf13/cobra"
)

func init() { register(newLsCmd) }

func newLsCmd(app *App) *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:   "ls [id]",
		Short: "Show the ticket tree as an outline with positions",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := app.LoadTree()
			if err != nil {
				return err
			}
			roots := t.Roots()
			if len(args) == 1 {
				id, err := t.Resolve(args[0])
				if err != nil {
					return err
				}
				roots = []string{id}
			}
			w := cmd.OutOrStdout()
			for _, id := range roots {
				if t.Get(id) != nil && (all || len(args) == 1 || !allClosed(t, id)) {
					writeOutline(w, t, id, 0, all)
				}
			}
			return nil
		},
	}
	c.Flags().BoolVar(&all, "all", false, "include closed subtrees")
	return c
}

// allClosed reports whether id and all its descendants are closed.
func allClosed(t *tree.Tree, id string) bool {
	for _, s := range t.Subtree(id) {
		if t.Get(s).Status != store.StatusClosed {
			return false
		}
	}
	return true
}

func writeOutline(w io.Writer, t *tree.Tree, id string, depth int, all bool) {
	tk := t.Get(id)
	line := fmt.Sprintf("%s%s %s %s  %s", strings.Repeat("  ", depth), t.Position(id), marker(tk.Status), id, tk.Title)
	if open := openBlockers(t, tk); len(open) > 0 {
		line += "  <- " + strings.Join(open, ", ")
	}
	fmt.Fprintln(w, line)
	for _, c := range t.Children(id) {
		if all || !allClosed(t, c) {
			writeOutline(w, t, c, depth+1, all)
		}
	}
}

// openBlockers lists the ids in tk.BlockedBy that are not closed.
func openBlockers(t *tree.Tree, tk *store.Ticket) []string {
	var out []string
	for _, b := range tk.BlockedBy {
		if bt := t.Get(b); bt != nil && bt.Status != store.StatusClosed {
			out = append(out, b)
		}
	}
	return out
}
