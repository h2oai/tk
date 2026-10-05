package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/h2oai/tk/internal/store"
	"github.com/h2oai/tk/internal/tree"
	"github.com/spf13/cobra"
)

func init() { register(newNewCmd) }

func newNewCmd(app *App) *cobra.Command {
	var under, before, after, body, file, typ string
	var at int
	c := &cobra.Command{
		Use:   "new <title>",
		Short: "Create a ticket and print its id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := strings.TrimSpace(args[0])
			if title == "" {
				return errors.New("title must not be empty")
			}
			ty, err := store.ParseType(typ)
			if err != nil {
				return err
			}
			text, err := ReadText(cmd.InOrStdin(), body, cmd.Flags().Changed("body"), file)
			if err != nil {
				return err
			}
			st := app.Store()
			if err := st.Init(); err != nil {
				return err
			}
			t, err := tree.Load(st)
			if err != nil {
				return err
			}
			var parent string
			if under != "" {
				if parent, err = t.Resolve(under); err != nil {
					return err
				}
			}
			place, err := placeFromFlags(t, at, before, after)
			if err != nil {
				return err
			}
			id := store.GenerateID()
			for t.Get(id) != nil {
				id = store.GenerateID()
			}
			tk := &store.Ticket{ID: id, Status: store.StatusOpen, Type: ty, Created: now(), Title: title, Body: text}
			if err := t.Add(tk, parent, place); err != nil {
				return fmt.Errorf("add ticket: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), id)
			return nil
		},
	}
	c.Flags().StringVar(&under, "under", "", "parent ticket (default: top level)")
	c.Flags().IntVar(&at, "at", 0, "1-based position among siblings (default: last)")
	c.Flags().StringVar(&before, "before", "", "place before this sibling")
	c.Flags().StringVar(&after, "after", "", "place after this sibling")
	c.Flags().StringVar(&typ, "type", string(store.TypeTask), "ticket type: "+store.TypeNames)
	c.Flags().StringVarP(&body, "body", "b", "", "body text, or - to read stdin")
	c.Flags().StringVarP(&file, "file", "F", "", "read body from file")
	return c
}
