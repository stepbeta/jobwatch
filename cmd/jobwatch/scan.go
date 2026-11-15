package main

import (
	"errors"
	"fmt"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/scan"
	"github.com/stepbeta/jobwatch/internal/sources"
)

func newScanCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "scan [all|<source-id>]",
		Short: "Scan all enabled sources, or one source by ID",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			target := "all"
			if len(args) == 1 {
				target = args[0]
			}
			sqlDB, err := opts.open(ctx)
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			sc := &scan.Scanner{DB: sqlDB, Fetcher: sources.NewRegistry(sources.NewClient()), Now: time.Now}
			var results []scan.Result
			if target == "all" {
				if results, err = sc.ScanAll(ctx); err != nil {
					return err
				}
			} else {
				id, perr := strconv.ParseInt(target, 10, 64)
				if perr != nil {
					return fmt.Errorf("scan: argument must be \"all\" or a numeric source ID, got %q", target)
				}
				src, err := db.GetSource(ctx, sqlDB, id)
				if errors.Is(err, db.ErrNotFound) {
					return fmt.Errorf("scan: no source with ID %d", id)
				}
				if err != nil {
					return err
				}
				results = []scan.Result{sc.ScanSource(ctx, src)}
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tSOURCE\tSTATUS\tFETCHED\tNEW\tSEEN\tCHANGED\tREAPPEARED\tDISAPPEARED\tERROR")
			failed := 0
			for _, r := range results {
				status := string(r.Status)
				if status == "" {
					status = "skipped"
				}
				msg := ""
				if r.Err != nil {
					msg = r.Err.Error()
					if r.Status != "" {
						failed++
					}
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%s\n",
					r.SourceID, r.SourceName, status, r.Fetched, r.New, r.Seen, r.Changed, r.Reappeared, r.Disappeared, msg)
			}
			tw.Flush()
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no enabled sources; add one with `jobwatch add-source <url>`")
			}
			if failed > 0 {
				return fmt.Errorf("%d source(s) failed", failed)
			}
			return nil
		},
	}
}
