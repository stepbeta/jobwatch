package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/stepbeta/jobwatch/internal/db"
)

// sourceListLine is one URL parsed from a bulk source file.
type sourceListLine struct {
	Line int
	URL  string
}

// bulkSourceResult records the outcome for one line of the input file.
type bulkSourceResult struct {
	Line   int
	URL    string
	Name   string
	Type   string
	ID     int64
	Reason string
}

func newAddSourcesCmd(opts *rootOptions) *cobra.Command {
	var disabled bool
	cmd := &cobra.Command{
		Use:   "add-sources <file>",
		Short: "Register every recognized source URL listed in a file",
		Long: `Read a file with one URL per line and register each recognized ATS
board. Blank lines and lines starting with '#' are ignored; a ' #'
suffix starts an inline comment. URL fragments (a '#' without preceding
whitespace) are kept.

Each URL goes through the same detection and validation as add-source.
The command never stops at one bad line: it reports every URL as added,
already registered or not detected, and prints the undetected URLs as a
plain list at the end.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			f, err := os.Open(args[0])
			if err != nil {
				return fmt.Errorf("add-sources: %w", err)
			}
			defer f.Close()

			lines, err := parseSourceList(f)
			if err != nil {
				return fmt.Errorf("add-sources: %s: %w", args[0], err)
			}

			sqlDB, err := opts.open(ctx)
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			var added, existing, notDetected []bulkSourceResult
			seen := make(map[string]int, len(lines))
			for _, l := range lines {
				src, detErr := sourceFromURL(l.URL, "", "", "")

				if first, dup := seen[l.URL]; dup {
					reason := fmt.Sprintf("duplicate of line %d", first)
					if detErr != nil {
						reason += " (not detected)"
						notDetected = append(notDetected, bulkSourceResult{Line: l.Line, URL: l.URL, Reason: reason})
					} else {
						existing = append(existing, bulkSourceResult{Line: l.Line, URL: l.URL, Reason: reason})
					}
					continue
				}
				seen[l.URL] = l.Line

				if detErr != nil {
					reason := detErr.Error()
					if errors.Is(detErr, errNotDetected) {
						reason = "no known ATS board URL"
					}
					notDetected = append(notDetected, bulkSourceResult{Line: l.Line, URL: l.URL, Reason: reason})
					continue
				}
				src.Enabled = !disabled

				found, err := db.FindSourceByURL(ctx, sqlDB, l.URL)
				switch {
				case err == nil:
					existing = append(existing, bulkSourceResult{
						Line: l.Line, URL: l.URL, ID: found.ID, Name: found.Name, Reason: "already registered",
					})
					continue
				case !errors.Is(err, db.ErrNotFound):
					return err
				}

				saved, err := db.InsertSource(ctx, sqlDB, src, time.Now())
				if err != nil {
					return err
				}
				added = append(added, bulkSourceResult{
					Line: l.Line, URL: saved.URL, ID: saved.ID, Name: saved.Name, Type: string(saved.Type),
				})
			}

			writeBulkReport(cmd.OutOrStdout(), args[0], len(lines), len(seen), added, existing, notDetected)
			return nil
		},
	}
	cmd.Flags().BoolVar(&disabled, "disabled", false, "register new sources disabled")
	return cmd
}

// parseSourceList reads URL lines, skipping blank lines and full-line
// comments and stripping inline comments introduced by whitespace + '#'.
// The '#' of a URL fragment is not a comment.
func parseSourceList(r io.Reader) ([]sourceListLine, error) {
	var out []sourceListLine
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(stripInlineComment(line))
		if line == "" {
			continue
		}
		out = append(out, sourceListLine{Line: n, URL: line})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// stripInlineComment removes a trailing comment. Only a '#' preceded by
// whitespace starts one, so "https://example.com/jobs#openings" keeps its
// fragment.
func stripInlineComment(line string) string {
	for i := 1; i < len(line); i++ {
		if line[i] == '#' && (line[i-1] == ' ' || line[i-1] == '\t') {
			return line[:i]
		}
	}
	return line
}

func writeBulkReport(w io.Writer, file string, total, unique int, added, existing, notDetected []bulkSourceResult) {
	fmt.Fprintf(w, "read %d URL(s) from %s (%d unique)\n", total, file, unique)
	fmt.Fprintf(w, "added %d, already registered %d, not detected %d\n\n", len(added), len(existing), len(notDetected))

	fmt.Fprintf(w, "ADDED (%d)\n", len(added))
	if len(added) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "  ID\tNAME\tTYPE\tURL")
		for _, r := range added {
			fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\n", r.ID, r.Name, r.Type, r.URL)
		}
		tw.Flush()
	}

	fmt.Fprintf(w, "\nALREADY REGISTERED (%d)\n", len(existing))
	if len(existing) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "  ID\tNAME\tREASON\tURL")
		for _, r := range existing {
			id := "-"
			if r.ID != 0 {
				id = fmt.Sprintf("%d", r.ID)
			}
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", id, r.Name, r.Reason, r.URL)
		}
		tw.Flush()
	}

	fmt.Fprintf(w, "\nNOT DETECTED (%d)\n", len(notDetected))
	if len(notDetected) == 0 {
		fmt.Fprintln(w, "  (none)")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "  LINE\tREASON\tURL")
	for _, r := range notDetected {
		fmt.Fprintf(tw, "  %d\t%s\t%s\n", r.Line, r.Reason, r.URL)
	}
	tw.Flush()

	fmt.Fprintln(w, "\nNOT DETECTED URLs")
	for _, r := range notDetected {
		fmt.Fprintln(w, r.URL)
	}
}
