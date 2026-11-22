package main

import (
	"fmt"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/stepbeta/jobwatch/internal/classify"
	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/internal/sources"
)

func newListCmd(opts *rootOptions) *cobra.Command {
	var offerState, appState string
	var limit int
	var listSources bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List offers and their states (or sources with --sources)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if offerState != "" && !model.OfferState(offerState).Valid() {
				return fmt.Errorf("list: invalid --offer-state %q", offerState)
			}
			if appState != "" && !model.ApplicationState(appState).Valid() {
				return fmt.Errorf("list: invalid --app-state %q", appState)
			}
			sqlDB, err := opts.open(ctx)
			if err != nil {
				return err
			}
			defer sqlDB.Close()
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			defer tw.Flush()

			if listSources {
				srcs, err := db.ListSources(ctx, sqlDB, false)
				if err != nil {
					return err
				}
				fmt.Fprintln(tw, "ID\tNAME\tTYPE\tENABLED\tLAST SCAN\tSTATUS\tURL\tNOTE")
				for _, s := range srcs {
					last, status := "-", "-"
					if s.LastScanAt != nil {
						last = model.FormatTime(*s.LastScanAt)
					}
					if s.LastScanStatus != nil {
						status = string(*s.LastScanStatus)
					}
					cfg, _ := sources.ParseConfig(s)
					fmt.Fprintf(tw, "%d\t%s\t%s\t%v\t%s\t%s\t%s\t%s\n", s.ID, s.Name, s.Type, s.Enabled, last, status, s.URL, truncate(cfg.Note, 60))
				}
				return nil
			}

			profile, err := db.DefaultSearchProfile(ctx, sqlDB)
			if err != nil {
				return err
			}
			offers, err := db.ListOffers(ctx, sqlDB, db.OfferFilter{
				OfferState:       model.OfferState(offerState),
				ApplicationState: model.ApplicationState(appState),
				Limit:            limit,
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(tw, "ID\tCOMPANY\tTITLE\tLOCATION\tREMOTE\tOK\tTITLE AFF\tLOC AFF\tOFFER\tAPPLICATION\tLAST SEEN")
			for _, o := range offers {
				loc := ""
				if o.Location != nil {
					loc = *o.Location
				}
				desc := ""
				if o.RawText != nil {
					desc = *o.RawText
				}
				res := classify.Score(o.Title, loc, desc, profile)
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%d\t%d\t%s\t%s\t%s\n", o.ID, truncate(o.CompanyName, 22),
					truncate(o.Title, 44), truncate(loc, 24), o.RemotePolicy, accepted(profile.Accepts(o.RemotePolicy)),
					res.TitleAffinity.Score, res.LocationAffinity.Score,
					o.OfferState, o.ApplicationState, model.FormatTime(o.LastSeenAt))
			}
			if len(offers) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no offers; add a source with `jobwatch add-source <url>` or import one")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&offerState, "offer-state", "", "filter: active|disappeared")
	cmd.Flags().StringVar(&appState, "app-state", "", "filter: todo|applied|responded|interview|offer|negative|maybe|positive")
	cmd.Flags().IntVar(&limit, "limit", 100, "maximum rows (0 = no limit)")
	cmd.Flags().BoolVar(&listSources, "sources", false, "list sources instead of offers")
	return cmd
}

func accepted(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}
