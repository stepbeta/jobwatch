# Smoke test — documented flows

The primary flow is: **add a source → scan → classify → track an
application**. Two variants are provided: an automated one (no network)
and a manual UI walk-through for a live source.

## Automated (no network)

```sh
go test ./internal/api -run TestSmokeFlows -v
```

`internal/api/e2e_test.go` starts a local `httptest` server that serves a
recorded Workable widget response (`internal/sources/testdata/workable_jobs.json`)
and an allow-all `robots.txt`. It then drives the real HTTP API:

1. `POST /api/sources` with the Workable URL and config.
2. `POST /api/sources/:id/scan`; the real Workable adapter parses the
   fixture, the scan pipeline creates the company/offers/provenance, and
   the deterministic classifier fills both scores.
3. `GET /api/offers` verifies 3 offers, company name, `todo` state and
   non-null scores.
4. `PATCH /api/offers/:id` sets `application_state = applied`.
5. `POST /api/offers/:id/events` adds a backdated `cv_sent` event.
6. `GET /api/offers/:id` verifies the timeline, provenance/source name,
   snapshot, and `GET /api/offers?has_application=true&sort=recent_event`
   verifies the Applications view.

Adapter parsing and error paths are separately covered by
`go test ./internal/sources` and `go test ./internal/importmeta`.

## Manual UI walk-through (live source)

Use a throwaway database so the smoke test does not mix with real data.
The `ui` build needs the SPA bundle first (`web/dist` is a build
artifact and is gitignored):

```sh
(cd web && npm run build)
go build -tags ui -o /tmp/jobwatch-smoke ./cmd/jobwatch
/tmp/jobwatch-smoke --db /tmp/jobwatch-smoke.db serve
# open http://127.0.0.1:8080
```

1. **Sources → Add source.** Paste a public Greenhouse/Lever/Ashby,
   Personio or Workable board URL. The type and board key are detected;
   leave the form's defaults otherwise. Save.
2. Click **Refresh** on that row. Wait for the scan to finish (the
   button shows “Scanning…”). A toast reports `N new, M changed, D
   disappeared`. The row's status becomes `OK`; failures show `Error`
   or `Blocked` with the reason in the last-result column.
3. **Dashboard.** New offers appear. Hover the `T`/`L` badges to see the
   classifier's matched terms. Filter by `Title ≥ 80` and `Location ≥
   80` to see the target roles; change the sort and page through.
4. Open an offer. Set **Application** to `Applied`, leave **Offer** on
   `active`, correct the location if needed, and click **Save**.
5. In the **Timeline** panel add a `CV sent` event; set *Occurred at* to
   yesterday to exercise backdating; add an optional description. In the
   **Notes** panel add a note and pin it to that event.
6. Go back to the Dashboard: the offer is in **Applications**, sorted by
   recent activity. The **Company** link opens the company page (offers,
   notes, possible duplicate companies).
7. **Sources** → edit the source and disable it. “Refresh all” now skips
   it. Re-enable it before deleting if you want to test deletion; the
   confirm dialog explains that offers and links are kept.
8. Import: open **Dashboard → Import an offer seen elsewhere**, paste a
   job URL, click **Fetch & import**. If metadata is incomplete the form
   is prefilled and asks for the missing company/title. Alternatively
   fill company+title yourself (no page fetch).
9. `Ctrl-C` stops the server. Run
   `/tmp/jobwatch-smoke --db /tmp/jobwatch-smoke.db backup /tmp/jobwatch-smoke-backup.db`;
   the command prints “integrity check ok”.

### Expected crawl behaviour

- Requests honor `robots.txt`; a disallowed path marks the source
  `blocked`, never crashes the scan.
- Requests to one host are spaced by `rate_limit_ms` (default 2 s) or the
  robots `Crawl-delay`, whichever is larger. HTML sources never poll
  faster than every 5 s. Only the configured listing page is fetched.
- A failing source never blocks the others. Failed scans change no offer.

### Seeded rows

A fresh database seeds every source decided in Milestones 5a/5b: 37
enabled ATS/RSS/HTML sources and 28 disabled not-trackable rows with a
`note`, plus the four disabled known-hard rows from 0002
(Datadog/Cisco Workday, SAP SuccessFactors, WTTJ). The known-hard rows
are present only as reminders: no adapter reads them. Do not enable
them expecting a scan; use manual import or an RSS/HTML source instead.
Deleting `jobwatch.db` and reopening it recreates the same 69 rows
without re-running `add-sources`.
