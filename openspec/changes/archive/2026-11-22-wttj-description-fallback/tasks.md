# Tasks

Apply one numbered group at a time and stop for review before starting the next group.

## 1. Branch and database safety

- [x] 1.1 Fetch `origin`, confirm the working tree is clean, and create `wttj-description-fallback` from `origin/main`; verify with `git status --short --branch` and `git log -1 --oneline`.
- [x] 1.2 Confirm no `jobwatch` process holds the database; the current `jobwatch serve` process must be stopped with the user's confirmation. Verify with `pgrep -af '[j]obwatch'` returning nothing.
- [x] 1.3 Back up the database with `jobwatch backup jobwatch.db.<timestamp>.bkp` (or `cp` only when no process is running) and verify the file exists and the command reports a successful integrity check.

## 2. Shared JSON-LD JobPosting helper

- [x] 2.1 Move the JSON-LD `JobPosting` finder (`findJobPosting`, `typeContains`) from `internal/importmeta` into `internal/sources/jsonld.go` as an exported helper that accepts an `*html.Node`; verify with a new unit test covering a direct object, an array, and an `@graph` wrapper.
- [x] 2.2 Refactor `internal/importmeta` to call the shared finder and keep its existing field extraction; verify `go test ./internal/importmeta/...` passes with the existing fixtures.

## 3. WTTJ description fallback

- [x] 3.1 Add a `SiteURL` override to `WTTJ` (default `https://www.welcometothejungle.com`) and carry the resolved base through `wttjSearch`/`mapWTTJHit`; verify all existing WTTJ tests pass unchanged.
- [x] 3.2 Implement the detail fallback: when `profile` and `summary` are both blank, fetch the offer URL through the shared client with the WTTJ browser User-Agent, find the JSON-LD `JobPosting.description`, convert it with `HTMLToText`, and leave the description empty on any failure without failing the source; verify the fallback test passes against an httptest page.
- [x] 3.3 Add a trimmed WTTJ job-page fixture with a JSON-LD `JobPosting` and table/full-fetch tests for: blank fields fetch once, non-blank `profile` fetches never, non-blank `summary` fetches never, missing JSON-LD returns an empty description, detail 404 keeps the source `ok`, and the detail request uses the WTTJ browser User-Agent; verify `go test ./internal/sources/...` and that no test touches the real network.
- [x] 3.4 Update `docs/SOURCE-DECISIONS.md` (WTTJ mapping table, flow diagram, known gaps) and `docs/ARCHITECTURE.md` (adapter flow) to describe the JSON-LD fallback and its per-scan cost; verify the docs no longer claim that no offer page is ever fetched.
- [x] 3.5 Pass the descriptions already stored for a source from the scan pipeline to the adapter and skip the detail fetch for those uids; verify with a WTTJ table case (stored text → 0 requests) and a scan test (the pipeline supplies the map and a text-less sighting preserves the stored text).

## 4. Joko WTTJ → Ashby migration

- [x] 4.1 Record the bookmark to preserve: old offer 2869 (`Platform Software Engineer (senior/staff level)`, `saved_at = 2026-11-22T17:09:35Z`) and its Ashby homologue id `b83d7f0f-a0d0-4dc6-b305-7d9336e4d389` (`https://jobs.ashbyhq.com/joko/b83d7f0f-a0d0-4dc6-b305-7d9336e4d389`); verify the title still matches the live board before deleting anything.
- [x] 4.2 Delete the 46 Joko offers and source 87 in one transaction with `PRAGMA foreign_keys = ON`, scoping offers by `job_offer_provenance.source_id = 87` and deleting offers before the source; verify 0 offers remain behind source 87, source 87 is gone, and company 115 (`Joko`) still exists. This is the explicit user-confirmed one-off exception to the no-delete rule.
- [x] 4.3 Add `https://jobs.ashbyhq.com/joko` as an Ashby source named `Joko` with `{"company":"Joko"}`, insert `company_keys(company_id = 115, key = 'ashby:joko')`, and scan the new source; verify 46 fetched/new offers, 0 incomplete, every offer's `company_id = 115`, and one company with `name_normalized = 'joko'`.
- [x] 4.4 Restore the bookmark on the Ashby homologue by `external_uid = 'b83d7f0f-a0d0-4dc6-b305-7d9336e4d389'` (fallback: exact title), setting `saved_at = '2026-11-22T17:09:35Z'`; verify exactly one Joko offer has that `saved_at` and its title matches. If the job is no longer listed, leave the bookmark for the user instead of guessing.
- [x] 4.5 Update `docs/SOURCE-DECISIONS.md` with the Joko→Ashby decision, `docs/SMOKE-TEST.md` source counts, and `docs/MILESTONES.md` known gaps; verify no stale claim remains that Joko is tracked through WTTJ.

## 5. Partoo backfill and integration checks

- [x] 5.1 Scan Partoo (source 82); verify 15 fetched/seen, every Partoo offer has a non-empty `raw_text`, and the scan status is `ok`.
- [x] 5.2 Verify no duplicate Joko source, company or offers remain, and that the new Ashby offers have non-empty descriptions; verify with read-only SQL queries against `jobwatch.db`.
- [x] 5.3 Run `go build ./...`, `go vet ./...` and `go test ./...`; verify all are green.
- [x] 5.4 Add the `CHANGELOG.md` Unreleased entry (fallback, shared helper, Joko→Ashby migration, bookmark preservation) and commit the branch with a conventional commit message; verify `git status` is clean and do not push.

## Workflow follow-up

- Archive the change after review; sync the `wttj-source` capability and check that `docs/SPEC.md` still points to it.
- Push and merge only when the user asks.
