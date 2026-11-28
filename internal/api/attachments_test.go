package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
)

func testPDF(title string) []byte {
	return []byte(fmt.Sprintf(`%%PDF-1.7
1 0 obj
<< /Title (%s) /Producer (Test Suite) /CreationDate (D:20260101120000Z) >>
endobj
2 0 obj
<< /Type /Page >>
endobj
%%%%EOF
`, title))
}

func digestOf(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

type attachmentPayload struct {
	ID              int64  `json:"id"`
	OfferID         int64  `json:"offer_id"`
	SHA256          string `json:"sha256"`
	Kind            string `json:"kind"`
	Filename        string `json:"filename"`
	Size            int64  `json:"size"`
	Mime            string `json:"mime"`
	AttachedAt      string `json:"attached_at"`
	FirstUploadedAt string `json:"first_uploaded_at"`
	DownloadURL     string `json:"download_url"`
	PDF             struct {
		Title        *string `json:"title"`
		PageCount    *int    `json:"page_count"`
		CreationDate *string `json:"creation_date"`
	} `json:"pdf"`
}

type attachmentEnvelope struct {
	Attachment attachmentPayload `json:"attachment"`
}

type offerDetailEnvelope struct {
	Offer struct {
		ApplicationState string `json:"application_state"`
	} `json:"offer"`
	Attachments []attachmentPayload `json:"attachments"`
	Events      []struct {
		ID         int64  `json:"id"`
		Type       string `json:"type"`
		OccurredAt string `json:"occurred_at"`
	} `json:"events"`
}

type documentReferencePayload struct {
	AttachmentID int64  `json:"attachment_id"`
	OfferID      int64  `json:"offer_id"`
	OfferTitle   string `json:"offer_title"`
	CompanyName  string `json:"company_name"`
	Kind         string `json:"kind"`
	AttachedAt   string `json:"attached_at"`
}

type documentPayload struct {
	SHA256          string `json:"sha256"`
	Size            int64  `json:"size"`
	Mime            string `json:"mime"`
	FirstUploadedAt string `json:"first_uploaded_at"`
	DownloadURL     string `json:"download_url"`
	PDF             struct {
		Title *string `json:"title"`
	} `json:"pdf"`
	Offers []documentReferencePayload `json:"offers"`
}

type documentsEnvelope struct {
	Documents []documentPayload `json:"documents"`
}

func (ts *testServer) uploadAttachment(t *testing.T, offerID int64, kind, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if kind != "" {
		if err := mw.WriteField("kind", kind); err != nil {
			t.Fatal(err)
		}
	}
	part, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/offers/%d/attachments", offerID), &buf)
	req.Host = "127.0.0.1:8080"
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	ts.handler.ServeHTTP(rec, req)
	return rec
}

func countTable(t *testing.T, ts *testServer, query string, args ...any) int {
	t.Helper()
	var n int
	if err := ts.db.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestUploadOfferAttachment(t *testing.T) {
	ts := newTestServer(t)
	company := ts.seedCompany(t, "Acme")
	offer := ts.seedOffer(t, company, "SRE", "", model.AppNone, model.RemoteUnknown)
	content := testPDF("Acme CV")

	rec := ts.uploadAttachment(t, offer.ID, "cv", "../../cv.pdf", content)
	wantCode(t, rec, http.StatusCreated)
	att := decodeBody[attachmentEnvelope](t, rec).Attachment
	if att.ID == 0 || att.OfferID != offer.ID || att.Kind != "cv" || att.Filename != "cv.pdf" {
		t.Fatalf("attachment = %+v", att)
	}
	if att.Size != int64(len(content)) || att.SHA256 != digestOf(content) {
		t.Errorf("size/sha = %d/%s", att.Size, att.SHA256)
	}
	if att.PDF.Title == nil || *att.PDF.Title != "Acme CV" || att.PDF.PageCount == nil || *att.PDF.PageCount != 1 {
		t.Errorf("pdf metadata = %+v", att.PDF)
	}
	if att.PDF.CreationDate == nil || *att.PDF.CreationDate != "2026-01-01T12:00:00Z" {
		t.Errorf("pdf creation date = %v", att.PDF.CreationDate)
	}
	if att.AttachedAt == "" || att.FirstUploadedAt != att.AttachedAt {
		t.Errorf("times = attached %q, first uploaded %q", att.AttachedAt, att.FirstUploadedAt)
	}
	if want := fmt.Sprintf("/api/offers/%d/attachments/%d/file", offer.ID, att.ID); att.DownloadURL != want {
		t.Errorf("download_url = %q, want %q", att.DownloadURL, want)
	}

	// The detail payload lists the document and the timeline stays empty.
	rec = ts.do(t, http.MethodGet, fmt.Sprintf("/api/offers/%d", offer.ID), nil)
	wantCode(t, rec, http.StatusOK)
	detail := decodeBody[offerDetailEnvelope](t, rec)
	if len(detail.Attachments) != 1 || detail.Attachments[0].Filename != "cv.pdf" {
		t.Errorf("detail attachments = %+v", detail.Attachments)
	}
	if detail.Offer.ApplicationState != "none" {
		t.Errorf("application_state after upload = %q, want none", detail.Offer.ApplicationState)
	}
	if n := countTable(t, ts, `SELECT COUNT(*) FROM events WHERE offer_id = ?`, offer.ID); n != 0 {
		t.Errorf("attaching created %d timeline events", n)
	}

	// Non-PDF content is rejected even with a .pdf name.
	rec = ts.uploadAttachment(t, offer.ID, "cv", "fake.pdf", []byte("this is not a PDF"))
	wantErrorCode(t, rec, http.StatusBadRequest, "unsupported_file_type")

	// Oversized content is rejected.
	big := append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte("a"), maxAttachmentBytes)...)
	rec = ts.uploadAttachment(t, offer.ID, "cv", "big.pdf", big)
	wantErrorCode(t, rec, http.StatusRequestEntityTooLarge, "file_too_large")

	// Kind is required and validated.
	rec = ts.uploadAttachment(t, offer.ID, "", "cv.pdf", content)
	wantErrorCode(t, rec, http.StatusBadRequest, "validation_failed")
	rec = ts.uploadAttachment(t, offer.ID, "label", "cv.pdf", content)
	wantErrorCode(t, rec, http.StatusBadRequest, "validation_failed")

	// Unknown offers are not found and store nothing.
	rec = ts.uploadAttachment(t, 99999, "cv", "cv.pdf", testPDF("Other"))
	wantErrorCode(t, rec, http.StatusNotFound, "not_found")

	// Re-uploading the same bytes to the same offer conflicts.
	rec = ts.uploadAttachment(t, offer.ID, "cv", "again.pdf", content)
	wantErrorCode(t, rec, http.StatusConflict, "already_attached")

	if n := countTable(t, ts, `SELECT COUNT(*) FROM offer_attachments`); n != 1 {
		t.Errorf("attachments after rejections = %d, want 1", n)
	}
	if n := countTable(t, ts, `SELECT COUNT(*) FROM attachment_blobs`); n != 1 {
		t.Errorf("blobs after rejections = %d, want 1", n)
	}
}

func TestAttachmentDownloadAndDetach(t *testing.T) {
	ts := newTestServer(t)
	company := ts.seedCompany(t, "Acme")
	first := ts.seedOffer(t, company, "SRE", "", model.AppNone, model.RemoteUnknown)
	second := ts.seedOffer(t, company, "Platform Engineer", "", model.AppNone, model.RemoteUnknown)
	content := testPDF("Shared CV")
	digest := digestOf(content)

	cvEvent := ts.seedEvent(t, first.ID, model.EventCVSent, ts.clock.now(), "sent")
	interviewEvent := ts.seedEvent(t, first.ID, model.EventInterview, ts.clock.now().Add(time.Hour), "call")

	rec := ts.uploadAttachment(t, first.ID, "cv", "cv.pdf", content)
	wantCode(t, rec, http.StatusCreated)
	att := decodeBody[attachmentEnvelope](t, rec).Attachment
	rec = ts.uploadAttachment(t, second.ID, "cv", "cv.pdf", content)
	wantCode(t, rec, http.StatusCreated)

	// Both download routes return the exact bytes as forced attachments.
	for _, path := range []string{
		fmt.Sprintf("/api/offers/%d/attachments/%d/file", first.ID, att.ID),
		"/api/documents/" + digest + "/file",
	} {
		rec = ts.do(t, http.MethodGet, path, nil)
		wantCode(t, rec, http.StatusOK)
		if !bytes.Equal(rec.Body.Bytes(), content) {
			t.Errorf("%s bytes differ from the upload", path)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/pdf" {
			t.Errorf("%s content type = %q", path, got)
		}
		if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment") || !strings.Contains(got, "cv.pdf") {
			t.Errorf("%s disposition = %q", path, got)
		}
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s nosniff = %q", path, got)
		}
	}

	// Unknown ids are 404.
	rec = ts.do(t, http.MethodGet, fmt.Sprintf("/api/offers/%d/attachments/99999/file", first.ID), nil)
	wantErrorCode(t, rec, http.StatusNotFound, "not_found")
	rec = ts.do(t, http.MethodGet, "/api/documents/"+strings.Repeat("0", 64)+"/file", nil)
	wantErrorCode(t, rec, http.StatusNotFound, "not_found")

	// Detaching one reference keeps the shared file available.
	rec = ts.do(t, http.MethodDelete, fmt.Sprintf("/api/offers/%d/attachments/%d", first.ID, att.ID), nil)
	wantCode(t, rec, http.StatusNoContent)
	rec = ts.do(t, http.MethodGet, "/api/documents/"+digest+"/file", nil)
	wantCode(t, rec, http.StatusOK)

	// Detaching leaves the offer's timeline exactly as it was.
	firstDetail := decodeBody[offerDetailEnvelope](t,
		ts.do(t, http.MethodGet, fmt.Sprintf("/api/offers/%d", first.ID), nil))
	if len(firstDetail.Events) != 2 ||
		firstDetail.Events[0].ID != cvEvent.ID ||
		firstDetail.Events[0].Type != string(model.EventCVSent) ||
		firstDetail.Events[0].OccurredAt != model.FormatTime(cvEvent.OccurredAt) ||
		firstDetail.Events[1].ID != interviewEvent.ID ||
		firstDetail.Events[1].Type != string(model.EventInterview) ||
		firstDetail.Events[1].OccurredAt != model.FormatTime(interviewEvent.OccurredAt) {
		t.Errorf("timeline changed by detach: %+v", firstDetail.Events)
	}

	docs := decodeBody[documentsEnvelope](t, ts.do(t, http.MethodGet, "/api/documents", nil))
	if len(docs.Documents) != 1 || len(docs.Documents[0].Offers) != 1 {
		t.Fatalf("documents after first detach = %+v", docs.Documents)
	}
	if docs.Documents[0].Offers[0].OfferID != second.ID {
		t.Errorf("remaining reference = %+v", docs.Documents[0].Offers[0])
	}

	// The last reference removes the file everywhere.
	detail := decodeBody[offerDetailEnvelope](t, ts.do(t, http.MethodGet, fmt.Sprintf("/api/offers/%d", second.ID), nil))
	if len(detail.Attachments) != 1 {
		t.Fatalf("second offer attachments = %+v", detail.Attachments)
	}
	rec = ts.do(t, http.MethodDelete,
		fmt.Sprintf("/api/offers/%d/attachments/%d", second.ID, detail.Attachments[0].ID), nil)
	wantCode(t, rec, http.StatusNoContent)
	rec = ts.do(t, http.MethodGet, "/api/documents/"+digest+"/file", nil)
	wantErrorCode(t, rec, http.StatusNotFound, "not_found")
	docs = decodeBody[documentsEnvelope](t, ts.do(t, http.MethodGet, "/api/documents", nil))
	if len(docs.Documents) != 0 {
		t.Errorf("documents after last detach = %+v", docs.Documents)
	}

	// Detaching again (or through the wrong offer) is 404.
	rec = ts.do(t, http.MethodDelete, fmt.Sprintf("/api/offers/%d/attachments/%d", second.ID, att.ID), nil)
	wantErrorCode(t, rec, http.StatusNotFound, "not_found")
}

func TestDocumentsAggregate(t *testing.T) {
	ts := newTestServer(t)
	company := ts.seedCompany(t, "Acme")
	first := ts.seedOffer(t, company, "SRE", "", model.AppNone, model.RemoteUnknown)
	second := ts.seedOffer(t, company, "Platform Engineer", "", model.AppNone, model.RemoteUnknown)
	shared := testPDF("Shared CV")
	other := testPDF("Cover letter")

	for _, upload := range []struct {
		offer   int64
		kind    string
		name    string
		content []byte
	}{
		{first.ID, "cv", "cv.pdf", shared},
		{second.ID, "cv", "cv-again.pdf", shared},
		{first.ID, "cover_letter", "letter.pdf", other},
	} {
		rec := ts.uploadAttachment(t, upload.offer, upload.kind, upload.name, upload.content)
		wantCode(t, rec, http.StatusCreated)
	}

	docs := decodeBody[documentsEnvelope](t, ts.do(t, http.MethodGet, "/api/documents", nil))
	if len(docs.Documents) != 2 {
		t.Fatalf("documents = %d, want 2", len(docs.Documents))
	}
	var sharedDoc *documentPayload
	for i := range docs.Documents {
		d := &docs.Documents[i]
		if d.SHA256 == digestOf(shared) {
			sharedDoc = d
			if d.PDF.Title == nil || *d.PDF.Title != "Shared CV" {
				t.Errorf("shared pdf title = %v", d.PDF.Title)
			}
			if d.DownloadURL != "/api/documents/"+d.SHA256+"/file" {
				t.Errorf("download_url = %q", d.DownloadURL)
			}
		}
		if d.FirstUploadedAt == "" {
			t.Errorf("document %s has no first upload time", d.SHA256)
		}
		if d.Size != int64(len(shared)) && d.Size != int64(len(other)) {
			t.Errorf("document %s size = %d", d.SHA256, d.Size)
		}
	}
	if sharedDoc == nil {
		t.Fatal("shared document missing from the aggregate")
	}
	if len(sharedDoc.Offers) != 2 {
		t.Fatalf("shared document references = %d, want 2", len(sharedDoc.Offers))
	}
	seen := map[int64]bool{}
	for _, ref := range sharedDoc.Offers {
		seen[ref.OfferID] = true
		if ref.CompanyName != "Acme" || ref.Kind != "cv" || ref.AttachedAt == "" {
			t.Errorf("reference = %+v", ref)
		}
	}
	if !seen[first.ID] || !seen[second.ID] {
		t.Errorf("references cover offers %v", seen)
	}

	// Detaching the last reference of the cover letter removes its row.
	var letterAttachment int64
	for _, d := range docs.Documents {
		if d.SHA256 == digestOf(other) {
			letterAttachment = d.Offers[0].AttachmentID
		}
	}
	if letterAttachment == 0 {
		t.Fatal("cover letter attachment missing")
	}
	rec := ts.do(t, http.MethodDelete, fmt.Sprintf("/api/offers/%d/attachments/%d", first.ID, letterAttachment), nil)
	wantCode(t, rec, http.StatusNoContent)
	docs = decodeBody[documentsEnvelope](t, ts.do(t, http.MethodGet, "/api/documents", nil))
	if len(docs.Documents) != 1 || docs.Documents[0].SHA256 != digestOf(shared) {
		t.Errorf("documents after detach = %+v", docs.Documents)
	}
}

func TestMergeOffersAttachmentsEndToEnd(t *testing.T) {
	ts := newTestServer(t)
	company := ts.seedCompany(t, "Acme")
	target := ts.seedOffer(t, company, "SRE", "", model.AppNone, model.RemoteUnknown)
	source := ts.seedOffer(t, company, "SRE - Platform", "", model.AppNone, model.RemoteUnknown)
	shared := testPDF("Shared CV")
	sourceOnly := testPDF("Source letter")
	targetOnly := testPDF("Target CV")

	for _, upload := range []struct {
		offer   int64
		kind    string
		name    string
		content []byte
	}{
		{target.ID, "cv", "target.pdf", targetOnly},
		{target.ID, "cv", "shared.pdf", shared},
		{source.ID, "cv", "shared.pdf", shared},
		{source.ID, "cover_letter", "source.pdf", sourceOnly},
	} {
		rec := ts.uploadAttachment(t, upload.offer, upload.kind, upload.name, upload.content)
		wantCode(t, rec, http.StatusCreated)
	}

	rec := ts.do(t, http.MethodPost, fmt.Sprintf("/api/offers/%d/merge", source.ID), map[string]any{"into_id": target.ID})
	wantCode(t, rec, http.StatusOK)
	merged := decodeBody[struct {
		MovedAttachments int `json:"moved_attachments"`
	}](t, rec)
	if merged.MovedAttachments != 1 {
		t.Errorf("moved_attachments = %d, want 1", merged.MovedAttachments)
	}

	detail := decodeBody[offerDetailEnvelope](t, ts.do(t, http.MethodGet, fmt.Sprintf("/api/offers/%d", target.ID), nil))
	if len(detail.Attachments) != 3 {
		t.Fatalf("kept offer attachments = %d, want 3", len(detail.Attachments))
	}
	bySHA := map[string]int{}
	for _, a := range detail.Attachments {
		bySHA[a.SHA256]++
	}
	if bySHA[digestOf(shared)] != 1 {
		t.Errorf("shared document appears %d times, want 1", bySHA[digestOf(shared)])
	}
	if bySHA[digestOf(sourceOnly)] != 1 || bySHA[digestOf(targetOnly)] != 1 {
		t.Errorf("documents after merge = %v", bySHA)
	}

	docs := decodeBody[documentsEnvelope](t, ts.do(t, http.MethodGet, "/api/documents", nil))
	if len(docs.Documents) != 3 {
		t.Errorf("documents after merge = %d, want 3", len(docs.Documents))
	}
	for _, d := range docs.Documents {
		for _, ref := range d.Offers {
			if ref.OfferID != target.ID {
				t.Errorf("document %s still references offer %d", d.SHA256, ref.OfferID)
			}
		}
	}

	// The absorbed offer's attachment endpoint is gone.
	rec = ts.do(t, http.MethodGet, fmt.Sprintf("/api/offers/%d", source.ID), nil)
	wantErrorCode(t, rec, http.StatusNotFound, "not_found")

	// The moved document is downloadable end to end.
	for _, a := range detail.Attachments {
		if a.SHA256 != digestOf(sourceOnly) {
			continue
		}
		rec = ts.do(t, http.MethodGet, a.DownloadURL, nil)
		wantCode(t, rec, http.StatusOK)
		if !bytes.Equal(rec.Body.Bytes(), sourceOnly) {
			t.Error("moved document bytes differ")
		}
	}
}
