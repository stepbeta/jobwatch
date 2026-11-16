// Command jobwatch is the JobWatch CLI and local server.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/stepbeta/jobwatch/internal/db"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

type rootOptions struct {
	dbPath string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	opts := &rootOptions{}
	root := &cobra.Command{
		Use:           "jobwatch",
		Short:         "Local job offer tracker",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&opts.dbPath, "db", "./jobwatch.db", "path to the SQLite database file")
	root.AddCommand(
		newServeCmd(opts),
		newScanCmd(opts),
		newAddSourceCmd(opts),
		newAddSourcesCmd(opts),
		newListCmd(opts),
		newImportCmd(opts),
		newBackupCmd(opts),
		newReclassifyCmd(opts),
	)
	return root
}

func (o *rootOptions) open(ctx context.Context) (*sql.DB, error) {
	return db.Open(ctx, o.dbPath)
}
