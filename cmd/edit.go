package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/h2oai/tk/internal/store"
	"github.com/spf13/cobra"
)

func init() { register(newEditCmd) }

func newEditCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a ticket in $EDITOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			editor := os.Getenv("EDITOR")
			if editor == "" {
				return fmt.Errorf("$EDITOR is not set")
			}
			st := app.Store()
			id, err := st.ResolveID(args[0])
			if err != nil {
				return err
			}
			path := filepath.Join(st.Dir, id+".md")
			orig, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			tmp, err := os.CreateTemp("", "tk-"+id+"-*.md")
			if err != nil {
				return err
			}
			defer os.Remove(tmp.Name())
			_, werr := tmp.Write(orig)
			if cerr := tmp.Close(); werr == nil {
				werr = cerr
			}
			if werr != nil {
				return werr
			}
			c := exec.Command("sh", "-c", editor+` "$1"`, "sh", tmp.Name())
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, cmd.OutOrStdout(), cmd.ErrOrStderr()
			if err := c.Run(); err != nil {
				return fmt.Errorf("editor: %w", err)
			}
			data, err := os.ReadFile(tmp.Name())
			if err != nil {
				return err
			}
			// Validate before touching the real file, so a bad edit is rejected.
			tk, err := store.Unmarshal(data)
			if err != nil {
				return fmt.Errorf("edited ticket is invalid, nothing saved: %w", err)
			}
			if tk.ID != id {
				return fmt.Errorf("edited ticket is invalid, nothing saved: id changed to %q", tk.ID)
			}
			if err := tk.Validate(); err != nil {
				return fmt.Errorf("edited ticket is invalid, nothing saved: %w", err)
			}
			if err := st.Save(tk); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "edited %s\n", id)
			return nil
		},
	}
}
