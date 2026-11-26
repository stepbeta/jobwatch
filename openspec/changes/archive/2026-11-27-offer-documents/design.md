# Design

## Context

See `proposal.md` for motivation and `specs/offer-documents/spec.md` for the
behavior contract. Constraints that shape the approach:

- One SQLite file is the whole data store; `jobwatch backup` is
  `VACUUM INTO` + integrity check, so anything stored outside that file is not
  backed up.
- `docs/DATA-MODEL.md` requires state-changing migrations to be additive and
  reversible via the pre-change backup; the FTS and merge machinery already
  enumerate the rows that belong to an offer.
- `internal/api` caps JSON bodies at 1 MiB (`maxBodyBytes`); a binary upload
  needs its own limit and its own endpoint.
- The offer detail payload (`GET /api/offers/{id}`) already embeds notes, events,
  provenance and snapshots, so document listing follows that pattern.
- Go standard library only; no PDF parsing library is available and none is
  wanted.

## Goals / Non-Goals

**Goals:**

- Store uploaded PDFs content-addressed inside SQLite with a hard 2 MiB cap.
- Keep attachments a plain offer property: no event link, no label, no library
  entity, no new top-level lifecycle.
- Expose the stored files twice: per offer (manage) and once globally on
  `/documents` (read-only aggregate with download).
- Never lose content silently on detach or on offer merge.

**Non-Goals:**

- Formats other than PDF, text extraction, thumbnails, or previews.
- Renaming or re-tagging an existing file; versioning a document's history.
- A canonical "current CV" object shared as an entity.
- Storing the browser's file modification time.
- Linking a document to a specific timeline event.

## Decisions

### B1. Two tables: immutable content blob + thin attachment row

`attachment_blobs(sha256 PK, bytes, size, mime, pdf_* metadata, created_at)` and
`offer_attachments(id, offer_id, blob_sha256, kind, filename, created_at,
UNIQUE(offer_id, blob_sha256))`.

Splitting content from reference makes deduplication natural (one row per
distinct byte sequence, one thin row per offer) and lets the Documents page be a
`GROUP BY` over attachments joined to blobs. Alternative considered: a single
table with the bytes inline and one row per offer — that duplicates a 120 KB PDF
per offer and makes "the same file" undetectable. Alternative: files on disk
next to the database — breaks single-file backup and portability.

### B2. SHA-256 over the raw bytes, computed server-side

The client may hash for UX, but the server always computes the digest of what it
received and never trusts a client-supplied hash. Deduplication is exact-bytes
only; two exports of the same CV differ in bytes and therefore in rows, which is
the desired behavior (they *are* different documents).

### B3. Upload via `multipart/form-data`, detached from the 1 MiB JSON cap

`POST /api/offers/{id}/attachments` reads the multipart stream through
`http.MaxBytesReader` with a small overhead allowance above 2 MiB, sniffs the
first bytes for `%PDF-`, and rejects the request before writing anything when the
content is not a PDF or too large. JSON endpoints keep their existing cap.

### B4. Metadata extraction in a small `internal/pdfmeta` package

The parser scans the file for the information dictionary (`/Title`, `/Author`,
`/Producer`, `/CreationDate`, `/ModDate`), decodes PDF string syntax
(literal and hex, UTF-16BE BOM), parses `D:YYYYMMDDHHmmSS±hh'mm'` dates to
RFC3339 UTC, and counts page objects for the page count. Everything is
best-effort: a missing value yields `NULL`, and no parse error fails the upload.

Trade-off: PDFs that keep their information dictionary inside a compressed
object stream report unknown metadata. Alternative considered: adding a PDF
library — rejected for size and dependency cost on a feature where the fields are
presentation only.

### B5. `created_at` exists twice, on purpose

`attachment_blobs.created_at` is the first time those bytes entered the
database ("uploaded"); `offer_attachments.created_at` is when that offer got the
document. The UI labels them separately and explains that both come from
JobWatch, while a third date — the PDF's own creation/modification — comes from
the file. The browser's `lastModified` is not captured: it is a claim about the
disk, unrecoverable later, and nothing consumes it.

### B6. Download by content hash, always as an attachment

`GET /api/documents/{sha256}/file` (and the offer-scoped alias
`GET /api/offers/{id}/attachments/{aid}/file`) streams the blob with
`Content-Disposition: attachment; filename="..."`, `application/pdf` and
`X-Content-Type-Options: nosniff`. Nothing is served inline, so an unexpected
content type can never execute in the application's origin. Only PDFs are
accepted, so the allowlist is a single type; the magic-byte check makes the
stored `mime` a fact rather than a client claim.

### B7. Detach deletes a blob only when unreferenced; merge moves rows

Detach runs in one transaction: delete the attachment row, then
`DELETE FROM attachment_blobs WHERE sha256 = ? AND NOT EXISTS (...)`.
Offer merge, inside its existing transaction, updates `offer_attachments.offer_id`
to the kept offer for rows the kept offer does not already have, deletes the
duplicates, and finally drops blobs that no longer have references. The absorbed
offer's cascade delete therefore never strands bytes.

### B8. Documents page is an aggregate, not a resource

`GET /api/documents` returns one entry per referenced blob with metadata, first
upload time and the referencing offers (id, title, company name, kind, attachment
time). No detail endpoint, no create, no patch, no delete. This keeps the
"one owner per behavior" rule: attachment management lives on the offer page,
and `/documents` only reads.

### B9. Additive migration 0018

Two `CREATE TABLE` statements plus indexes; `job_offers` is untouched, so no
rebuild, no FTS rebuild, and no ID churn. Foreign keys: attachments cascade with
their offer, and blobs are deleted explicitly rather than by cascade so the
cleanup rule is visible in one place.

## Risks / Trade-offs

- **Database growth** — 2 MiB cap times a few dozen applications; `VACUUM INTO`
  copies blobs into every backup. Acceptable; the cap is the control.
- **PDF metadata frequently unknown** — information dictionaries inside object
  streams. Mitigated by nullable fields and an explicit "unknown" rendering.
- **Blob orphaning on unusual paths** (manual SQL, future deletes) — mitigated by
  cleaning up inside the detach and merge transactions; a blob with no
  attachment is invisible in all read paths regardless.
- **Re-upload cost** — an already stored file is uploaded again from the browser
  because labels are gone. Mitigated by the Documents page listing what exists;
  no client-side hashing in this change.
- **Large transaction** — inserting 2 MiB in one write briefly holds a WAL
  frame; acceptable for a single-user local server.
- **`kind` is per attachment, not per file** — the same bytes could be tagged
  `cv` on one offer and `other` on another. The Documents page shows the tags in
  use; this is allowed deliberately.

## Migration Plan

1. `jobwatch backup jobwatch.db.<timestamp>.bkp` before touching any database
   (mandatory first task).
2. Apply `0018_offer_documents.sql` through the normal migration runner on a copy
   of the backup first; confirm existing row counts are unchanged.
3. Serve; existing pages behave as before, offers show an empty documents
   section.
4. Rollback: stop the server, replace the database with the backup file. The
   migration adds tables only, so no data written before it is affected.
