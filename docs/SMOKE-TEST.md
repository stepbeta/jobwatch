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
3. `GET /api/offers` verifies 3 offers, company name, `none` state and
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

1. **Sources → Add source.** The form opens in a modal. Paste a public
   Greenhouse/Lever/Ashby, Personio or Workable board URL. The type and
   board key are detected; leave the form's defaults otherwise. Save.
2. Click **Refresh** on that row. Wait for the scan to finish (the
   button shows “Scanning…”). A toast reports `N new, M incomplete, D
   disappeared, G ghosted`. The row's status becomes `OK`; failures show `Error`
   or `Blocked` with the reason in the last-result column.
3. **Offers.** New offers appear. Hover the `T`/`L` badges to see the
   classifier's matched terms. Filter by `Title ≥ 80` and `Location ≥
   80` to see the target roles; change the sort and page through. Open an
   offer and set **Application** to `Saved`; the Dashboard **Saved**
   section lists it.
4. On the same offer, set **Application** to `Applied`, leave **Offer** on
   `active`, correct the location if needed, and click **Save**. The offer
   leaves Saved and appears in Applications; `saved_at` is cleared.
5. In the **Timeline** panel add a `CV sent` event; set *Occurred at* to
   yesterday (leave the time empty to exercise date-only backdating); add
   an optional description. In the **Notes** panel add a note and pin it
   to that event. The note card shows a badge with the event type and
   date; click it to scroll to and highlight the event. The event row
   shows a note icon with the count; click it to expand the pinned note
   inline. Use the pencil icon to edit the event (change the type or
   backdate it) and the trash icon to delete it; after deleting a pinned
   event the note keeps its text and loses the badge.
6. Go back to the Dashboard: the offer is in **Applications**, sorted by
   recent activity. Its **Refresh** button scans only the enabled sources
   behind applications. The **Company** link opens the company page
   (notes above offers, possible duplicate companies).
7. **Ghosting.** Use a throwaway source you control (or a local test
   board): set one of its offers to **Application → Applied**, remove that
   posting from the source, and rescan. The toast reports `1 disappeared,
   1 ghosted`; the offer is `negative`, and its timeline shows one
   **Ghosted** event. Scan again: the ghosted count stays `0` and no
   second event appears.
8. **Sources** → edit the source and disable it. The header “Refresh all”
   now skips it. Re-enable it before deleting if you want to test
   deletion; the confirm dialog explains that offers and links are kept.
9. Import: open **Offers → Import an offer seen elsewhere**, paste a
   job URL, click **Fetch & import**. If metadata is incomplete the form
   is prefilled and asks for the missing company/title. Alternatively
   fill company+title yourself (no page fetch).
10. `Ctrl-C` stops the server. Run
   `/tmp/jobwatch-smoke --db /tmp/jobwatch-smoke.db backup /tmp/jobwatch-smoke-backup.db`;
   the command prints “integrity check ok”.

### Expected crawl behaviour

- Requests honor `robots.txt`; a disallowed path marks the source
  `blocked`, never crashes the scan.
- Requests to one host are spaced by `rate_limit_ms` (default 2 s) or the
  robots `Crawl-delay`, whichever is larger. HTML sources never poll
  faster than every 5 s and fetch the configured listing page; when
  `detail_description_selector` is set, an offer page is fetched only for
  an offer with no stored description or whose stored text still equals
  the listing summary, and a blocked detail page stops the remaining
  detail requests for that scan. A
  `wttj` source fetches one offer page per hit whose index `profile` and
  `summary` are both blank and which has no stored description, through
  the same rate limit, to read its JSON-LD description. Those detail
  requests use the adapter's single browser User-Agent (WTTJ's WAF
  challenges the JobWatch UA); a challenged page leaves the description
  empty.
- A failing source never blocks the others. Failed scans change no offer.

### Seeded rows

A fresh database seeds every source decided in Milestones 5a/5b/6a/10a:
39 enabled ATS/RSS/HTML/WTTJ sources (including the Semble BambooHR board
and the Fabriq WTTJ company page) and 28 disabled not-trackable rows with
a `note`, plus the four disabled known-hard rows from 0002 (Datadog/Cisco
Workday, SAP SuccessFactors, WTTJ marketplace). The known-hard rows are
present only as reminders: no adapter reads them (the `wttj` adapter only
reads `/companies/` pages). Do not enable them expecting a scan; use
manual import or an RSS/HTML source instead.
Deleting `jobwatch.db` and reopening it recreates the same 71 rows
without re-running `add-sources`. User-added sources are not part of the
seed: the live instance also tracks Partoo (WTTJ, id 82) and Joko (Ashby,
id 88, added when Joko's WTTJ source was retired).
