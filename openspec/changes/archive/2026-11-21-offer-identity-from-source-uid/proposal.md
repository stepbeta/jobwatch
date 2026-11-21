# Proposal

## Why

An offer's identity is currently a fingerprint over `(company name, title, location)`,
and the company name is filled by the scan pipeline with the source name when an
adapter cannot provide one. Both choices are guesses:

- A feed that lists many employers (We Work Remotely) becomes a company: 243 offers
  from 169 real employers are attributed to the feed itself.
- Any change to the reported company name re-creates every offer under it. When the
  Greenhouse board began reporting "Automattic Careers" instead of "Automattic", all
  seven postings were duplicated and the originals became `disappeared`. The old
  company row remains as a ghost with no active offers.
- Every source already supplies a stable identifier for each posting (`external_uid`
  is populated for 2300 of 2465 provenance rows) and identity never uses it, so a
  title edit of the same posting creates a new offer.

The domain model the application is built around is sound: the offer is the centre,
it belongs to exactly one company, and a source is where it was seen — several
sources may hold the same offer. The code does not enforce that model.

## What Changes

- Offer identity becomes `(source, external_uid)`, falling back to `(source, url)`.
  The fingerprint stops being identity and is removed as a uniqueness rule.
- `offer_state` becomes `active | disappeared`; `changed` is dropped. Description
  history stays in `snapshots` and is compared at presentation time.
- Adapters become the only place where source knowledge lives: they emit a stable
  uid, a normalized title and location, and the company identifier they already hold
  (`board_token`, `org`, `site`, `account`, `subdomain`, `tenant`, company slug).
- The pipeline stops inventing a company. `src.Name` leaves the fallback chain; a
  single-employer source must declare `config.company`.
- Items an adapter cannot normalize are kept, marked incomplete, and repaired in the
  UI instead of being skipped or failing the source.
- Cross-source sameness of a company stays a user-confirmed merge, and the
  confirmation is remembered so later scans resolve the same way without asking.
- **BREAKING**: existing `changed` rows are migrated to `active`; the `changed` value
  disappears from the API enum and the UI filter.

## Capabilities

### New Capabilities

- `offer-lifecycle`: what makes an offer the same offer, and how scans move it
  between `active` and `disappeared` while keeping description history.
- `company-resolution`: how an offer's company is determined from adapter-attested
  identifiers and names, and how a confirmed merge is reused by later scans.
- `source-normalization`: the adapter contract — stable uid, canonical title and
  location, company evidence, and the removal of pipeline-side guessing.
- `incomplete-listings`: retention, marking and repair of items a source could not
  normalize.

### Modified Capabilities

None. The project has no OpenSpec specs yet; durable behavior is currently described
in `docs/SPEC.md`, `docs/ARCHITECTURE.md` and `docs/DATA-MODEL.md`.

## Impact

- `internal/scan` (identity, sighting, disappearance, result vocabulary)
- `internal/sources` (each adapter's company evidence; rss/html extraction config)
- `internal/db` (companies, provenance, fingerprint column, merge)
- `internal/model` (offer state enum, fingerprint helpers)
- `internal/api` and `web/` (offer state filter, changed badge, incomplete listing view)
- `migrations/` (new migration for company identity and offer state)
- `docs/SPEC.md`, `docs/ARCHITECTURE.md`, `docs/DATA-MODEL.md`, `docs/API.md`
