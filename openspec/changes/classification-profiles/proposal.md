# Proposal: classification-profiles

## Why

Classification decisions are hardcoded in Go: changing the target role, geography or term weights requires code changes and a reclassification pass. At the same time, title and location scores are stored at ingestion and go stale when the user edits an offer, and the current model mixes user preferences with offer facts. This change separates facts from preferences and computes title and location affinity from the current search profile at read time, so a single user can change the target without touching code. The schema is ready for multiple profiles, while the UI starts with one.

## What Changes

- **Facts vs preferences vs scores.** Store offer facts (title, location, description, remote_policy) and user search profiles (title, location and description terms plus remote policy selection). Compute title and location affinity at read time from the facts and the current profile. Do not store affinity scores.
- **Three affinity signals.** Title affinity and location affinity are weighted term scores. Remote is a policy membership check: the user selects acceptable policies; the offer's policy is a fact derived by the existing hardcoded detector. Remote is not a weighted score.
- **Search profiles.** Add `search_profiles`, `search_profile_terms` and `search_profile_remote` (additive migration). Seed one default profile from the current hardcoded title, location and description tables and the current remote target. The schema supports multiple profiles; the UI starts with one.
- **Weights.** Title and location/description terms use 3 or 5 buckets (neutral, mild, strong) with separate scales for title vs location/description. Advanced users can enter numeric weights. Description terms are an advanced section; the UI explains that they affect geography only and not remote detection.
- **Read-time scoring.** The scan stores facts and derives `remote_policy`; it no longer computes or stores title and location scores. The API computes title and location affinity and returns the matched terms and reasons for the current profile. A profile change takes effect immediately; no `reclassify` command is needed.
- **API and UI.** Add profile read/update endpoints. The offers list and detail return facts, title affinity, location affinity, remote policy and whether the policy is accepted. Score filters and sorts use the computed affinities. The SPA adds a Search profile page and updates the offers list and detail. The remote policy remains a fact filter; the UI shows a policy badge rather than a remote score.
- **Migration and legacy data.** The migration is additive. The legacy `title_confidence`, `location_confidence` and `match_reasons_json` columns remain in `job_offers` but are unused and marked deprecated; no destructive migration. The seeded profile is the only place the old hardcoded decisions live. A future product can replace the seed with a from-scratch migration.
- **BREAKING** API changes: offer responses no longer expose `title_confidence`, `location_confidence` or `match_reasons_json`; they expose `title_affinity`, `location_affinity`, `remote_policy` and `remote_policy_accepted`. Score filters and sorts change names. The `reclassify` CLI command is removed. The CLI `list` output changes.

## Capabilities

### New Capabilities

- `classification`: owns the facts/preferences/scores model, title and location affinity computation, remote policy detection as a fact, remote policy membership as a preference, and read-time score reasons.
- `search-profiles`: owns profile persistence, term weights and buckets, description terms, remote policy selection, the default profile seed, profile validation, and multi-profile readiness.

### Modified Capabilities

None. Existing offer lifecycle, source normalization, incomplete listings, company resolution and WTTJ specs keep their requirements.

## Impact

- **Code:** `internal/classify`, `internal/db`, `internal/scan`, `internal/api`, `cmd/jobwatch`, `web/`.
- **Data:** additive migration; new profile tables; seeded default profile; legacy score columns retained and unused.
- **APIs:** new profile endpoints; changed offers list and detail fields and filters/sorts; changed CLI list output; removed `reclassify`.
- **Docs:** `docs/ARCHITECTURE.md`, `docs/DATA-MODEL.md`, `docs/API.md`, `docs/SPEC.md` (classification section becomes a pointer after archive), `docs/MILESTONES.md`, `README.md`, `CHANGELOG.md`.
- **Tests:** classifier unit tests, profile CRUD and validation tests, scan and ingestion tests, API tests, SPA tests, migration seed tests.
- **Dependencies:** none.
- **Operational:** the implementation MUST start on a new branch cut from an up-to-date `main`, and MUST create a DB backup after the user confirms no `jobwatch` binary is running. The detailed tasks are in `tasks.md`.
