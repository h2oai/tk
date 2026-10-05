package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// Exit codes. 0, 1 and 2 keep their meaning for `ready` (found, nothing left,
// blocked); `fsck` also uses 1 for "problems found". Every other failure
// (bad id, missing directory, corrupt tickets, usage errors) exits ExitGeneric.
const (
	ExitFsck    = 1
	ExitGeneric = 3
)

// DefaultDir is the tickets directory used when --dir is not given.
const DefaultDir = ".tickets"

// App carries state shared by all commands for one invocation.
type App struct {
	Dir string // tickets directory (--dir)
}

// builders holds one constructor per subcommand. Each command file registers
// itself from init(), so adding a command never requires editing root.go.
var builders []func(*App) *cobra.Command

func register(b func(*App) *cobra.Command) { builders = append(builders, b) }

// newRootCmd builds a fresh command tree (no state shared between calls, which
// keeps in-process tests independent).
func newRootCmd() *cobra.Command {
	app := &App{}
	root := &cobra.Command{
		Use:           "tk",
		Short:         "Minimal ordered-tree ticket tracker",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&app.Dir, "dir", DefaultDir, "tickets directory")
	for _, b := range builders {
		root.AddCommand(b(app))
	}
	return root
}

// Execute runs the root command and exits the process with the right code.
func Execute() {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		os.Exit(report(os.Stderr, err))
	}
}

// report prints err to w (unless it is a silent exit) and returns the exit code.
func report(w io.Writer, err error) int {
	var ee *ExitError
	if errors.As(err, &ee) {
		if ee.Msg != "" {
			fmt.Fprintln(w, ee.Msg)
		}
		return ee.Code
	}
	fmt.Fprintln(w, "Error:", err)
	return ExitGeneric
}
