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
			t, ids, err := app.LoadResolved(args[0])
			if err != nil {
				return err
			}
			id := ids[0]
			if err := t.AppendNote(id, text, time.Now()); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "noted %s\n", id)
			return nil
		},
	}
	c.Flags().StringVarP(&file, "file", "F", "", "read note from file")
	return c
}
