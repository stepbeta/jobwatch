package main

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/scan"
	"github.com/stepbeta/jobwatch/internal/sources"
)

func newReattributeCmd(opts *rootOptions) *cobra.Command {
	var all bool
	var sourceID int64
	cmd := &cobra.Command{
		Use:   "reattribute [--all | --source <id>]",
		Short: "Move existing offers to the company and title their adapter now reports",
		Long: `Re-fetches a source and rewrites the company and title of offers matched
by (source, uid) to the adapter's current output. Events, notes, snapshots,
application state and listing state are preserved. The command is
idempotent: a second run with the same source output changes nothing.

Offers without a (source, uid) link, and offers repaired by the user, are
left untouched. A company emptied by the move stays visible for the user
to delete.

Without flags every enabled source is re-attributed; --source targets one
source by ID, including a disabled one.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			sourceSet := cmd.Flags().Changed("source")
			if all && sourceSet {
				return errors.New("reattribute: use either --all or --source, not both")
			}
			sqlDB, err := opts.open(ctx)
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			sc := &scan.Scanner{DB: sqlDB, Fetcher: sources.NewRegistry(sources.NewClient()), Now: time.Now}
			var results []scan.ReattributeResult
			if sourceSet {
				src, err := db.GetSource(ctx, sqlDB, sourceID)
				if errors.Is(err, db.ErrNotFound) {
					return fmt.Errorf("reattribute: no source with ID %d", sourceID)
				}
				if err != nil {
					return err
				}
				results = []scan.ReattributeResult{sc.ReattributeSource(ctx, src)}
			} else if results, err = sc.ReattributeAll(ctx); err != nil {
				return err
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tSOURCE\tSTATUS\tFETCHED\tREATTRIBUTED\tUNCHANGED\tSKIPPED\tEMPTIED\tERROR")
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
				emptied := "-"
				if len(r.EmptiedCompanies) > 0 {
					emptied = strings.Join(r.EmptiedCompanies, ", ")
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t%d\t%d\t%d\t%d\t%s\t%s\n",
					r.SourceID, r.SourceName, status, r.Fetched, r.Reattributed,
					r.Unchanged, r.Skipped, emptied, msg)
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
	cmd.Flags().BoolVar(&all, "all", false, "reattribute every enabled source (the default)")
	cmd.Flags().Int64Var(&sourceID, "source", 0, "reattribute one source by ID, enabled or not")
	return cmd
}
