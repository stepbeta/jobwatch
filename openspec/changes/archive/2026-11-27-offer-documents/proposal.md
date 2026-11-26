# Proposal

## Why

JobWatch records that an application was sent (`cv_sent` events) but not *what*
was sent. The CV is revised over time and re-exported under the same filename,
so after a few applications there is no way to answer "which version of my CV
reached this employer?". Keeping the exact PDF next to the offer closes that
gap without leaving the single-file, offline model the project promises.

## What Changes

- Offers gain **documents**: an uploaded PDF (≤ 2 MiB) tagged as `cv`,
  `cover_letter` or `other`, listed on the offer page, downloadable from it,
  and detachable.
- Files are stored **inside SQLite** as content-addressed blobs (SHA-256), so
  `jobwatch backup` keeps capturing everything in one file and the binary keeps
  zero external storage and zero new dependencies.
- PDF-intrinsic metadata (title, author, producer, creation and modification
  date, page count, PDF version) is extracted best-effort at upload and every
  field may be unknown.
- A new read-only **Documents** page (`/documents`) lists every stored file once,
  with its intrinsic metadata, its first upload time, and the offers that
  reference it. It is an aggregate over blobs, not a new managed entity: uploads,
  re-tagging and deletion stay on the offer page.
- The UI states explicitly what each date means: the PDF's own creation date
  (from the file), the first upload time (JobWatch), and when the file was
  attached to that particular offer. The browser's file modification time is
  deliberately not stored.
- Offer merge moves attachments to the kept offer and collapses duplicate
  file/offer pairs; detaching deletes the blob only when no offer references it.

Not a breaking change. Existing tables are untouched; the schema change is
additive (migration `0018`), and the implementation starts from a
`jobwatch backup` snapshot.

## Capabilities

### New Capabilities

- `offer-documents`: storing a PDF document against an offer (upload, kind,
  limits, deduplication), reading it back (download, offer list, the Documents
  overview, date semantics), and its lifecycle under detach, merge and
  automatic blob cleanup.

### Modified Capabilities

- `web-ui-shell`: the persistent navigation must also link to Documents and mark
  it as the active section on that route.

## Impact

- `migrations/0018_offer_documents.sql` (new tables `attachment_blobs`,
  `offer_attachments`), `migrations/migrations.go` embed list.
- New package `internal/pdfmeta` (PDF `/Info` and page-tree scan, stdlib only).
- `internal/db`: blob and attachment queries, transactional attach/detach with
  orphan cleanup, merge integration.
- `internal/api`: upload/download handlers with a 2 MiB cap, attachment list and
  delete, the documents aggregate, attachments embedded in `GET /api/offers/{id}`.
- `web/src`: offer page documents section, new `DocumentsPage`, route, sidebar
  entry, API client and types.
- Docs: `API.md`, `DATA-MODEL.md`, `ARCHITECTURE.md`, `MILESTONES.md`,
  `CHANGELOG.md`, `README.md` (backup note), `AGENTS.md` (new package).
- No new Go module dependencies; no filesystem storage; no outbound requests.
