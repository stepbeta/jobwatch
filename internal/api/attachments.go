package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/internal/pdfmeta"
)

const (
	// maxAttachmentBytes is the hard cap on one uploaded PDF.
	maxAttachmentBytes = 2 << 20 // 2 MiB
	// multipartOverhead leaves room for the multipart boundaries and the
	// small kind field above the file itself.
	multipartOverhead = 64 << 10
)

type pdfMetadataJSON struct {
	Title            *string `json:"title"`
	Author           *string `json:"author"`
	Producer         *string `json:"producer"`
	CreationDate     *string `json:"creation_date"`
	ModificationDate *string `json:"modification_date"`
	PageCount        *int    `json:"page_count"`
	Version          *string `json:"version"`
}

func pdfMetadataToJSON(m model.PDFMetadata) pdfMetadataJSON {
	out := pdfMetadataJSON{
		Title:            m.Title,
		Author:           m.Author,
		Producer:         m.Producer,
		CreationDate:     timeStringPtr(m.CreationDate),
		ModificationDate: timeStringPtr(m.ModificationDate),
	}
	if m.PageCount > 0 {
		pages := m.PageCount
		out.PageCount = &pages
	}
	if m.Version != "" {
		version := m.Version
		out.Version = &version
	}
	return out
}

type attachmentJSON struct {
	ID              int64           `json:"id"`
	OfferID         int64           `json:"offer_id"`
	SHA256          string          `json:"sha256"`
	Kind            string          `json:"kind"`
	Filename        string          `json:"filename"`
	Size            int64           `json:"size"`
	Mime            string          `json:"mime"`
	AttachedAt      string          `json:"attached_at"`
	FirstUploadedAt string          `json:"first_uploaded_at"`
	PDF             pdfMetadataJSON `json:"pdf"`
	DownloadURL     string          `json:"download_url"`
}

func attachmentToJSON(a db.Attachment) attachmentJSON {
	return attachmentJSON{
		ID:              a.ID,
		OfferID:         a.OfferID,
		SHA256:          a.SHA256,
		Kind:            string(a.Kind),
		Filename:        a.Filename,
		Size:            a.Size,
		Mime:            a.Mime,
		AttachedAt:      model.FormatTime(a.AttachedAt),
		FirstUploadedAt: model.FormatTime(a.FirstUploadedAt),
		PDF:             pdfMetadataToJSON(a.PDF),
		DownloadURL:     fmt.Sprintf("/api/offers/%d/attachments/%d/file", a.OfferID, a.ID),
	}
}

type documentReferenceJSON struct {
	AttachmentID int64  `json:"attachment_id"`
	OfferID      int64  `json:"offer_id"`
	OfferTitle   string `json:"offer_title"`
	CompanyName  string `json:"company_name"`
	Kind         string `json:"kind"`
	AttachedAt   string `json:"attached_at"`
}

type documentJSON struct {
	SHA256          string                  `json:"sha256"`
	Size            int64                   `json:"size"`
	Mime            string                  `json:"mime"`
	FirstUploadedAt string                  `json:"first_uploaded_at"`
	PDF             pdfMetadataJSON         `json:"pdf"`
	DownloadURL     string                  `json:"download_url"`
	Offers          []documentReferenceJSON `json:"offers"`
}

func documentToJSON(d db.Document) documentJSON {
	out := documentJSON{
		SHA256:          d.SHA256,
		Size:            d.Size,
		Mime:            d.Mime,
		FirstUploadedAt: model.FormatTime(d.FirstUploadedAt),
		PDF:             pdfMetadataToJSON(d.PDF),
		DownloadURL:     "/api/documents/" + d.SHA256 + "/file",
		Offers:          make([]documentReferenceJSON, 0, len(d.References)),
	}
	for _, ref := range d.References {
		out.Offers = append(out.Offers, documentReferenceJSON{
			AttachmentID: ref.AttachmentID,
			OfferID:      ref.OfferID,
			OfferTitle:   ref.OfferTitle,
			CompanyName:  ref.CompanyName,
			Kind:         string(ref.Kind),
			AttachedAt:   model.FormatTime(ref.AttachedAt),
		})
	}
	return out
}

// safeFilename keeps only the client's basename, treating both slash
// styles as separators, and falls back to a neutral name.
func safeFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(strings.TrimSpace(name))
	switch name {
	case "", ".", "/":
		return "document.pdf"
	}
	return name
}

func isMaxBytesError(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

// uploadOfferAttachment accepts one multipart PDF up to maxAttachmentBytes.
// The magic bytes decide the type; the file extension never does.
func (s *Server) uploadOfferAttachment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "offer id must be a positive integer")
		return
	}
	ctx := r.Context()
	if _, err := db.GetOffer(ctx, s.DB, id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("offer %d not found", id))
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAttachmentBytes+multipartOverhead)
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed",
			"request body must be multipart/form-data with a file part")
		return
	}

	var content []byte
	var filename, kindField string
	var haveFile bool
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if isMaxBytesError(err) {
				writeError(w, http.StatusRequestEntityTooLarge, "file_too_large",
					fmt.Sprintf("file exceeds the %d byte limit", maxAttachmentBytes))
			} else {
				writeError(w, http.StatusBadRequest, "validation_failed",
					"malformed multipart body: "+err.Error())
			}
			return
		}
		switch part.FormName() {
		case "file":
			if !haveFile {
				b, err := io.ReadAll(io.LimitReader(part, maxAttachmentBytes+1))
				if err != nil {
					_ = part.Close()
					if isMaxBytesError(err) {
						writeError(w, http.StatusRequestEntityTooLarge, "file_too_large",
							fmt.Sprintf("file exceeds the %d byte limit", maxAttachmentBytes))
					} else {
						writeError(w, http.StatusBadRequest, "validation_failed",
							"could not read the uploaded file: "+err.Error())
					}
					return
				}
				if len(b) > maxAttachmentBytes {
					_ = part.Close()
					writeError(w, http.StatusRequestEntityTooLarge, "file_too_large",
						fmt.Sprintf("file exceeds the %d byte limit", maxAttachmentBytes))
					return
				}
				content = b
				filename = part.FileName()
				haveFile = true
			} else {
				_, _ = io.Copy(io.Discard, part)
			}
		case "kind":
			b, err := io.ReadAll(io.LimitReader(part, 256))
			if err != nil {
				_ = part.Close()
				writeError(w, http.StatusBadRequest, "validation_failed",
					"could not read the kind field: "+err.Error())
				return
			}
			kindField = strings.TrimSpace(string(b))
		default:
			_, _ = io.Copy(io.Discard, part)
		}
		_ = part.Close()
	}

	if !haveFile {
		writeError(w, http.StatusBadRequest, "validation_failed", "a file part is required")
		return
	}
	kind := model.AttachmentKind(kindField)
	if !kind.Valid() {
		writeError(w, http.StatusBadRequest, "validation_failed",
			"kind must be one of cv, cover_letter or other")
		return
	}
	if !bytes.HasPrefix(content, []byte("%PDF-")) {
		writeError(w, http.StatusBadRequest, "unsupported_file_type",
			"only PDF files are accepted (the content must start with %PDF-)")
		return
	}

	att, err := db.AttachOfferDocument(ctx, s.DB, id, kind, safeFilename(filename), content,
		pdfmeta.Extract(content), s.now())
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("offer %d not found", id))
		return
	case errors.Is(err, db.ErrAttachmentExists):
		writeError(w, http.StatusConflict, "already_attached",
			"this offer already references that file")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"attachment": attachmentToJSON(att)})
}

// deleteOfferAttachment detaches one document and lets the blob be removed
// when nothing else references it.
func (s *Server) deleteOfferAttachment(w http.ResponseWriter, r *http.Request) {
	offerID, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "offer id must be a positive integer")
		return
	}
	attachmentID, ok := pathID(r, "attachmentID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "attachment id must be a positive integer")
		return
	}
	err := db.DetachOfferAttachment(r.Context(), s.DB, offerID, attachmentID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found",
			fmt.Sprintf("attachment %d not found on offer %d", attachmentID, offerID))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// downloadOfferAttachment streams one attachment as a forced download.
func (s *Server) downloadOfferAttachment(w http.ResponseWriter, r *http.Request) {
	offerID, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "offer id must be a positive integer")
		return
	}
	attachmentID, ok := pathID(r, "attachmentID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "attachment id must be a positive integer")
		return
	}
	file, err := db.GetOfferAttachmentFile(r.Context(), s.DB, offerID, attachmentID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found",
			fmt.Sprintf("attachment %d not found on offer %d", attachmentID, offerID))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeStoredFile(w, file)
}

// listDocuments returns the read-only Documents aggregate: one entry per
// stored file with every offer that references it.
func (s *Server) listDocuments(w http.ResponseWriter, r *http.Request) {
	docs, err := db.ListDocuments(r.Context(), s.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]documentJSON, 0, len(docs))
	for _, d := range docs {
		out = append(out, documentToJSON(d))
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": out})
}

// downloadDocument streams a stored file by content hash.
func (s *Server) downloadDocument(w http.ResponseWriter, r *http.Request) {
	digest := strings.ToLower(strings.TrimSpace(r.PathValue("sha256")))
	file, err := db.GetBlobFile(r.Context(), s.DB, digest)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("document %s not found", digest))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeStoredFile(w, file)
}

// writeStoredFile serves bytes as an attachment, never inline, so an
// unexpected content type cannot execute in the application's origin.
func writeStoredFile(w http.ResponseWriter, file db.StoredFile) {
	mimeType := file.Mime
	if mimeType == "" {
		mimeType = "application/pdf"
	}
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": file.Filename})
	if disposition == "" {
		disposition = `attachment; filename="document.pdf"`
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Bytes)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Bytes)
}
