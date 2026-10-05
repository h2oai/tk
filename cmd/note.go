package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func init() { register(newNoteCmd) }

func newNoteCmd(app *App) *cobra.Command {
	var file string
	c := &cobra.Command{
		Use:   "note <id> [text | -]",
		Short: "Append a timestamped note (text inline, - for stdin, or -F file)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			hasInline := len(args) == 2
			inline := ""
			if hasInline {
				inline = args[1]
			}
			text, err := ReadText(cmd.InOrStdin(), inline, hasInline, file)
			if err != nil {
				return err
			}
			if strings.TrimSpace(text) == "" {
				return errors.New("note text is empty")
			}
			st := app.Store()
			id, err := st.ResolveID(args[0])
			if err != nil {
				return err
			}
			tk, err := st.Load(id)
			if err != nil {
				return err
			}
			stamp := time.Now().UTC().Format(time.RFC3339)
			tk.Body = strings.TrimRight(tk.Body, "\n")
			if tk.Body != "" {
				tk.Body += "\n\n"
			}
			tk.Body += fmt.Sprintf("## Note %s\n\n%s", stamp, text)
			if err := st.Save(tk); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "noted %s\n", id)
			return nil
		},
	}
	c.Flags().StringVarP(&file, "file", "F", "", "read note from file")
	return c
}
