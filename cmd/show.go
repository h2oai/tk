package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/h2oai/tk/internal/render"
	"github.com/spf13/cobra"
)

func init() { register(newShowCmd) }

func newShowCmd(app *App) *cobra.Command {
	var pretty bool
	c := &cobra.Command{
		Use:         "show [id]",
		Annotations: map[string]string{lockAnnotation: lockShared},
		Short:       "Show a ticket in full (default: the ticket tk ready picks)",
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := app.LoadTree()
			if err != nil {
				return err
			}
			var id string
			if len(args) == 1 {
				id, err = t.Resolve(args[0])
			} else {
				id, err = readyID(cmd.OutOrStdout(), t, "")
			}
			if err != nil {
				return err
			}
			if pretty {
				out, err := render.Render(t, id, glamour.WithAutoStyle())
				if err != nil {
					return err
				}
				fmt.Fprint(cmd.OutOrStdout(), out)
				return nil
			}
			tk := t.Get(id)
			var b strings.Builder
			fmt.Fprintf(&b, "# %s\n", tk.Title)
			fmt.Fprintf(&b, "id:       %s\n", id)
			fmt.Fprintf(&b, "status:   %s\n", tk.Status)
			fmt.Fprintf(&b, "type:     %s\n", tk.Type)
			fmt.Fprintf(&b, "effort:   %s\n", tk.Effort)
			if tk.Archived {
				b.WriteString("archived: true\n")
			}
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
					fmt.Fprintf(&b, "  %s %s %s  %s\n", t.Position(c), t.Get(c).Marker(), c, t.Get(c).Title)
				}
			}
			if body := strings.TrimSpace(tk.Body); body != "" {
				fmt.Fprintf(&b, "\n%s\n", body)
			}
			fmt.Fprint(cmd.OutOrStdout(), b.String())
			return nil
		},
	}
	c.Flags().BoolVarP(&pretty, "pretty", "p", false, "render as formatted markdown for reading")
	return c
}
