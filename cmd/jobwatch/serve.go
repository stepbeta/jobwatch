package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/stepbeta/jobwatch/internal/api"
	"github.com/stepbeta/jobwatch/web"
)

func newServeCmd(opts *rootOptions) *cobra.Command {
	var port int
	var addr string
	var unsafeBind bool
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the local web server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			warning, err := bindPolicy(addr, unsafeBind)
			if err != nil {
				return err
			}
			if warning != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), warning)
			}

			ctx := cmd.Context()
			sqlDB, err := opts.open(ctx)
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			mux := http.NewServeMux()
			mux.Handle("/api/", (&api.Server{DB: sqlDB, Version: version, BindAddr: addr}).Handler())
			mux.Handle("/", web.Handler())

			srv := &http.Server{
				Addr:              listenAddr(addr, port),
				Handler:           mux,
				ReadHeaderTimeout: 10 * time.Second,
			}
			errc := make(chan error, 1)
			go func() { errc <- srv.ListenAndServe() }()
			fmt.Fprintf(cmd.OutOrStdout(), "JobWatch listening on http://%s (db %s)\n", srv.Addr, opts.dbPath)

			select {
			case err := <-errc:
				return fmt.Errorf("serve: %w", err)
			case <-ctx.Done():
			}
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("shutdown: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&port, "port", 8080, "TCP port to listen on")
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1", "address to bind (localhost only by default)")
	cmd.Flags().BoolVar(&unsafeBind, "unsafe-bind", false, "allow binding a non-loopback address (the API has no authentication)")
	return cmd
}

// listenAddr joins a bind address with the port. Bracketed IPv6 input
// such as [::1] is accepted on the command line and normalized so that
// net.JoinHostPort does not produce [[::1]]:port.
func listenAddr(addr string, port int) string {
	return net.JoinHostPort(strings.Trim(strings.TrimSpace(addr), "[]"), strconv.Itoa(port))
}

// bindPolicy validates --addr before the database is opened. It returns a
// warning to print when the operator explicitly allows a non-loopback bind.
func bindPolicy(addr string, unsafeBind bool) (string, error) {
	if isLoopbackBindAddr(addr) {
		return "", nil
	}
	if !unsafeBind {
		return "", fmt.Errorf("refusing to bind %q: the API has no authentication; pass --unsafe-bind to expose it", addr)
	}
	return fmt.Sprintf("warning: binding %q exposes the API to the network; it has no authentication", addr), nil
}

// isLoopbackBindAddr reports whether addr names loopback: localhost,
// 127.0.0.0/8 or ::1. An empty or wildcard address is not loopback.
func isLoopbackBindAddr(addr string) bool {
	addr = strings.TrimSpace(addr)
	if strings.EqualFold(addr, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(addr, "[]"))
	return ip != nil && ip.IsLoopback()
}
