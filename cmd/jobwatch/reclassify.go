package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/stepbeta/jobwatch/internal/scan"
)

func newReclassifyCmd(opts *rootOptions) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "reclassify",
		Short: "Backfill title/location scores and remote policies with the deterministic classifier",
		Long: `Re-runs the deterministic classifier over stored offers.

Without flags only offers that have no score yet are classified. Use --all
to recompute every offer, for example after tweaking the keyword tables.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			sqlDB, err := opts.open(ctx)
			if err != nil {
				return err
			}
			defer sqlDB.Close()
			n, err := scan.Reclassify(ctx, sqlDB, all)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "reclassified %d offer(s)\n", n)
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "recompute every offer, not only the unclassified ones")
	return cmd
}
