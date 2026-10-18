package cmd

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/h2oai/tk/internal/web"
	"github.com/spf13/cobra"
)

func init() { register(newServeCmd) }

func newServeCmd(app *App) *cobra.Command {
	var addr string
	c := &cobra.Command{
		Use:         "serve",
		Annotations: map[string]string{lockAnnotation: lockNone},
		Short:       "Serve a read-only web view of the tickets",
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Fail at startup, not on the first request, outside a project.
			if _, err := app.LoadTree(); err != nil {
				return err
			}
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return err
			}
			// The server is long-lived, so it locks per request, not per session.
			h := web.Handler(app.LoadTree, app.Lock)
			if a, ok := ln.Addr().(*net.TCPAddr); ok && a.IP.IsLoopback() {
				h = web.LocalOnly(h)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "serving %s at http://%s/ (Ctrl-C to stop)\n", app.Dir, ln.Addr())
			srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
			return srv.Serve(ln)
		},
	}
	c.Flags().StringVar(&addr, "addr", "127.0.0.1:7777", "address to listen on")
	return c
}
