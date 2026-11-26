# Spec Delta

## Purpose

Lets the user keep the exact CV and cover letter PDF sent with an application
attached to the offer, read them back byte-for-byte, and see which revision
reached which employer.

## ADDED Requirements

### Requirement: Uploading a PDF against an offer

An offer SHALL accept an uploaded file of at most 2 MiB whose content begins
with the PDF header, tagged as `cv`, `cover_letter` or `other`. The stored
filename SHALL be the client's basename, with no directory part. Rejections:
non-PDF `400 unsupported_file_type`, oversized `413 file_too_large`, missing or
invalid kind `400 validation_failed`, unknown offer `404 not_found`.

#### Scenario: Upload appears on the offer

- **WHEN** the user uploads a valid 120 KB PDF tagged `cv` to an offer
- **THEN** the offer lists it immediately with kind `cv` and its filename

#### Scenario: Content decides the type, not the extension

- **WHEN** the user uploads a file named `cv.pdf` whose content is not a PDF
- **THEN** the request returns `400 unsupported_file_type` and nothing is stored

#### Scenario: Oversized file is rejected

- **WHEN** the uploaded content exceeds 2 MiB
- **THEN** the request returns `413 file_too_large` and nothing is stored

#### Scenario: Kind is required

- **WHEN** an upload omits the kind or sends `label`
- **THEN** the request returns `400 validation_failed`

#### Scenario: Directory parts are stripped

- **WHEN** the client sends a filename containing directory separators
- **THEN** only the basename is stored

### Requirement: Identical content is stored once

Stored files SHALL be content-addressed: bytes identical to an already stored
file SHALL reuse the existing copy instead of a second one. Attaching an already
stored file to another offer SHALL be allowed; attaching it to an offer that
already references it MUST be rejected with `409 already_attached` and change
nothing.

#### Scenario: Same CV on a second offer

- **WHEN** the user uploads the same PDF to a second offer
- **THEN** it is attached and the stored content is not duplicated

#### Scenario: Same CV twice on one offer

- **WHEN** the user uploads a PDF the offer already references
- **THEN** the request returns `409 already_attached` and the offer keeps one entry

#### Scenario: An edited file is a new file

- **WHEN** the user re-exports the CV with a changed byte and uploads it
- **THEN** it is stored as a separate document and both appear on the offer

### Requirement: Documents belong to the offer, not the timeline

An attached document SHALL be a property of the offer. Attaching or detaching a
document MUST NOT create, modify or delete timeline events, MUST NOT change the
application state, and no document SHALL be attributed to a specific event.

#### Scenario: Attaching creates no event

- **WHEN** the user attaches a cover letter to an offer with no events
- **THEN** the timeline stays empty

#### Scenario: Attaching does not open an application

- **WHEN** the user attaches a CV to an offer in state `none`
- **THEN** the offer stays in `none`

#### Scenario: Detaching leaves the timeline untouched

- **WHEN** the user detaches a document from an offer with `cv_sent` and `interview` events
- **THEN** both events remain unchanged

### Requirement: Files live in the database

Uploaded content SHALL be stored inside the single SQLite database, so that a
`jobwatch backup` copy contains every document. JobWatch MUST NOT write document
files to a side directory and MUST NOT depend on an external service. The bytes
returned by a download MUST equal the bytes uploaded.

#### Scenario: A backup carries the documents

- **WHEN** the user backs up, restores that copy and opens the offer
- **THEN** the attached documents are listed and download identically

#### Scenario: Round trip is byte-identical

- **WHEN** the user downloads an uploaded PDF
- **THEN** its bytes and size match the original file exactly

### Requirement: PDF metadata is extracted best-effort

At upload the system SHALL record the PDF's title, author, producer, creation
time, modification time, page count and PDF version when they are readable.
Every field SHALL be optional: a missing, unreadable or malformed information
section MUST NOT fail the upload and MUST leave those fields empty.

#### Scenario: A generated CV reports its metadata

- **WHEN** the user uploads a PDF written by a common PDF producer
- **THEN** the stored document shows its producer, creation time and page count

#### Scenario: Unreadable metadata still uploads

- **WHEN** the PDF's information section cannot be read
- **THEN** the upload succeeds and those fields are shown as unknown

#### Scenario: Unknown is not invented

- **WHEN** a PDF carries no creation date
- **THEN** the document shows no creation date rather than a guessed one

### Requirement: The offer page lists its documents

The offer detail page SHALL list every attached document with its kind,
filename, size, page count when known, and a download action and a detach
action. The section MUST be present with an empty state when no document is
attached.

#### Scenario: Two documents on an offer

- **WHEN** an offer has a CV and a cover letter attached
- **THEN** both are listed with their kind, and each is downloadable

#### Scenario: Empty offer

- **WHEN** an offer has no attached document
- **THEN** the section states that no document is attached and offers an upload

#### Scenario: Detach from the offer page

- **WHEN** the user detaches a document and confirms
- **THEN** it disappears from the offer and the rest of the page is unchanged

### Requirement: Upload requires a chosen file

The upload control SHALL place the kind selector and the file picker on one
compact row, with the upload action beside them. The upload action MUST stay
disabled until a file has been chosen, so an empty submission can never be sent.
A chosen file SHALL be shown as a chip naming it, with a control that clears the
choice. When the offer already has a document, the row SHALL be revealed by an
"Add another" control instead of being shown by default.

#### Scenario: Upload disabled before a file is chosen

- **WHEN** the upload row is open and no file is selected
- **THEN** the upload action is disabled

#### Scenario: Upload enabled after choosing a file

- **WHEN** the user chooses a PDF
- **THEN** the upload action becomes enabled and shows the chosen filename

#### Scenario: A wrong pick can be removed

- **WHEN** the user activates the chosen file chip's remove control
- **THEN** the picker returns to its initial state and the upload action is disabled again

#### Scenario: The same file can be chosen again

- **WHEN** the user removes a chosen file and selects the same file again
- **THEN** the chip shows it and the upload action is enabled

#### Scenario: Add another reveals the row

- **WHEN** the offer already has a document and the user activates "Add another"
- **THEN** the upload row appears once

#### Scenario: First upload needs no extra click

- **WHEN** the offer has no document
- **THEN** the upload row is open without activating "Add another"

### Requirement: Kind filters on the offer page

When an offer holds documents of more than one kind, the offer page SHALL offer
kind filters with a count of files per kind, unfiltered by default. Selecting a
kind SHALL list only that kind's attachments. The filter row MUST be hidden when
only one kind is present.

#### Scenario: Offer page shows the CVs

- **WHEN** an offer has two CVs and one cover letter and the user selects CV
- **THEN** only the two CVs are listed

#### Scenario: Single kind needs no filter

- **WHEN** every document attached to the offer has the same kind
- **THEN** no filter row is shown

#### Scenario: Default shows everything

- **WHEN** the offer page opens
- **THEN** every attached document is visible

### Requirement: Kind filters on the documents overview

When stored files cover more than one kind, the Documents page SHALL offer kind
filters whose counts are distinct files, unfiltered by default. A filter SHALL
select the files referenced at least once with that kind, so a file used with
two kinds appears under both. The filter row MUST be hidden when only one kind is
present.

#### Scenario: Documents page counts distinct files

- **WHEN** one file is attached to three offers and another file to one offer
- **THEN** the file filters show two distinct files, not the attachment count

#### Scenario: A mixed file is reachable from both kinds

- **WHEN** a stored file is referenced as `cv` by one offer and as `cover_letter` by another
- **THEN** selecting either kind lists that file

#### Scenario: Overview defaults to every file

- **WHEN** the Documents page opens
- **THEN** every stored file is visible

### Requirement: Dates are labelled without ambiguity

The user interface SHALL present three distinct times under labels that cannot
be confused: the document's own creation and modification dates read from the
file, the first upload time recorded by JobWatch, and the time the document was
attached to that particular offer. The browser-reported file modification time
MUST NOT be displayed as a document date.

#### Scenario: Reuse shows the original first upload

- **WHEN** a file uploaded in October is attached to another offer in November
- **THEN** its first upload is shown as October and that offer's attach time as November

#### Scenario: Unknown file date is not the upload date

- **WHEN** a PDF carries no creation date
- **THEN** its creation date is shown as unknown while its upload time is shown

#### Scenario: The distinction is explained

- **WHEN** the user opens the Documents page or an offer's document list
- **THEN** help text explains that the file date comes from the PDF while the upload and attach times come from JobWatch

### Requirement: Detaching cleans up unreferenced content

Detaching SHALL remove the attachment from that offer. The stored content SHALL
be deleted once no attachment references it, and MUST be kept, downloadable and
listed while at least one other offer still references it.

#### Scenario: Last reference removed

- **WHEN** the user detaches the only attachment of a file
- **THEN** the file disappears from the Documents page

#### Scenario: Shared file survives

- **WHEN** the user detaches a file that another offer still references
- **THEN** that other offer keeps it and it stays downloadable and listed

### Requirement: Merging offers keeps every document

When two offers are merged, the kept offer SHALL reference every document of the
absorbed offer. A document both offers already referenced SHALL appear once on
the kept offer, and no document MUST be lost.

#### Scenario: Both offers had documents

- **WHEN** the kept offer has a CV and the absorbed offer has a different cover letter, and they are merged
- **THEN** the kept offer lists both documents

#### Scenario: Same file on both offers

- **WHEN** both offers already referenced the same CV, and they are merged
- **THEN** the kept offer lists that CV once

### Requirement: The Documents page lists each stored file once

A read-only Documents page (`/documents`) SHALL list each stored file exactly
once, with its intrinsic metadata, its first upload time and the offers that
reference it. Every row SHALL be downloadable and SHALL link to those offers.
The page MUST NOT offer upload, renaming, re-tagging or deletion.

#### Scenario: A shared file appears once

- **WHEN** one CV is attached to three offers
- **THEN** the page shows a single row naming all three offers

#### Scenario: The page is read-only

- **WHEN** the Documents page is open
- **THEN** it offers no upload, rename or delete action

#### Scenario: A document vanishes with its last reference

- **WHEN** the last attachment of a file is detached on the offer page
- **THEN** the row disappears from the Documents page

### Requirement: Downloads are forced attachments

A download SHALL be served with `Content-Disposition: attachment`, the PDF media
type and `X-Content-Type-Options: nosniff`, and MUST NOT be rendered inline. An
unknown file identifier MUST return `404 not_found`.

#### Scenario: Download header

- **WHEN** the user downloads a document
- **THEN** the response is an attachment with `nosniff` and the PDF media type

#### Scenario: Unknown file

- **WHEN** a download is requested for content that is not stored
- **THEN** the request returns `404 not_found`

### Requirement: The schema change is additive

Upgrading SHALL add the document tables only. No existing table, column, row or
ID SHALL change, and no offer, event, note, snapshot or provenance row may be
deleted or rewritten. A database without documents MUST behave exactly as before
the upgrade apart from the new document sections.

#### Scenario: Upgrade preserves every row

- **WHEN** an existing database is upgraded
- **THEN** every offer, event, note, snapshot and provenance row still exists with the same IDs

#### Scenario: Documents work on an upgraded database

- **WHEN** the upgraded database is served
- **THEN** uploading, listing, downloading and detaching work without further action

### Requirement: The schema change starts from a backup

Before the change is applied to a real database, a consistent copy SHALL be
taken with `jobwatch backup`, which refuses to overwrite and verifies
integrity. Implementation and verification MUST run against that copy first.

#### Scenario: Backup exists before the change is applied

- **WHEN** implementation of this change begins
- **THEN** a timestamped backup file exists and its integrity check passed

#### Scenario: Restoring is possible

- **WHEN** the new behaviour must be abandoned
- **THEN** replacing the database with the backup restores the previous behaviour with all data
