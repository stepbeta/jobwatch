package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/internal/sources"
)

// errNotDetected is returned by sourceFromURL when the URL is valid but no
// adapter can be inferred from it.
var errNotDetected = errors.New("cannot detect source type from URL")

func newAddSourceCmd(opts *rootOptions) *cobra.Command {
	var name, typ, cfgJSON string
	var disabled bool
	cmd := &cobra.Command{
		Use:   "add-source <url>",
		Short: "Register a source (type and config are detected from known ATS URLs)",
		Long: `Register a source. Greenhouse, Lever, Ashby, Personio, Workable and
BambooHR board URLs are recognized automatically, e.g.:

  jobwatch add-source https://boards.greenhouse.io/gitlab
  jobwatch add-source https://jobs.lever.co/spotify --name Spotify
  jobwatch add-source https://jobs.ashbyhq.com/linear
  jobwatch add-source https://acme.jobs.personio.de
  jobwatch add-source https://apply.workable.com/acme
  jobwatch add-source https://acme.bamboohr.com/careers

RSS/Atom feeds and HTML listing pages need --type plus --config; see the README.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			rawURL := strings.TrimSpace(args[0])

			src, err := sourceFromURL(rawURL, name, typ, cfgJSON)
			if err != nil {
				if errors.Is(err, errNotDetected) {
					return fmt.Errorf("add-source: %w; pass --type", err)
				}
				return fmt.Errorf("add-source: %w", err)
			}
			src.Enabled = !disabled

			sqlDB, err := opts.open(ctx)
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			if existing, err := db.FindSourceByURL(ctx, sqlDB, rawURL); err == nil {
				return fmt.Errorf("add-source: URL already registered as source %d (%s)", existing.ID, existing.Name)
			} else if !errors.Is(err, db.ErrNotFound) {
				return err
			}

			saved, err := db.InsertSource(ctx, sqlDB, src, time.Now())
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added source %d: %s [%s] %s config=%s\n", saved.ID, saved.Name, saved.Type, saved.URL, saved.ConfigJSON)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "display name (default: board name from the URL); also the default company name")
	cmd.Flags().StringVar(&typ, "type", "", "source type: greenhouse|lever|ashby|personio|workable|bamboohr|rss|html (wttj is not implemented)")
	cmd.Flags().StringVar(&cfgJSON, "config", "", `extra config_json keys, e.g. '{"company":"Acme","rate_limit_ms":5000}'`)
	cmd.Flags().BoolVar(&disabled, "disabled", false, "register the source disabled")
	return cmd
}

// sourceFromURL validates a source URL, detects its type and per-type
// config, derives the default name, and returns an enabled source ready
// for insertion. It is shared by `add-source` and `add-sources` so both
// commands accept exactly the same URLs.
func sourceFromURL(rawURL, name, typ, cfgJSON string) (model.Source, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return model.Source{}, fmt.Errorf("%q is not an http(s) URL", rawURL)
	}

	detType, detCfg, detected := sources.Detect(rawURL)
	st := model.SourceType(typ)
	if st == "" {
		if !detected {
			return model.Source{}, errNotDetected
		}
		st = detType
	}
	if !st.Valid() {
		return model.Source{}, fmt.Errorf("unknown type %q", st)
	}
	if st == model.SourceManual {
		return model.Source{}, errors.New("manual offers have no source; use `jobwatch import`")
	}

	cfg := map[string]any{}
	if detected && detType == st {
		b, _ := json.Marshal(detCfg)
		_ = json.Unmarshal(b, &cfg)
	}
	if cfgJSON != "" {
		var extra map[string]any
		if err := json.Unmarshal([]byte(cfgJSON), &extra); err != nil {
			return model.Source{}, fmt.Errorf("--config is not a JSON object: %w", err)
		}
		for k, v := range extra {
			cfg[k] = v
		}
	}
	cfgBytes, _ := json.Marshal(cfg)

	if name == "" {
		name = defaultSourceName(detCfg, u)
	}

	return model.Source{
		Name:       name,
		URL:        rawURL,
		Type:       st,
		ConfigJSON: string(cfgBytes),
		Enabled:    true,
	}, nil
}

func defaultSourceName(c sources.Config, u *url.URL) string {
	for _, v := range []string{c.BoardToken, c.Site, c.Org, c.Tenant, c.Account, c.Subdomain} {
		if v != "" {
			return v
		}
	}
	return u.Hostname()
}
