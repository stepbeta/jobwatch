# Changelog

## Unreleased

### Added
- Go module, project layout, `.gitignore`, placeholders for `web/` and
  `internal/classify`.
- SQLite storage (pure-Go driver, WAL mode) with an embedded, ordered
  migration runner. Migration 0001 creates companies, sources,
  job_offers, job_offer_provenance, events, notes, snapshots, and
  full-text search over offer titles/descriptions and notes.
- Normalization helpers for company names (legal suffixes such as
  GmbH/Inc/SAS/Ltd/SpA/LLC removed), job titles (H/F-style markers
  removed), location buckets, fingerprints and description hashes.
- Source adapters for Greenhouse, Lever and Ashby job boards, with a
  shared client that honors robots.txt, spaces requests (2 s default,
  configurable per source), and identifies itself with a JobWatch
  User-Agent. Board URLs are recognized automatically.
- Scan pipeline: new offers are created with a description snapshot;
  offers seen again are refreshed; changed descriptions mark the offer
  `changed` with a new snapshot; offers no longer listed become
  `disappeared` (never deleted). Each source is written in one
  transaction, and a failing source is marked `error`/`blocked` without
  stopping the others.
- Manual import of offers seen elsewhere, by URL or JSON, matched to
  existing offers when the company and title are the same.
- CLI commands: `serve` (local server with `GET /api/health`), `scan`,
  `add-source`, `list`, `import`, `backup`.
- Documentation: ARCHITECTURE, DATA-MODEL, API, MILESTONES.
- README with build, quick-start and command reference.

### Changed
- SPEC updated to match decisions made in Milestone 1: Ashby is fetched
  with GET, a few schema columns are NOT NULL with defaults, enum values
  are checked by the application, and the fingerprint collision rule and
  missing offer-state transitions are spelled out. Revisions are listed
  in SPEC §11.
- MILESTONES now separates permanent deviations from open known gaps.
