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

func newAddSourceCmd(opts *rootOptions) *cobra.Command {
	var name, typ, cfgJSON string
	var disabled bool
	cmd := &cobra.Command{
		Use:   "add-source <url>",
		Short: "Register a source (type and config are detected from known ATS URLs)",
		Long: `Register a source. Greenhouse, Lever, Ashby, Personio and Workable
board URLs are recognized automatically, e.g.:

  jobwatch add-source https://boards.greenhouse.io/gitlab
  jobwatch add-source https://jobs.lever.co/spotify --name Spotify
  jobwatch add-source https://jobs.ashbyhq.com/linear
  jobwatch add-source https://acme.jobs.personio.de
  jobwatch add-source https://apply.workable.com/acme

RSS/Atom feeds and HTML listing pages need --type plus --config; see the README.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			rawURL := strings.TrimSpace(args[0])
			u, err := url.Parse(rawURL)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return fmt.Errorf("add-source: %q is not an http(s) URL", rawURL)
			}

			detType, detCfg, detected := sources.Detect(rawURL)
			st := model.SourceType(typ)
			if st == "" {
				if !detected {
					return errors.New("add-source: cannot detect source type from URL; pass --type")
				}
				st = detType
			}
			if !st.Valid() {
				return fmt.Errorf("add-source: unknown type %q", st)
			}
			if st == model.SourceManual {
				return errors.New("add-source: manual offers have no source; use `jobwatch import`")
			}

			cfg := map[string]any{}
			if detected && detType == st {
				b, _ := json.Marshal(detCfg)
				_ = json.Unmarshal(b, &cfg)
			}
			if cfgJSON != "" {
				var extra map[string]any
				if err := json.Unmarshal([]byte(cfgJSON), &extra); err != nil {
					return fmt.Errorf("add-source: --config is not a JSON object: %w", err)
				}
				for k, v := range extra {
					cfg[k] = v
				}
			}
			cfgBytes, _ := json.Marshal(cfg)

			if name == "" {
				name = defaultSourceName(detCfg, u)
			}

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

			src, err := db.InsertSource(ctx, sqlDB, model.Source{
				Name: name, URL: rawURL, Type: st, ConfigJSON: string(cfgBytes), Enabled: !disabled,
			}, time.Now())
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added source %d: %s [%s] %s config=%s\n", src.ID, src.Name, src.Type, src.URL, src.ConfigJSON)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "display name (default: board name from the URL); also the default company name")
	cmd.Flags().StringVar(&typ, "type", "", "source type: greenhouse|lever|ashby|personio|workable|rss|html (wttj is not implemented)")
	cmd.Flags().StringVar(&cfgJSON, "config", "", `extra config_json keys, e.g. '{"company":"Acme","rate_limit_ms":5000}'`)
	cmd.Flags().BoolVar(&disabled, "disabled", false, "register the source disabled")
	return cmd
}

func defaultSourceName(c sources.Config, u *url.URL) string {
	for _, v := range []string{c.BoardToken, c.Site, c.Org, c.Tenant, c.Account} {
		if v != "" {
			return v
		}
	}
	return u.Hostname()
}
