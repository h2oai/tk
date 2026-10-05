package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func init() { register(newShowCmd) }

func newShowCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:         "show <id>",
		Annotations: map[string]string{lockAnnotation: lockShared},
		Short:       "Show a ticket in full",
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, ids, err := app.LoadResolved(args[0])
			if err != nil {
				return err
			}
			id := ids[0]
			tk := t.Get(id)
			var b strings.Builder
			fmt.Fprintf(&b, "# %s\n", tk.Title)
			fmt.Fprintf(&b, "id:       %s\n", id)
			fmt.Fprintf(&b, "status:   %s\n", tk.Status)
			fmt.Fprintf(&b, "type:     %s\n", tk.Type)
			fmt.Fprintf(&b, "position: %s\n", t.Position(id))
			if p := t.Parent(id); p != "" {
				fmt.Fprintf(&b, "parent:   %s %q\n", p, t.Get(p).Title)
			}
			fmt.Fprintf(&b, "created:  %s\n", tk.Created.Format("2006-01-02T15:04:05Z"))
			if len(tk.BlockedBy) > 0 {
				b.WriteString("blocked-by:\n")
				for _, d := range tk.BlockedBy {
					if dt := t.Get(d); dt != nil {
						fmt.Fprintf(&b, "  %s %q (%s, %s)\n", d, dt.Title, t.Position(d), dt.Status)
					} else {
						fmt.Fprintf(&b, "  %s (missing)\n", d)
					}
				}
			}
			if kids := t.Children(id); len(kids) > 0 {
				b.WriteString("children:\n")
				for _, c := range kids {
					fmt.Fprintf(&b, "  %s %s %s  %s\n", t.Position(c), t.Get(c).Status.Marker(), c, t.Get(c).Title)
				}
			}
			if body := strings.TrimSpace(tk.Body); body != "" {
				fmt.Fprintf(&b, "\n%s\n", body)
			}
			fmt.Fprint(cmd.OutOrStdout(), b.String())
			return nil
		},
	}
}
