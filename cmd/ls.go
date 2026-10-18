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
	var all, archived bool
	c := &cobra.Command{
		Use:         "ls [id]",
		Annotations: map[string]string{lockAnnotation: lockShared},
		Short:       "Show the ticket tree as an outline with positions",
		Args:        cobra.MaximumNArgs(1),
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
			f := tree.Filter{All: all || archived, Archived: archived}
			if len(args) == 1 && !f.Archived && t.IsArchived(roots[0]) {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s is archived; use --archived to show it\n", roots[0])
			}
			w := cmd.OutOrStdout()
			for _, id := range roots {
				if t.Get(id) != nil && (len(args) == 1 && !t.IsArchived(id) || f.Shows(t, id)) {
					writeOutline(w, t, id, 0, f)
				}
			}
			if n := len(t.Orphans()); n > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "%d unreachable ticket(s); run `tk fsck`\n", n)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&all, "all", false, "include closed subtrees")
	c.Flags().BoolVar(&archived, "archived", false, "include archived tickets (implies --all)")
	return c
}

func writeOutline(w io.Writer, t *tree.Tree, id string, depth int, f tree.Filter) {
	tk := t.Get(id)
	line := fmt.Sprintf("%s%s %s %s  %s", strings.Repeat("  ", depth), t.Position(id), tk.Marker(), id, titleWithType(tk))
	if open := openBlockers(t, tk); len(open) > 0 {
		line += "  <- " + strings.Join(open, ", ")
	}
	fmt.Fprintln(w, line)
	for _, c := range t.Children(id) {
		if f.Shows(t, c) {
			writeOutline(w, t, c, depth+1, f)
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
