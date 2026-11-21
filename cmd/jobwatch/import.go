package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/stepbeta/jobwatch/internal/importmeta"
	"github.com/stepbeta/jobwatch/internal/scan"
	"github.com/stepbeta/jobwatch/internal/sources"
)

func newImportCmd(opts *rootOptions) *cobra.Command {
	var flags scan.ImportInput
	var noFetch bool
	cmd := &cobra.Command{
		Use:   "import <url | json | file.json | ->",
		Short: "Manually add an offer seen elsewhere (fetches URL metadata when needed)",
		Long: `Manually add an offer. The argument is either:
  - a URL: the page title/company/description are extracted when --company
    and --title are not both given (use --no-fetch to skip that request)
  - a JSON object or array: {"company","title","location","url","description"}
  - a path to a .json file, or "-" to read JSON from stdin

If the URL already belongs to an offer, the import is attached to that
offer's provenance instead of creating a new offer. A new URL creates a
new offer even when the company and title match another one.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			arg := args[0]
			switch {
			case arg == "-":
				b, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("import: read stdin: %w", err)
				}
				arg = string(b)
			case strings.HasSuffix(strings.ToLower(arg), ".json"):
				b, err := os.ReadFile(arg)
				if err != nil {
					return fmt.Errorf("import: %w", err)
				}
				arg = string(b)
			}
			inputs, err := scan.ParseImport(arg)
			if err != nil {
				return err
			}
			if len(inputs) == 0 {
				return errors.New("import: nothing to import")
			}
			if len(inputs) > 1 && (flags != scan.ImportInput{} || noFetch) {
				return errors.New("import: field flags cannot be combined with a JSON array")
			}
			if len(inputs) == 1 {
				inputs[0] = overlay(inputs[0], flags)
				for i := range inputs {
					if noFetch || inputs[i].URL == "" || (inputs[i].Company != "" && inputs[i].Title != "") {
						continue
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "fetching metadata from %s…\n", inputs[i].URL)
					meta, err := importmeta.Extract(ctx, sources.NewClient(), inputs[i].URL)
					if err != nil {
						return fmt.Errorf("import: %w (pass --company/--title or --no-fetch)", err)
					}
					inputs[i].FillMissing(meta.Company, meta.Title, meta.Location, meta.URL, meta.Description)
					if inputs[i].Company == "" || inputs[i].Title == "" {
						return fmt.Errorf("import: could not extract company and title from %s (pass --company/--title or --no-fetch)", inputs[i].URL)
					}
				}
			}

			sqlDB, err := opts.open(ctx)
			if err != nil {
				return err
			}
			defer sqlDB.Close()
			for _, in := range inputs {
				res, err := scan.Import(ctx, sqlDB, in, time.Now())
				if err != nil {
					return err
				}
				verb := "created"
				if !res.Created {
					verb = "matched existing"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s offer %d: %s — %s\n", verb, res.OfferID, in.Company, in.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flags.Company, "company", "", "company name")
	cmd.Flags().StringVar(&flags.Title, "title", "", "job title")
	cmd.Flags().StringVar(&flags.Location, "location", "", "location as advertised")
	cmd.Flags().StringVar(&flags.Description, "description", "", "description text")
	cmd.Flags().BoolVar(&noFetch, "no-fetch", false, "do not fetch a URL for metadata; require --company and --title")
	return cmd
}

// overlay fills in, or overrides, fields of in with non-empty flag values.
func overlay(in, flags scan.ImportInput) scan.ImportInput {
	if flags.Company != "" {
		in.Company = flags.Company
	}
	if flags.Title != "" {
		in.Title = flags.Title
	}
	if flags.Location != "" {
		in.Location = flags.Location
	}
	if flags.Description != "" {
		in.Description = flags.Description
	}
	return in
}
