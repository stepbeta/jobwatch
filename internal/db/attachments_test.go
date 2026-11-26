package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
)

func attachmentContent(label string) []byte {
	return []byte("%PDF-1.7\n1 0 obj\n<< /Title (" + label + ") >>\nendobj\n%%EOF\n")
}

func attachmentDigest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// countRows is a small helper for the attachment tests.
func countRows(t *testing.T, q Querier, query string, args ...any) int {
	t.Helper()
	var n int
	if err := q.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestAttachOfferDocumentDedupesContent(t *testing.T) {
	sqlDB := openTest(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	company, err := CreateCompany(ctx, sqlDB, "Acme", nil, now)
	if err != nil {
		t.Fatal(err)
	}
	first := seedMergeOffer(t, sqlDB, company, "SRE", "")
	second := seedMergeOffer(t, sqlDB, company, "Platform Engineer", "")

	title, author, producer := "My CV", "Alice", "Test Producer"
	created := now.Add(-48 * time.Hour)
	modified := now.Add(-24 * time.Hour)
	meta := model.PDFMetadata{
		Title:            &title,
		Author:           &author,
		Producer:         &producer,
		CreationDate:     &created,
		ModificationDate: &modified,
		PageCount:        2,
		Version:          "1.7",
	}
	content := attachmentContent("cv")
	digest := attachmentDigest(content)

	att, err := AttachOfferDocument(ctx, sqlDB, first.ID, model.AttachmentCV, "cv.pdf", content, meta, now)
	if err != nil {
		t.Fatal(err)
	}
	if att.ID == 0 || att.OfferID != first.ID || att.SHA256 != digest || att.Size != int64(len(content)) {
		t.Fatalf("attachment = %+v", att)
	}
	if att.Kind != model.AttachmentCV || att.Filename != "cv.pdf" {
		t.Errorf("kind/filename = %q/%q", att.Kind, att.Filename)
	}
	if att.PDF.Title == nil || *att.PDF.Title != title || att.PDF.Author == nil || *att.PDF.Author != author {
		t.Errorf("pdf metadata = %+v", att.PDF)
	}
	if att.PDF.CreationDate == nil || !att.PDF.CreationDate.Equal(created) ||
		att.PDF.ModificationDate == nil || !att.PDF.ModificationDate.Equal(modified) {
		t.Errorf("pdf dates = %v / %v", att.PDF.CreationDate, att.PDF.ModificationDate)
	}
	if att.PDF.PageCount != 2 || att.PDF.Version != "1.7" {
		t.Errorf("pdf pages/version = %d/%q", att.PDF.PageCount, att.PDF.Version)
	}
	if !att.AttachedAt.Equal(now) || !att.FirstUploadedAt.Equal(now) {
		t.Errorf("times = attached %v, first uploaded %v", att.AttachedAt, att.FirstUploadedAt)
	}

	// The same bytes on another offer reuse the blob and keep the first
	// upload time, while the attach time records this offer.
	later := now.Add(time.Hour)
	reused, err := AttachOfferDocument(ctx, sqlDB, second.ID, model.AttachmentOther, "other.pdf", content, meta, later)
	if err != nil {
		t.Fatal(err)
	}
	if reused.SHA256 != digest || reused.Filename != "other.pdf" || reused.Kind != model.AttachmentOther {
		t.Errorf("reused attachment = %+v", reused)
	}
	if !reused.FirstUploadedAt.Equal(now) {
		t.Errorf("first uploaded = %v, want %v", reused.FirstUploadedAt, now)
	}
	if !reused.AttachedAt.Equal(later) {
		t.Errorf("attached at = %v, want %v", reused.AttachedAt, later)
	}
	if n := countRows(t, sqlDB, `SELECT COUNT(*) FROM attachment_blobs`); n != 1 {
		t.Errorf("blobs = %d, want 1", n)
	}
	if n := countRows(t, sqlDB, `SELECT COUNT(*) FROM offer_attachments`); n != 2 {
		t.Errorf("attachments = %d, want 2", n)
	}

	// The same content twice on one offer is a conflict and changes nothing.
	_, err = AttachOfferDocument(ctx, sqlDB, first.ID, model.AttachmentCV, "again.pdf", content, meta, later)
	if !errors.Is(err, ErrAttachmentExists) {
		t.Fatalf("duplicate err = %v, want ErrAttachmentExists", err)
	}
	if n := countRows(t, sqlDB, `SELECT COUNT(*) FROM offer_attachments`); n != 2 {
		t.Errorf("attachments after duplicate = %d, want 2", n)
	}

	// An unknown offer is not found and stores no blob.
	unknown := attachmentContent("unknown")
	if _, err := AttachOfferDocument(ctx, sqlDB, 99999, model.AttachmentCV, "u.pdf", unknown, model.PDFMetadata{}, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown offer err = %v, want ErrNotFound", err)
	}
	if n := countRows(t, sqlDB, `SELECT COUNT(*) FROM attachment_blobs`); n != 1 {
		t.Errorf("blobs after unknown offer = %d, want 1", n)
	}

	// An invalid kind is rejected before any write.
	if _, err := AttachOfferDocument(ctx, sqlDB, first.ID, model.AttachmentKind("label"), "x.pdf", attachmentContent("x"), model.PDFMetadata{}, now); err == nil {
		t.Error("invalid kind was accepted")
	}
}

func TestDetachOfferAttachmentCleansUp(t *testing.T) {
	sqlDB := openTest(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	company, _ := CreateCompany(ctx, sqlDB, "Acme", nil, now)
	first := seedMergeOffer(t, sqlDB, company, "SRE", "")
	second := seedMergeOffer(t, sqlDB, company, "Platform Engineer", "")
	content := attachmentContent("shared")
	digest := attachmentDigest(content)

	att1, err := AttachOfferDocument(ctx, sqlDB, first.ID, model.AttachmentCV, "cv.pdf", content, model.PDFMetadata{}, now)
	if err != nil {
		t.Fatal(err)
	}
	att2, err := AttachOfferDocument(ctx, sqlDB, second.ID, model.AttachmentCV, "cv.pdf", content, model.PDFMetadata{}, now)
	if err != nil {
		t.Fatal(err)
	}

	// A shared file survives detaching one reference.
	if err := DetachOfferAttachment(ctx, sqlDB, first.ID, att1.ID); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, sqlDB, `SELECT COUNT(*) FROM attachment_blobs WHERE sha256 = ?`, digest); n != 1 {
		t.Errorf("blob after detaching a shared reference = %d, want 1", n)
	}
	file, err := GetBlobFile(ctx, sqlDB, digest)
	if err != nil {
		t.Fatalf("shared blob not downloadable: %v", err)
	}
	if !bytes.Equal(file.Bytes, content) {
		t.Error("shared blob bytes changed")
	}
	remaining, err := ListOfferAttachments(ctx, sqlDB, second.ID)
	if err != nil || len(remaining) != 1 || remaining[0].ID != att2.ID {
		t.Errorf("remaining attachments = %+v, err %v", remaining, err)
	}

	// The last reference removes the attachment and the blob.
	if err := DetachOfferAttachment(ctx, sqlDB, second.ID, att2.ID); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, sqlDB, `SELECT COUNT(*) FROM attachment_blobs`); n != 0 {
		t.Errorf("blobs after last detach = %d, want 0", n)
	}
	if _, err := GetBlobFile(ctx, sqlDB, digest); !errors.Is(err, ErrNotFound) {
		t.Errorf("blob after last detach err = %v, want ErrNotFound", err)
	}

	// Unknown or foreign attachment IDs are not found and change nothing.
	if err := DetachOfferAttachment(ctx, sqlDB, first.ID, att2.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("foreign detach err = %v, want ErrNotFound", err)
	}

	// Two distinct files are stored separately.
	if _, err := AttachOfferDocument(ctx, sqlDB, first.ID, model.AttachmentCV, "one.pdf", attachmentContent("one"), model.PDFMetadata{}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := AttachOfferDocument(ctx, sqlDB, first.ID, model.AttachmentOther, "two.pdf", attachmentContent("two"), model.PDFMetadata{}, now); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, sqlDB, `SELECT COUNT(*) FROM attachment_blobs`); n != 2 {
		t.Errorf("distinct blobs = %d, want 2", n)
	}
}

func TestListDocumentsGroupsReferences(t *testing.T) {
	sqlDB := openTest(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	company, _ := CreateCompany(ctx, sqlDB, "Acme", nil, now)
	first := seedMergeOffer(t, sqlDB, company, "SRE", "")
	second := seedMergeOffer(t, sqlDB, company, "Platform Engineer", "")
	shared := attachmentContent("shared")
	other := attachmentContent("other")

	if _, err := AttachOfferDocument(ctx, sqlDB, first.ID, model.AttachmentCV, "cv.pdf", shared, model.PDFMetadata{}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := AttachOfferDocument(ctx, sqlDB, second.ID, model.AttachmentCV, "cv-again.pdf", shared, model.PDFMetadata{}, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := AttachOfferDocument(ctx, sqlDB, first.ID, model.AttachmentCoverLetter, "letter.pdf", other, model.PDFMetadata{}, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}

	docs, err := ListDocuments(ctx, sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("documents = %d, want 2 (one row per stored file)", len(docs))
	}
	bySHA := map[string]Document{}
	for _, d := range docs {
		bySHA[d.SHA256] = d
	}
	sharedDoc, ok := bySHA[attachmentDigest(shared)]
	if !ok {
		t.Fatal("shared document missing")
	}
	if len(sharedDoc.References) != 2 {
		t.Errorf("shared references = %d, want 2", len(sharedDoc.References))
	}
	if !sharedDoc.FirstUploadedAt.Equal(now) {
		t.Errorf("first uploaded = %v, want %v", sharedDoc.FirstUploadedAt, now)
	}
	seenOffers := map[int64]bool{}
	for _, ref := range sharedDoc.References {
		seenOffers[ref.OfferID] = true
		if ref.Kind != model.AttachmentCV || ref.CompanyName != "Acme" {
			t.Errorf("reference = %+v", ref)
		}
	}
	if !seenOffers[first.ID] || !seenOffers[second.ID] {
		t.Errorf("references cover offers %v", seenOffers)
	}

	// Detaching the last reference removes the row from the aggregate.
	otherDoc := bySHA[attachmentDigest(other)]
	if len(otherDoc.References) != 1 {
		t.Fatalf("other references = %d, want 1", len(otherDoc.References))
	}
	if err := DetachOfferAttachment(ctx, sqlDB, first.ID, otherDoc.References[0].AttachmentID); err != nil {
		t.Fatal(err)
	}
	docs, err = ListDocuments(ctx, sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 || docs[0].SHA256 != attachmentDigest(shared) {
		t.Errorf("documents after detach = %+v", docs)
	}
}

func TestMergeOffersMovesAttachments(t *testing.T) {
	tests := []struct {
		name           string
		targetContent  []string
		sourceContent  []string
		wantFiles      int
		wantMoved      int
		wantRefsOnKept int
		wantDocs       int
	}{
		{
			name:           "disjoint documents",
			targetContent:  []string{"target"},
			sourceContent:  []string{"source"},
			wantFiles:      2,
			wantMoved:      1,
			wantRefsOnKept: 2,
			wantDocs:       2,
		},
		{
			name:           "overlapping documents collapse",
			targetContent:  []string{"shared", "target-only"},
			sourceContent:  []string{"shared", "source-only"},
			wantFiles:      3,
			wantMoved:      1,
			wantRefsOnKept: 3,
			wantDocs:       3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlDB := openTest(t)
			ctx := context.Background()
			now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
			company, _ := CreateCompany(ctx, sqlDB, "Acme", nil, now)
			target := seedMergeOffer(t, sqlDB, company, "SRE", "")
			source := seedMergeOffer(t, sqlDB, company, "SRE - Platform", "")

			for _, label := range tt.targetContent {
				if _, err := AttachOfferDocument(ctx, sqlDB, target.ID, model.AttachmentCV,
					label+".pdf", attachmentContent(label), model.PDFMetadata{}, now); err != nil {
					t.Fatal(err)
				}
			}
			for _, label := range tt.sourceContent {
				if _, err := AttachOfferDocument(ctx, sqlDB, source.ID, model.AttachmentOther,
					label+".pdf", attachmentContent(label), model.PDFMetadata{}, now); err != nil {
					t.Fatal(err)
				}
			}

			res, err := MergeOffers(ctx, sqlDB, source.ID, target.ID, now)
			if err != nil {
				t.Fatal(err)
			}
			if res.MovedAttachments != tt.wantMoved {
				t.Errorf("moved attachments = %d, want %d", res.MovedAttachments, tt.wantMoved)
			}
			kept, err := ListOfferAttachments(ctx, sqlDB, target.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(kept) != tt.wantRefsOnKept {
				t.Errorf("kept attachments = %d, want %d", len(kept), tt.wantRefsOnKept)
			}
			if n := countRows(t, sqlDB, `SELECT COUNT(*) FROM offer_attachments WHERE offer_id = ?`, source.ID); n != 0 {
				t.Errorf("absorbed offer still has %d attachments", n)
			}
			if n := countRows(t, sqlDB, `SELECT COUNT(*) FROM attachment_blobs`); n != tt.wantFiles {
				t.Errorf("blobs = %d, want %d", n, tt.wantFiles)
			}
			docs, err := ListDocuments(ctx, sqlDB)
			if err != nil {
				t.Fatal(err)
			}
			if len(docs) != tt.wantDocs {
				t.Errorf("documents = %d, want %d", len(docs), tt.wantDocs)
			}
			for _, d := range docs {
				if len(d.References) != 1 || d.References[0].OfferID != target.ID {
					t.Errorf("document references after merge = %+v", d.References)
				}
			}
		})
	}
}
