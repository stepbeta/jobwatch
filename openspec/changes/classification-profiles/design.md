# Design: classification-profiles

## Context

See `proposal.md` for motivation. The current implementation stores title and location scores on `job_offers` at ingestion. The classifier in `internal/classify` owns the term tables, the remote policy detector, the score caps and the reasons. The scan, manual import, repair and split paths all write scores. The API reads the stored columns and the SQL query layer filters and sorts on them. The `reclassify` command backfills scores after keyword changes. Manual edits to an offer's location or description do not refresh scores, which is a known gap.

The change creates two capabilities: `classification` (facts, preferences and read-time scoring) and `search-profiles` (profile storage, weights, validation and seed). The existing `docs/SPEC.md` classification section becomes a pointer when the change is archived. The existing offer lifecycle, source normalization, incomplete listings, company resolution and WTTJ specs keep their requirements.

Constraints: single static binary, SQLite in WAL mode, additive migrations, no new dependencies, localhost only, deterministic scoring in Go. The API and CLI are the only scoring surfaces; the SPA displays scores and does not reimplement the classifier.

## Goals / Non-Goals

### Goals

- Store offer facts independently of user preferences.
- Persist search profiles with title, location and description terms and remote policy selection.
- Compute title and location affinity at read time from the current profile.
- Treat remote policy as a fact and remote acceptance as a checkbox preference.
- Seed the default profile from the current hardcoded tables in an additive migration.
- Expose profile read/update and offer scoring through the API and SPA.
- Remove the `reclassify` command and the stored-score write paths.
- Keep the schema ready for multiple profiles while the initial UI edits one default profile.

### Non-Goals

- Multiple-profile creation, switching and comparison in the SPA. The API and schema support multiple profiles; the initial UI edits the default profile.
- Editing an offer's title or location as a user preference. The change edits profile criteria, not offer facts. Existing location edits and incomplete-offer title repair remain as they are.
- User-editable remote detection phrases. The detector stays fixed.
- Caching or materializing affinity scores. Read-time computation is the chosen model.
- Dropping the legacy score columns. They stay unused and deprecated; a future product can drop them in a from-scratch migration.
- Changing the source adapters or the title/location extraction rules.

## Decisions

### 1. Facts, preferences and scores are separate

**Facts** are stored on the offer: title, location, description and remote policy. The remote policy is derived from the offer text at ingestion and whenever location or description changes. The title and location are stored as reported by the source. The description is stored as the raw text.

**Preferences** are stored in search profiles: title terms, location terms, optional description terms, term weights, bucket choices, and accepted remote policies. A profile is a user object, not an offer attribute.

**Scores** are computed at read time: title affinity, location affinity and remote policy acceptance. Title and location affinity are integers clamped to 0–100. Remote policy acceptance is a boolean. No affinity score is stored on the offer.

The same description text has two independent uses: the fixed remote detector runs at ingestion and derives the offer's remote policy; the user's description terms run at read time and contribute to location affinity. The user's description terms never change the remote policy.

### 2. Three signals, not three weighted scores

The UI and API expose three signals:

- **Title affinity** — weighted term score from the title.
- **Location affinity** — weighted term score from the location and description.
- **Remote policy** — the offer's derived policy plus whether the current profile accepts it.

Remote is not a weighted score. The profile selects acceptable policies with checkboxes. The API returns `remote_policy` (the fact) and `remote_policy_accepted` (the membership result). The UI shows a policy badge, not a remote score. This avoids the current entanglement where remote preference points and policy caps are folded into the location score.

### 3. Title and location affinity are additive term scores

Title affinity uses the offer title and the profile's title terms. Location affinity uses the offer location and description and the profile's location and description terms. Matching is whole-token phrase matching after the existing normalization rules. Each matching term adds its stored weight. The total is clamped to 0–100. Neutral terms have weight 0 and appear only in the reasons. The response returns the matched term, weight and bucket for each affinity.

The old policy points and caps (`remoteLocationWeight`, `remoteDescriptionWeight`, `hybridLocationWeight`, `onsitePolicyCap`, `hybridPolicyCap`, `unknownPolicyCap`) are removed from the location score. The remote policy is a separate signal. The seed does not carry those points or caps.

### 4. Remote policy detection remains fixed

The remote/hybrid/onsite phrase lists remain in Go. The detector produces `remote`, `hybrid`, `onsite` or `unknown` from the location and description. The detection runs at ingestion and on location/description edits. The user cannot edit the phrase lists. The profile only selects which of the four policies are acceptable. The default seed accepts `remote` and `unknown`; `hybrid` and `onsite` are not accepted. The stored value for full remote is `remote`; the UI labels it "Full remote".

### 5. Profiles are stored in three additive tables

- `search_profiles`: `id`, `name`, `title_label`, `location_label`, `bucket_scale`, `is_default`, `created_at`, `updated_at`.
- `search_profile_terms`: `id`, `profile_id`, `affinity` (`title`, `location`, `description`), `bucket`, `term`, `weight`, `sort_order`.
- `search_profile_remote`: `profile_id`, `policy`, `accepted`.

A partial unique index enforces at most one default profile. Term uniqueness is `(profile_id, affinity, term)`; the term is trimmed and normalized for matching but stored as entered. Foreign keys cascade on profile deletion. The schema supports multiple profiles; the initial SPA edits the default profile only.

### 6. The migration seeds the current hardcoded decisions

The new migration `0015_search_profiles.sql` is additive. It creates the three tables and seeds one default profile named "Default" with title label "Platform Engineer / SRE", location label "France / full remote", and the five-bucket scale. It inserts every title, location and description term from the current tables with its exact weight. The title terms map to the five buckets: strong positive (+60), mild positive (+25, with custom +20/+15 for the weaker positives), neutral (0), mild negative (-20), strong negative (-120). The location and description terms use the location scale: strong positive (+35), mild positive (+20, with custom +25/+15/+10), neutral (0), mild negative (-20/-15), strong negative (-35). The seed preserves the exact weights; the bucket is a display label. It inserts remote rows accepting `remote` and `unknown` and rejecting `hybrid` and `onsite`.

The seed is the only place the old hardcoded term decisions live. The Go term tables are removed. The migration test asserts the seed term counts and weights. A future product can replace the seed with a from-scratch migration.

### 7. The classifier package becomes a pure scoring library

`internal/classify` keeps the fixed remote detector and gains scoring functions that accept a profile value. It no longer owns the title/location/description term tables. The package has no I/O. The types live in `internal/model` so `internal/db` can return a profile without importing the classifier. The API and CLI load a profile and call the classifier. The scan calls only the remote detector. The classifier tests use explicit profile fixtures and the seeded profile.

### 8. The scan and write paths store facts only

New sightings, manual imports, repairs, splits and merges store title, location, description and the derived remote policy. They no longer call the classifier for title/location scores and no longer write `title_confidence`, `location_confidence` or `match_reasons_json`. The existing columns remain in the table but are not selected or written by the new code. The `SetOfferClassification` helper, `ListOffersForClassification`, `Reclassify` and the `reclassify` CLI command are removed. The `reattribute` command and merge/split behavior keep their existing identity semantics.

### 9. The API computes affinity on read

Profile endpoints:

- `GET /api/search-profiles` lists profiles.
- `GET /api/search-profiles/{id}` reads one profile.
- `PUT /api/search-profiles/{id}` replaces a profile's name, labels, bucket scale, terms and remote selection in one transaction.

Offer endpoints:

- `GET /api/offers` accepts `profile_id` and computes title and location affinity for each returned offer. It accepts `min_title_affinity`, `min_location_affinity` and `remote_accepted`; the existing `remote_policy` filter remains a fact filter. Sorts include `title_affinity` and `location_affinity`.
- `GET /api/offers/{id}` computes the same signals and reasons for one offer.
- Offer responses expose `title_affinity`, `location_affinity`, `remote_policy`, `remote_policy_accepted` and `match_reasons` with title/location term reasons. They no longer expose `title_confidence`, `location_confidence` or the old `match_reasons` shape.

The offers query keeps non-score filters in SQL. When score filters or score sorts are requested, the API loads the filtered facts, computes affinity in Go, applies the score filter and sort, then paginates. The total is computed after score filtering. The profile is loaded once per request. The description is normalized once per offer. This is acceptable for a personal database; a future cache can restore SQL-level score filtering if needed.

### 10. The SPA edits one profile and displays three signals

The SPA adds a Search profile page with:

- Profile name.
- Title label and term lists.
- Location label and term lists.
- Advanced description term lists.
- A 3-bucket or 5-bucket selector.
- Optional numeric weight per term.
- Remote policy checkboxes.

The title, location and description sections carry the concise explanatory text agreed in the proposal. The offers list replaces the title/location score filters with title/location affinity filters and adds an accepted-remote filter. The offer table shows title affinity, location affinity and a remote policy badge. The offer detail shows the facts, the three signals and the term reasons. The initial UI edits the default profile; a profile selector is deferred.

### 11. Alternatives considered

- **Keep scores cached at ingestion and recompute on profile save.** This preserves SQL filters and sorts but still stores derived data and requires invalidation on every write path. It also conflicts with the user's request for read-time computation. Rejected.
- **Compute scores in the SPA.** This would duplicate the deterministic engine in TypeScript, break determinism and split tests. Rejected. The API computes; the SPA displays.
- **Store remote as a weighted score.** This preserves the current single location score shape but hides the difference between "the offer says hybrid" and "the user dislikes hybrid". Rejected. Remote is a fact plus a checkbox preference.
- **Keep the hardcoded term tables and load overrides from a config file.** This avoids a migration but does not give the user a profile editor and still requires a restart. Rejected.
- **Drop the old score columns in the same migration.** This is destructive and makes rollback harder. Rejected. The columns stay unused and deprecated.
- **One table with JSON term blobs.** Simpler schema, but weaker validation and query support. Rejected in favor of relational term rows.

## Risks / Trade-offs

- **Read-time scoring cost on long descriptions** → Load the profile once per request, normalize each description once, and keep the list's non-score filters narrow. Every read does work because no score is cached: that is the intended trade-off, and it removes the stale-score gap. For a personal database the dataset is small. If it grows, add a cache or a SQLite UDF rather than changing the API.
- **Score filters and sorts cannot run in SQL** → Apply non-score filters in SQL, compute affinity in Go, then filter, sort and paginate. Document the total-count semantics. A future cache can restore SQL-level filtering.
- **Seed drift from the hardcoded tables** → Generate the seed from the current tables and add a migration test that asserts term counts and weights. Use the seeded profile in classifier tests for the current examples. The seed is the only source after the change.
- **Breaking API and CLI changes** → Update the SPA, CLI, docs and tests in the same change. The user is the only consumer. List the removed fields and command in `docs/API.md`, `README.md` and `CHANGELOG.md`.
- **Profile edits can create invalid data** → Validate names, terms, weights and policies; reject empty terms and non-numeric weights; replace the profile in one transaction and keep it unchanged on failure.
- **Description terms are easy to misunderstand** → Keep them in an advanced section with concise text that states they affect location affinity only and not remote detection. The seeded description terms are weaker and positive-only; the user can edit them without changing remote detection.
- **Unknown remote policy default** → Accept `unknown` by default and explain it in the UI. The user can uncheck it for strict full-remote searches.
- **Multiple profiles are only partially exposed** → Ship the schema and API with multiple-profile support; the initial UI edits the default profile. Document the deferred selector.
- **Legacy columns remain** → Mark them deprecated in `docs/DATA-MODEL.md`, stop selecting and writing them, and keep them out of API responses. A future destructive migration can remove them.
- **Remote detection is fixed and can misread negation** → The detector remains the existing deterministic phrase matcher; the change does not add negation handling. The user preference is the policy selection, not the detector.
- **No new dependencies** → Use the existing Go, SQLite and React stack. No new packages.

## Migration Plan

1. **Branch.** Fetch `main`, switch to it, update it fast-forward, then create a new branch for the change. Do not start code or migration work on a stale or dirty `main`.
2. **Backup.** Ask the user to confirm the `jobwatch` binary is not running. Then run `jobwatch backup jobwatch.db.<timestamp>.bkp`. The command refuses to overwrite and runs an integrity check. Do not apply the migration until the backup exists and passes.
3. **Apply.** Add `0015_search_profiles.sql`, run the normal migration path at startup or scan, and verify the seed: one default profile, the expected term counts and weights, and the expected remote rows. The migration runner records the version in `schema_migrations`.
4. **Verify.** Run `go build ./...`, `go vet ./...`, `go test ./...`, the SPA tests, and the migration seed test. Exercise the profile read/update endpoints, the offers list with affinity filters and sorts, and the offer detail reasons.
5. **Rollback.** Stop the binary. Restore the pre-migration backup. Deploy the previous binary. The new tables are additive and can remain; the previous binary ignores them. The old score columns remain in both versions. Re-running the migration on a restored database is safe because the migration is additive and recorded once.

The change never drops the old score columns, never deletes offers and never rewrites offer facts during migration. The only data writes are the new profile tables and the seed.

## Open Questions

None. The remaining choices are recorded as decisions: five-bucket default with separate title and location/description scales, description terms in an advanced section, remote policy as checkboxes, `unknown` accepted by default, and the initial UI editing the default profile.
