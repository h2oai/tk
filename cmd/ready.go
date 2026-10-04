package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/h2oai/tk/internal/readytree"
	"github.com/h2oai/tk/internal/ticket"
	"github.com/spf13/cobra"
)

var readyCmd = &cobra.Command{
	Use:   "ready [ticket-id]",
	Short: "List ready tickets",
	Long: `List open/in-progress tickets with all dependencies resolved.

With no arguments, lists every ready ticket. With a ticket id, lists only
ready tickets whose parent is that ticket, which is useful for finding the
next available work inside an epic.

With --tree, ready tickets are drawn as a tree following parent links, with
non-ready ancestors shown as dim context lines. With --watch (implies --tree),
the tree is redrawn every --interval seconds until interrupted. When stdout is
not a terminal, --watch prints the tree once and exits.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runReady,
}

var (
	readySort     string
	readyTree     bool
	readyWatch    bool
	readyInterval int
)

func init() {
	rootCmd.AddCommand(readyCmd)
	readyCmd.Flags().StringVar(&readySort, "sort", "priority", "Sort by field (date|priority|status)")
	readyCmd.Flags().BoolVar(&readyTree, "tree", false, "Show ready tickets as a tree of parent links")
	readyCmd.Flags().BoolVar(&readyWatch, "watch", false, "Redraw the tree every --interval seconds (implies --tree)")
	readyCmd.Flags().IntVarP(&readyInterval, "interval", "n", 5, "Seconds between refreshes with --watch")
}

func runReady(cmd *cobra.Command, args []string) error {
	if readyInterval <= 0 {
		return fmt.Errorf("interval must be positive, got %d", readyInterval)
	}
	if readyWatch {
		readyTree = true
	}

	if !readyWatch || !isTTY(os.Stdout) {
		all, ready, err := loadReady(args)
		if err != nil {
			return err
		}
		if !readyTree {
			for _, t := range ready {
				fmt.Printf("%s - %s\n", formatTicketPrefix(t), t.Title)
			}
			return nil
		}
		return printReadyTree(os.Stdout, all, ready, isTTY(os.Stdout))
	}
	return watchReady(args)
}

// loadReady returns every ticket plus the ready subset (scoped to the children
// of args[0] when given), sorted by --sort.
func loadReady(args []string) (all, ready []*ticket.Ticket, err error) {
	tickets, err := store.List()
	if err != nil {
		return nil, nil, err
	}
	all = tickets

	// Build the status map from all tickets so dependency resolution stays
	// global even when the listing is scoped to one ticket's children.
	statusMap := make(map[string]ticket.Status)
	for _, t := range tickets {
		statusMap[t.ID] = t.Status
	}

	// Scope to children of the given ticket when an id is supplied.
	if len(args) == 1 {
		target, err := store.Get(args[0])
		if err != nil {
			return nil, nil, err
		}

		children := make([]*ticket.Ticket, 0, len(tickets))
		for _, t := range tickets {
			if t.Parent == target.ID {
				children = append(children, t)
			}
		}
		tickets = children
	}

	if err := ticket.SortBy(tickets, readySort); err != nil {
		return nil, nil, err
	}

	// Filter ready tickets (preserves sort order)
	for _, t := range tickets {
		// Must be open or in_progress
		if t.Status != ticket.StatusOpen && t.Status != ticket.StatusInProgress {
			continue
		}

		// All deps must be closed
		allDepsResolved := true
		for _, dep := range t.Deps {
			if statusMap[dep] != ticket.StatusClosed {
				allDepsResolved = false
				break
			}
		}

		if allDepsResolved {
			ready = append(ready, t)
		}
	}
	return all, ready, nil
}

func printReadyTree(w io.Writer, all, ready []*ticket.Ticket, color bool) error {
	if len(ready) == 0 {
		fmt.Fprintln(w, "no ready tickets")
		return nil
	}
	return readytree.Render(w, all, ready, readySort, color && os.Getenv("NO_COLOR") == "")
}

// watchReady redraws the ready tree in the alternate screen until interrupted.
// A failed refresh keeps the last good frame and shows the error in the header.
func watchReady(args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Print("\x1b[?1049h\x1b[?25l")
	defer fmt.Print("\x1b[?25h\x1b[?1049l")

	var frame bytes.Buffer
	ticker := time.NewTicker(time.Duration(readyInterval) * time.Second)
	defer ticker.Stop()
	for {
		var errMsg string
		all, ready, err := loadReady(args)
		if err == nil {
			var next bytes.Buffer
			err = printReadyTree(&next, all, ready, true)
			if err == nil {
				frame = next
			}
		}
		if err != nil {
			errMsg = " · error: " + err.Error()
		}
		fmt.Printf("\x1b[H\x1b[2J%s\n\n%s", watchHeader(errMsg), frame.String())

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func watchHeader(errMsg string) string {
	h := fmt.Sprintf("tk ready · every %ds · %s · Ctrl-C to quit%s",
		readyInterval, time.Now().Format("15:04:05"), errMsg)
	if os.Getenv("NO_COLOR") == "" {
		return "\x1b[1m" + h + "\x1b[0m"
	}
	return h
}

// isTTY reports whether f is a character device (a TTY).
func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
