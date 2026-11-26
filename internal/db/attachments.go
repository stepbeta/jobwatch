package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
)

// ErrAttachmentExists is returned when an offer already references the
// exact same content. The upload changes nothing in that case.
var ErrAttachmentExists = errors.New("attachment already exists")

// Attachment is one document attached to an offer, joined with the stored
// file's metadata. The bytes are deliberately absent: listing never loads
// them.
type Attachment struct {
	ID              int64
	OfferID         int64
	SHA256          string
	Kind            model.AttachmentKind
	Filename        string
	Size            int64
	Mime            string
	PDF             model.PDFMetadata
	AttachedAt      time.Time
	FirstUploadedAt time.Time
}

// DocumentReference names one offer that references a stored file.
type DocumentReference struct {
	AttachmentID int64
	OfferID      int64
	OfferTitle   string
	CompanyName  string
	Kind         model.AttachmentKind
	AttachedAt   time.Time
}

// Document is one stored file in the Documents aggregate, with every offer
// that references it. It is a read-only view over attachment_blobs.
type Document struct {
	SHA256          string
	Size            int64
	Mime            string
	PDF             model.PDFMetadata
	FirstUploadedAt time.Time
	References      []DocumentReference
}

// StoredFile is a blob's bytes together with the response metadata.
type StoredFile struct {
	Bytes    []byte
	Mime     string
	Filename string
}

const attachmentJoinColumns = `a.id, a.offer_id, a.blob_sha256, a.kind, a.filename, a.created_at,
	b.size, b.mime, b.pdf_title, b.pdf_author, b.pdf_producer,
	b.pdf_creation_date, b.pdf_mod_date, b.pdf_page_count, b.pdf_version, b.created_at`

// ScanAttachment reads one row selected with attachmentJoinColumns.
func scanAttachment(row interface{ Scan(...any) error }) (Attachment, error) {
	var a Attachment
	var kind, attachedAt, firstUploaded sql.NullString
	var title, author, producer, creation, mod, version sql.NullString
	var pages sql.NullInt64
	if err := row.Scan(&a.ID, &a.OfferID, &a.SHA256, &kind, &a.Filename, &attachedAt,
		&a.Size, &a.Mime, &title, &author, &producer, &creation, &mod, &pages, &version,
		&firstUploaded); err != nil {
		return Attachment{}, err
	}
	a.Kind = model.AttachmentKind(kind.String)
	a.PDF = model.PDFMetadata{
		Title:            strPtr(title),
		Author:           strPtr(author),
		Producer:         strPtr(producer),
		CreationDate:     timePtr(creation),
		ModificationDate: timePtr(mod),
		Version:          version.String,
	}
	if pages.Valid {
		a.PDF.PageCount = int(pages.Int64)
	}
	a.AttachedAt = parseNullTime(attachedAt)
	a.FirstUploadedAt = parseNullTime(firstUploaded)
	return a, nil
}

// timePtr converts a nullable stored timestamp to a pointer.
func timePtr(n sql.NullString) *time.Time {
	if !n.Valid {
		return nil
	}
	t := parseTime(n.String)
	if t.IsZero() {
		return nil
	}
	return &t
}

// nullTimePtr converts an optional metadata time to a stored value.
func nullTimePtr(t *time.Time) sql.NullString {
	if t == nil || t.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: model.FormatTime(*t), Valid: true}
}

// AttachOfferDocument stores content as a content-addressed blob and links
// it to offerID. Identical bytes reuse the existing blob, so the first
// upload time and the PDF metadata of the first copy are kept. Attaching
// content the offer already references returns ErrAttachmentExists and
// changes nothing. The whole operation runs in one transaction.
func AttachOfferDocument(ctx context.Context, sqlDB *sql.DB, offerID int64, kind model.AttachmentKind,
	filename string, content []byte, pdf model.PDFMetadata, now time.Time) (Attachment, error) {
	if !kind.Valid() {
		return Attachment{}, fmt.Errorf("attach document: invalid kind %q", kind)
	}
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return Attachment{}, fmt.Errorf("begin attachment of offer %d: %w", offerID, err)
	}
	defer tx.Rollback()

	var exists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM job_offers WHERE id = ?`, offerID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return Attachment{}, ErrNotFound
	}
	if err != nil {
		return Attachment{}, fmt.Errorf("check offer %d: %w", offerID, err)
	}

	var pageCount any
	if pdf.PageCount > 0 {
		pageCount = int64(pdf.PageCount)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO attachment_blobs(
		sha256, bytes, size, mime, pdf_title, pdf_author, pdf_producer,
		pdf_creation_date, pdf_mod_date, pdf_page_count, pdf_version, created_at)
		VALUES (?, ?, ?, 'application/pdf', ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(sha256) DO NOTHING`,
		digest, content, len(content), nullStr(pdf.Title), nullStr(pdf.Author), nullStr(pdf.Producer),
		nullTimePtr(pdf.CreationDate), nullTimePtr(pdf.ModificationDate), pageCount,
		nullStr(nonEmpty(pdf.Version)), model.FormatTime(now)); err != nil {
		return Attachment{}, fmt.Errorf("store document for offer %d: %w", offerID, err)
	}

	err = tx.QueryRowContext(ctx,
		`SELECT 1 FROM offer_attachments WHERE offer_id = ? AND blob_sha256 = ?`, offerID, digest).Scan(&exists)
	switch {
	case err == nil:
		return Attachment{}, ErrAttachmentExists
	case !errors.Is(err, sql.ErrNoRows):
		return Attachment{}, fmt.Errorf("check duplicate attachment: %w", err)
	}

	res, err := tx.ExecContext(ctx, `INSERT INTO offer_attachments(offer_id, blob_sha256, kind, filename, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		offerID, digest, string(kind), filename, model.FormatTime(now))
	if err != nil {
		return Attachment{}, fmt.Errorf("attach document to offer %d: %w", offerID, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Attachment{}, fmt.Errorf("attach document to offer %d: %w", offerID, err)
	}
	att, err := GetOfferAttachment(ctx, tx, offerID, id)
	if err != nil {
		return Attachment{}, err
	}
	if err := tx.Commit(); err != nil {
		return Attachment{}, fmt.Errorf("commit attachment of offer %d: %w", offerID, err)
	}
	return att, nil
}

// nonEmpty returns a pointer to s, or nil when s is empty.
func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ListOfferAttachments returns an offer's documents oldest attachment
// first.
func ListOfferAttachments(ctx context.Context, q Querier, offerID int64) ([]Attachment, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+attachmentJoinColumns+`
		FROM offer_attachments a JOIN attachment_blobs b ON b.sha256 = a.blob_sha256
		WHERE a.offer_id = ? ORDER BY a.created_at, a.id`, offerID)
	if err != nil {
		return nil, fmt.Errorf("list attachments of offer %d: %w", offerID, err)
	}
	defer rows.Close()
	out := []Attachment{}
	for rows.Next() {
		a, err := scanAttachment(rows)
		if err != nil {
			return nil, fmt.Errorf("list attachments of offer %d: %w", offerID, err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetOfferAttachment returns one attachment, enforcing that it belongs to
// the offer.
func GetOfferAttachment(ctx context.Context, q Querier, offerID, attachmentID int64) (Attachment, error) {
	a, err := scanAttachment(q.QueryRowContext(ctx, `SELECT `+attachmentJoinColumns+`
		FROM offer_attachments a JOIN attachment_blobs b ON b.sha256 = a.blob_sha256
		WHERE a.id = ? AND a.offer_id = ?`, attachmentID, offerID))
	if errors.Is(err, sql.ErrNoRows) {
		return Attachment{}, ErrNotFound
	}
	if err != nil {
		return Attachment{}, fmt.Errorf("get attachment %d of offer %d: %w", attachmentID, offerID, err)
	}
	return a, nil
}

// DetachOfferAttachment removes one attachment and deletes the stored
// content when no other attachment references it. Everything runs in one
// transaction.
func DetachOfferAttachment(ctx context.Context, sqlDB *sql.DB, offerID, attachmentID int64) error {
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin detach of attachment %d: %w", attachmentID, err)
	}
	defer tx.Rollback()

	var digest string
	err = tx.QueryRowContext(ctx,
		`SELECT blob_sha256 FROM offer_attachments WHERE id = ? AND offer_id = ?`,
		attachmentID, offerID).Scan(&digest)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read attachment %d: %w", attachmentID, err)
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM offer_attachments WHERE id = ? AND offer_id = ?`, attachmentID, offerID); err != nil {
		return fmt.Errorf("detach attachment %d: %w", attachmentID, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM attachment_blobs
		WHERE sha256 = ? AND NOT EXISTS (
			SELECT 1 FROM offer_attachments WHERE blob_sha256 = ?)`, digest, digest); err != nil {
		return fmt.Errorf("clean up blob %s: %w", digest, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit detach of attachment %d: %w", attachmentID, err)
	}
	return nil
}

// GetOfferAttachmentFile returns the bytes of one attachment together with
// the filename and media type to serve.
func GetOfferAttachmentFile(ctx context.Context, q Querier, offerID, attachmentID int64) (StoredFile, error) {
	var f StoredFile
	err := q.QueryRowContext(ctx, `SELECT b.bytes, b.mime, a.filename
		FROM offer_attachments a JOIN attachment_blobs b ON b.sha256 = a.blob_sha256
		WHERE a.id = ? AND a.offer_id = ?`, attachmentID, offerID).
		Scan(&f.Bytes, &f.Mime, &f.Filename)
	if errors.Is(err, sql.ErrNoRows) {
		return StoredFile{}, ErrNotFound
	}
	if err != nil {
		return StoredFile{}, fmt.Errorf("read attachment %d of offer %d: %w", attachmentID, offerID, err)
	}
	return f, nil
}

// GetBlobFile returns the bytes of a stored file by content hash. The
// filename comes from the first attachment that references it; a blob with
// no live reference falls back to "<sha256>.pdf".
func GetBlobFile(ctx context.Context, q Querier, digest string) (StoredFile, error) {
	var f StoredFile
	var storedDigest, filename sql.NullString
	err := q.QueryRowContext(ctx, `SELECT b.bytes, b.mime, b.sha256,
			(SELECT a.filename FROM offer_attachments a
			  WHERE a.blob_sha256 = b.sha256 ORDER BY a.id LIMIT 1)
		FROM attachment_blobs b WHERE b.sha256 = ?`, digest).
		Scan(&f.Bytes, &f.Mime, &storedDigest, &filename)
	if errors.Is(err, sql.ErrNoRows) {
		return StoredFile{}, ErrNotFound
	}
	if err != nil {
		return StoredFile{}, fmt.Errorf("read blob %s: %w", digest, err)
	}
	f.Filename = storedDigest.String + ".pdf"
	if filename.Valid && filename.String != "" {
		f.Filename = filename.String
	}
	return f, nil
}

// ListDocuments returns every stored file once, with the offers that
// reference it, newest upload first.
func ListDocuments(ctx context.Context, q Querier) ([]Document, error) {
	rows, err := q.QueryContext(ctx, `SELECT
			b.sha256, b.size, b.mime, b.pdf_title, b.pdf_author, b.pdf_producer,
			b.pdf_creation_date, b.pdf_mod_date, b.pdf_page_count, b.pdf_version, b.created_at,
			a.id, a.kind, a.created_at, o.id, o.title, COALESCE(c.name, '')
		FROM attachment_blobs b
		JOIN offer_attachments a ON a.blob_sha256 = b.sha256
		JOIN job_offers o ON o.id = a.offer_id
		LEFT JOIN companies c ON c.id = o.company_id
		ORDER BY b.created_at DESC, b.sha256, a.created_at, a.id`)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	out := []Document{}
	index := map[string]int{}
	for rows.Next() {
		var sha, mime, created, kind, attached, company string
		var offerID int64
		var offerTitle string
		var attachmentID int64
		var size int64
		var title, author, producer, creation, mod, version sql.NullString
		var pages sql.NullInt64
		if err := rows.Scan(&sha, &size, &mime, &title, &author, &producer,
			&creation, &mod, &pages, &version, &created,
			&attachmentID, &kind, &attached, &offerID, &offerTitle, &company); err != nil {
			return nil, fmt.Errorf("list documents: %w", err)
		}
		i, ok := index[sha]
		if !ok {
			d := Document{
				SHA256: sha,
				Size:   size,
				Mime:   mime,
				PDF: model.PDFMetadata{
					Title:            strPtr(title),
					Author:           strPtr(author),
					Producer:         strPtr(producer),
					CreationDate:     timePtr(creation),
					ModificationDate: timePtr(mod),
					Version:          version.String,
				},
				FirstUploadedAt: parseTime(created),
				References:      []DocumentReference{},
			}
			if pages.Valid {
				d.PDF.PageCount = int(pages.Int64)
			}
			out = append(out, d)
			i = len(out) - 1
			index[sha] = i
		}
		out[i].References = append(out[i].References, DocumentReference{
			AttachmentID: attachmentID,
			OfferID:      offerID,
			OfferTitle:   offerTitle,
			CompanyName:  company,
			Kind:         model.AttachmentKind(kind),
			AttachedAt:   parseTime(attached),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	return out, nil
}

// mergeAttachments moves the absorbed offer's documents to the kept offer,
// collapsing a file both offers already reference, and drops content that
// no longer has a reference. It runs inside the merge transaction.
func mergeAttachments(ctx context.Context, q Querier, fromID, intoID int64) (int, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id, blob_sha256 FROM offer_attachments WHERE offer_id = ? ORDER BY id`, fromID)
	if err != nil {
		return 0, fmt.Errorf("list attachments of offer %d: %w", fromID, err)
	}
	type row struct {
		id     int64
		digest string
	}
	var pending []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.digest); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan attachments of offer %d: %w", fromID, err)
		}
		pending = append(pending, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	moved := 0
	for _, r := range pending {
		var exists int
		err := q.QueryRowContext(ctx,
			`SELECT 1 FROM offer_attachments WHERE offer_id = ? AND blob_sha256 = ?`,
			intoID, r.digest).Scan(&exists)
		switch {
		case err == nil:
			if _, err := q.ExecContext(ctx,
				`DELETE FROM offer_attachments WHERE id = ?`, r.id); err != nil {
				return 0, fmt.Errorf("collapse duplicate attachment %d: %w", r.id, err)
			}
		case !errors.Is(err, sql.ErrNoRows):
			return 0, fmt.Errorf("check target attachment: %w", err)
		default:
			if _, err := q.ExecContext(ctx,
				`UPDATE offer_attachments SET offer_id = ? WHERE id = ?`, intoID, r.id); err != nil {
				return 0, fmt.Errorf("move attachment %d: %w", r.id, err)
			}
			moved++
		}
		if _, err := q.ExecContext(ctx, `DELETE FROM attachment_blobs
			WHERE sha256 = ? AND NOT EXISTS (
				SELECT 1 FROM offer_attachments WHERE blob_sha256 = ?)`, r.digest, r.digest); err != nil {
			return 0, fmt.Errorf("clean up blob %s: %w", r.digest, err)
		}
	}
	return moved, nil
}
