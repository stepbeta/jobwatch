# Tasks

## 1. Schema and migration

- [x] 1.0 Check no `jobwatch` process holds the database (`pgrep -af jobwatch`); if one is running, ask the user to stop it, then back up with `jobwatch backup jobwatch.db.<timestamp>.bkp`
- [x] 1.1 Add migration `00NN_company_identity.sql`: `company_keys`, `company_aliases`, `job_offers.incomplete_reason`, `job_offers.incomplete_raw`
- [x] 1.2 Backfill one alias per existing company from `name_normalized`; backfill keys from source config tokens where a source's offers map to one company
- [x] 1.3 Assert uniqueness on `job_offer_provenance(source_id, external_uid)`; stop writing `job_offers.fingerprint` as identity and drop its unique index
- [x] 1.4 Update `changed` rows to `active`, then remove the value from the enum
- [x] 1.5 Table-driven migration tests: repeated runs, existing data preserved, rollback notes in `docs/DATA-MODEL.md`

## 2. Offer identity

- [ ] 2.1 Resolve a sighting through provenance `(source, uid)` first, `(source, url)` second; create an offer only on a miss
- [ ] 2.2 Update an existing offer's title and location when the source reports new values, without creating a row
- [ ] 2.3 Keep `MarkDisappeared` semantics: an offer disappears only when no successfully scanned source still lists it
- [ ] 2.4 Replace `OutcomeChanged`/`Result.Changed` with snapshot-only recording; keep `snapshots` writes unchanged
- [ ] 2.5 Tests: title edit, same-title siblings, closed-then-reopened, company rename, description edit

## 3. Company resolution

- [ ] 3.1 Replace `db.ResolveCompany` with key-then-alias resolution (`key`, source alias, confirmed alias, create)
- [ ] 3.2 Remove `src.Name` from the sighting fallback; require adapter evidence or `config.company`
- [ ] 3.3 Extend merge to record the absorbed company's keys and aliases on the absorbing company before deleting it
- [ ] 3.4 Tests: identifier seen twice, board reporting a new display name, duplicate names, merge reuse on the next scan, merge leaves no addressable ghost

## 4. Adapter normalization

- [ ] 4.1 Extend `RawOffer` with company evidence (identifier and name) and a per-item incomplete reason
- [ ] 4.2 Each typed adapter emits its existing token as the company identifier
- [ ] 4.3 RSS and HTML gain a declarative employer rule that rewrites company and title together
- [ ] 4.4 Set `config.company` on the enabled single-employer rss/html sources; configure extraction for We Work Remotely
- [ ] 4.5 Fixture tests: scan each adapter twice and assert identical uids; WWR fixture yields 169 distinct employers

## 5. Incomplete items

- [ ] 5.1 Store items with missing company or title as incomplete offers with the raw reported values
- [ ] 5.2 Exclude incomplete offers from list, dashboard and search queries
- [ ] 5.3 Add repair: assign company and title, clear the incomplete marker, never overwritten by later scans
- [ ] 5.4 Report the incomplete count per source in the scan result

## 6. API and UI

- [ ] 6.1 `GET /api/offers` excludes incomplete by default; add a filter to list them
- [ ] 6.2 Add the incomplete view with count and repair form; reuse the manual import form
- [ ] 6.3 Remove `changed` from the offer state type, filter and badge; show a version count instead
- [ ] 6.4 Show the company key(s) on the company page so same-named companies are distinguishable

## 7. Documentation

- [ ] 7.1 `docs/DATA-MODEL.md`: company keys and aliases, offer identity, incomplete marker, state machine without `changed`
- [ ] 7.2 `docs/ARCHITECTURE.md`: adapter contract, resolution order, incomplete path
- [ ] 7.3 `docs/API.md`: incomplete filter and repair endpoint, offer state enum
- [ ] 7.4 `docs/SPEC.md` revision entry; `CHANGELOG.md`; `docs/MILESTONES.md` known-gaps rows that this change closes
