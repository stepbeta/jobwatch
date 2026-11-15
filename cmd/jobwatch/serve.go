package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/stepbeta/jobwatch/internal/api"
)

func newServeCmd(opts *rootOptions) *cobra.Command {
	var port int
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the local web server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			sqlDB, err := opts.open(ctx)
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			mux := http.NewServeMux()
			mux.Handle("/api/", (&api.Server{DB: sqlDB, Version: version}).Handler())
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/" {
					http.NotFound(w, r)
					return
				}
				fmt.Fprintln(w, "JobWatch is running. The web UI is not built yet; see /api/health.")
			})

			srv := &http.Server{
				Addr:              net.JoinHostPort(addr, strconv.Itoa(port)),
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
	return cmd
}
