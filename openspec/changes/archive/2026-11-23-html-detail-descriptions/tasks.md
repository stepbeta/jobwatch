# Tasks

## 1. Preparation (before any code or DB change)

- [x] 1.1 Cut the work branch from an up-to-date default branch: `git fetch origin`, `git switch main`, `git pull --ff-only`, `git switch -c feat/html-detail-descriptions` (the repo uses `main`, not `master`); verify `git branch --show-current` names the new branch and that the only untracked path is `openspec/changes/html-detail-descriptions/`.
- [x] 1.2 Check for a running `jobwatch serve` (`pgrep -af "jobwatch serve"`); if one is running, ask the user to stop it and wait for confirmation before touching the database; verify no process holds `jobwatch.db`.
- [x] 1.3 Take the backup with `jobwatch backup jobwatch.db.<timestamp>.bkp` (or `cp` once no process holds it); verify the command reports a valid copy / the integrity check passes.

## 2. HTML adapter detail descriptions

- [x] 2.1 Add `detail_description_selector` to `sources.Config` and compile it with the other selectors, failing the source on an invalid selector; verify `go build ./...` and an invalid-selector test case in `internal/sources`.
- [x] 2.2 Implement the conditional detail fetch in `internal/sources/html.go`: fetch only when the stored text is empty or equals the listing text, emit the stored text on the skip path, use the detail text when present, and fall back to the listing text on a per-item miss.
- [x] 2.3 Add fail-fast on `ErrBlocked`: stop detail requests for the rest of the scan, return the listing offers, and keep the source result `ok`; verify no further detail requests are made after the blocked one.
- [x] 2.4 Add httptest coverage for: detail text replacing a listing summary, stored-text skip, summary-equals-listing upgrade, no-uid behaviour, HTTP 500 fallback, selector miss, blocked detail stopping the loop, and invalid selector; verify `go test ./internal/sources/...`.
- [x] 2.5 Document the key in `docs/USAGE.md` and `docs/ARCHITECTURE.md` (one eligible request per item at the 5 s minimum, stored-text skip, fail-fast); verify the documented selectors match the tests and `go vet ./...` is clean.

## 3. Murena seeded config

- [x] 3.1 Add data-only migration `0016_murena_detail_description.sql` that sets `detail_description_selector: "section.entry-content"` on the Murena source with `json_set`, guarded so it is idempotent; verify a fresh DB contains the key.
- [x] 3.2 Add or extend the migration test in `internal/db` for the Murena config and idempotency; verify `go test ./internal/db/...`.
- [x] 3.3 Update the Murena row in `docs/SOURCE-DECISIONS.md` and the HTML listing-only row in `docs/MILESTONES.md` (known gaps) to describe the detail selector and its one-time upgrade.

## 4. Wazuh local source and verification

- [x] 4.1 Rebuild the binary; restart `serve` on it if that is safe (otherwise ask the user to restart) so the running UI uses the new adapter; register `https://wazuh.com/careers/` as `--type html --name Wazuh` with the open-only card selectors, `uid_attr: data-id`, `company: Wazuh` and `detail_description_selector: .job-content-description`; verify `jobwatch list --sources` shows the row and config.
- [x] 4.2 Scan the Wazuh source; verify it returns only open-position titles, each with a non-empty full description, the source status is `ok`, and a second scan keeps the same offers and descriptions.
- [x] 4.3 Scan Murena; verify the five offers' `raw_text` now contains full-page text (e.g. requirements/offer sections) instead of the summary plus "Read full job description.", a description snapshot exists, and the source status is `ok`.

## 5. Integration and close-out

- [x] 5.1 Add the `CHANGELOG.md` entry under Unreleased (adapter key, Murena upgrade, Wazuh source, fail-fast/skip rules).
- [x] 5.2 Replace the HTML source line in `docs/SPEC.md` §7 with a pointer to `openspec/specs/html-source/spec.md` and record the revision; verify the link resolves.
- [x] 5.3 Run `go build ./...`, `go vet ./...` and `go test ./...`; verify all pass.
- [x] 5.4 Run `openspec validate html-detail-descriptions --strict`; verify the change validates, including the new capability delta.
- [x] 5.5 Commit the batch with a conventional commit message (including the change artifacts); verify the working tree is clean.

## Workflow follow-up

- Archive the change after review and after every task above is complete.
- Keep the Wazuh source row local; do not add a seed migration for it.
