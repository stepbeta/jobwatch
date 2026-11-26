# Tasks

## 1. Protect the database before any change

- [x] 1.1 Run `jobwatch backup jobwatch.db.<timestamp>.bkp` (or `cp` while no process holds the file) and verify the copy passes `jobwatch backup`'s integrity check; record the file name in the session summary
- [x] 1.2 Copy that backup to a scratch path and confirm `jobwatch` starts and serves against it, so the apply phase has a safe database to migrate first

## 2. Schema and data access

- [x] 2.1 Add `migrations/0018_offer_documents.sql` (tables `attachment_blobs`, `offer_attachments`, indexes on `offer_id` and `blob_sha256`) and register it; verify `go test ./migrations/...` and that applying it to the 1.2 copy leaves every existing row count unchanged (`SELECT COUNT(*)` before/after for offers, events, notes, snapshots, provenance)
- [x] 2.2 Implement blob and attachment queries in `internal/db` (attach with dedupe lookup, list by offer, get attachment joined with blob, detach with unreferenced-blob cleanup, documents aggregate, merge integration) and verify table-driven tests cover: new content, reused content, duplicate attachment conflict, detach of last reference, detach with a shared reference, merge with disjoint and overlapping documents
- [x] 2.3 Document both tables, their columns, the uniqueness rule and the cleanup rule in `docs/DATA-MODEL.md`; verify the doc names every column present in 0018

## 3. PDF metadata extraction

- [x] 3.1 Add `internal/pdfmeta` extracting title, author, producer, creation date, modification date, page count and PDF version best-effort; verify table-driven unit tests with small in-repo fixtures: a normal information dictionary, hex and UTF-16BE strings, a `D:` date with an offset, a missing date, a missing dictionary, and a truncated file
- [x] 3.2 Verify the extractor never returns an error that can fail an upload (malformed input yields empty fields), by asserting `Extract` has no error path or returns only nil on garbage input in its tests
- [x] 3.3 Add the package to the repository layout and its role to `docs/ARCHITECTURE.md`; verify `AGENTS.md` lists `internal/pdfmeta` with the other packages

## 4. HTTP API

- [x] 4.1 Add `POST /api/offers/{id}/attachments` (multipart, 2 MiB cap via `MaxBytesReader`, `%PDF-` magic check, required kind, basename-only filename) and verify tests cover success, non-PDF `400 unsupported_file_type`, oversized `413 file_too_large`, invalid kind `400 validation_failed`, unknown offer `404 not_found`, and `409 already_attached`
- [x] 4.2 Add attachment listing to `GET /api/offers/{id}` and `DELETE /api/offers/{id}/attachments/{aid}`; verify tests cover the embedded list, detach removing the last reference, and detach keeping shared content
- [x] 4.3 Add `GET /api/offers/{id}/attachments/{aid}/file` and `GET /api/documents/{sha256}/file`; verify tests assert byte-identical round trip, `Content-Disposition: attachment`, `application/pdf`, `X-Content-Type-Options: nosniff`, and `404` for unknown ids
- [x] 4.4 Add `GET /api/documents`; verify tests assert one row per stored file, first upload time, referencing offers, and that a file whose last reference was detached disappears
- [x] 4.5 Verify merge behaviour end-to-end at the API level (attachments moved, duplicates collapsed, blobs kept); extend the existing merge tests
- [x] 4.6 Document every new endpoint, parameter, response shape and error code in `docs/API.md`; verify each documented code appears in `internal/api` errors

## 5. Offer page

- [x] 5.1 Add the documents section to `web/src/pages/OfferPage.tsx` (list with kind badge, filename, size, pages, dates, download, detach with confirmation, upload form with kind select) and verify `OfferPage.test.tsx` covers empty state, two documents, upload, and detach
- [x] 5.2 Add the date help text that separates the PDF's own dates from the first upload time and this offer's attach time, and verify a test asserts all three labels render and that no browser modification time is displayed
- [x] 5.3 Compact the upload control: kind selector and file picker on one row, a styled picker showing the chosen filename, the upload action disabled until a file is chosen, and an `Add another` entry point once the offer has documents; verify tests cover the disabled state, the enabled state after choosing, and `Add another` revealing the row
- [x] 5.4 Add kind filter chips with file counts on the offer page, hidden when only one kind is present; verify a test filters a two-CV/one-cover-letter offer down to the CVs and that the default shows everything
- [x] 5.5 Replace the chosen file's button with a removable filename chip; verify a test clears the choice, sees the upload action disabled again, and can choose the same file a second time

## 6. Documents page

- [x] 6.1 Add `DocumentsPage.tsx`, the `documents` route in `router.ts` and the sidebar entry, with title `Documents — JobWatch`; verify `router.test.ts`, a page test for one-row-per-file with multiple offers and the unknown-metadata placeholders, and a sidebar active-section assertion
- [x] 6.2 Verify the page exposes no upload, rename or delete action (absence assertion in the page test) and that each row downloads and links to its offers
- [x] 6.3 Add kind filter chips with counts of distinct files, where a file used with two kinds appears under both; verify a test proves the mixed-file behavior, that counts are files rather than attachments, and that the row is hidden for a single kind

## 7. Integration and project documentation

- [x] 7.1 Verify `go build ./...`, `go vet ./...`, `go test ./...` and the frontend test suite pass; run a manual smoke test on the migrated copy: upload, download, attach the same file to a second offer, detach from one, open `/documents`
- [x] 7.2 Update `docs/ARCHITECTURE.md` (request flow, storage model, upload/download path) and `README.md` (what a backup now contains, 2 MiB PDF limit); verify both statements match the implemented cap and endpoints
- [x] 7.3 Add the `CHANGELOG.md` entry under Unreleased and update `docs/MILESTONES.md` (milestone status, known gaps such as PDF-only uploads and unknown metadata for compressed PDFs)

## Workflow follow-up

- Archive the change with `openspec archive` once tasks are done and reviewed; the new `offer-documents` capability spec becomes authoritative and no `docs/SPEC.md` section needs a pointer because none covered attachments.
- Confirm `docs/SPEC.md` still holds no attachment description before archiving.
