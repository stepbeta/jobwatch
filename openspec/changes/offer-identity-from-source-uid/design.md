# Design

## Context

See `proposal.md - Why`. Current constraints that shape the approach:

- `job_offers.fingerprint` is `UNIQUE` and composed of the company's normalized
  display name, the normalized title and a location bucket.
- `job_offer_provenance` already stores `external_uid` per `(offer, source, url)`;
  it is populated for most rows but unused for identity.
- `db.ResolveCompany` looks a company up by `name_normalized` and creates it if
  missing; the scan pipeline supplies its input as
  `firstNonEmpty(raw.Company, cfg.Company, src.Name)`.
- Companies own offers and company-level notes; a confirmed merge already moves both
  and rewrites fingerprints.
- The UI reaches a company only through an offer or the dashboard.

## Goals / Non-Goals

**Goals:**

- Identity and company attribution are decided from values the source attests to.
- A confirmed decision (merge, repair) is durable and never re-asked.
- Existing data survives: no offer, event, note or snapshot is lost by the migration.

**Non-Goals:**

- Redesigning the UI beyond what the behavior changes require.
- Adding scheduling, notifications, or new source types.
- Deciding how descriptions are compared; history is kept, comparison is a
  presentation concern.

## Decisions

### Identity is `(source, external_uid)`, with `(source, url)` as fallback

`job_offer_provenance` becomes the identity carrier rather than a side record: a
sighting is first resolved to an existing provenance row, then to its offer. The
`fingerprint` column loses its uniqueness and stops being written as an identity; it
is retained only until the code that reads it is removed.

Alternatives: keep the fingerprint and add the uid as a tiebreaker. Rejected —
two identity sources mean two answers when they disagree.

### Every offer belongs to a company row, with one relaxed invariant

An offer is created only when its company is known. Items without a company are
stored as incomplete offers whose company is absent; the invariant becomes "an
offer has at most one company, and needs one to be usable". This keeps provenance,
snapshots and notes working for incomplete items.

Alternatives: a separate staging table. Rejected — it duplicates half of
`job_offers` and forces the UI to render two kinds of rows; a sentinel "Unresolved"
company. Rejected — it reintroduces the pseudo-company this change removes.

### Company identity is a set of keys plus per-source name claims

```
companies(id, name, website, ...)                 -- name is display, not unique
company_keys(id, company_id, key)                 -- UNIQUE(key); one company may hold several
company_aliases(id, company_id, name_normalized, source_id)
                                                  -- UNIQUE(source_id, name_normalized)
                                                  -- source_id NULL = confirmed for all sources
```

Resolution order: key, then alias for this source, then confirmed alias, then create.
Adapters supply the key they already hold (board token, org, site, account, subdomain,
tenant, company slug). A feed supplies only a name, so its resolution is scoped to that
source.

Alternatives: a single `companies.key` column. Rejected — a company can legitimately
have a board on more than one provider. Global uniqueness of names. Rejected — two
employers may share a name.

### A merge writes the absorbed identities into the absorbing company

The merge moves offers, notes and provenance as it does today, then records the
absorbed company's keys and names on the winner and deletes the emptied row. Later
sightings of those keys or names resolve to the winner, so the merge is learned once.

Alternatives: keep the absorbed row as a redirect. Rejected — it keeps ghost rows
addressable and re-creates the problem in a different shape.

### Incomplete items are offers with a reason

One nullable column records why an offer is incomplete (missing company, missing
title, or both) and keeps the raw reported values. Repairs are written by the user;
scans never update company or title on an existing offer, so a repair cannot be
undone by a refresh.

### `changed` is removed, `snapshots` is kept

The state was a permanent latch computed from a text hash. Listing state becomes
`active | disappeared`; description differences are recorded as snapshots and
compared when the user asks to see them.

## Risks / Trade-offs

- [Provenance becomes a primary key table] -> migration must backfill identity rows
  before `fingerprint` uniqueness is dropped, and the merge/split paths must be
  re-pointed at provenance rather than fingerprint.
- [Some sources have no uid] -> the URL fallback is weaker; the adapter spec requires
  a stable uid wherever the source offers one, and tests fetch twice to prove it.
- [Incomplete offers can accumulate] -> the count is surfaced per scan and in the
  incomplete view, so a source that stops normalizing is visible immediately.
- [Company resolution errors are now durable] -> a wrong merge propagates to later
  scans. Mitigation: merges stay explicit and the split path already exists.
- [Removing `changed`] -> existing rows need a data migration and the UI filter and
  badge must be updated in the same batch.

## Migration Plan

1. Add `company_keys`, `company_aliases`, `incomplete` marker; backfill one alias per
   company from its current `name_normalized`, and keys from each source's existing
   config token where the source's offers map to a single company.
2. Rewrite offer identity resolution to provenance and assert uniqueness on
   `(source_id, external_uid)`; keep the fingerprint column in place, unused.
3. Set `offer_state='active'` for rows in `changed`; drop the value from the enum.
4. Rollback: the new tables and columns are additive; the only destructive step is the
   `changed` update, which is reversible by re-running scans. A `jobwatch backup`
   before migrating is the supported safety net.

## Open Questions

- Whether the fingerprint column is dropped in this change or left unused for a later
  cleanup. Deferrable: it must stop being unique here either way.

## Implementation notes

- **Duplicate provenance rows.** The live database already held 80
  `(source_id, external_uid)` pairs attached to two or three offers, left
  by the old fingerprint identity. A plain unique index cannot be created
  over them. The user chose to squash each group onto the surviving offer
  (active, then changed, then most recently seen): the kept link inherits
  the earliest `first_seen_at` and latest `last_seen_at`, the other links
  are removed, and the duplicate offer rows are left in place without a
  source link. No offer, event, note or snapshot is deleted. This is the
  only place where links move between offers, and it is recorded in the
  0012 header and in `docs/DATA-MODEL.md`.
- **Migration runner.** Rebuilding `job_offers` (nullable `company_id`,
  no fingerprint uniqueness) requires foreign keys to be off, otherwise
  `DROP TABLE` cascades into events, notes and snapshots. The runner now
  pins a connection, disables enforcement, runs the migration, checks
  `PRAGMA foreign_key_check` before committing, and restores enforcement.
- **Company name uniqueness is still enforced.** Migration 0012 adds
  `company_keys` and `company_aliases` but leaves
  `companies.name_normalized UNIQUE` in place; the current resolver still
  matches by name. The company-resolution group must drop that constraint
  in a follow-up migration before two same-named companies can resolve
  independently. The backfilled global aliases are unique, so no conflict
  exists today.
- **Fingerprint identity is still written.** Group 1 removes the unique
  constraint and the `changed` state, but the scan pipeline still
  resolves and writes fingerprints until the offer-identity group lands.
  This is intentional and documented as transitional.
