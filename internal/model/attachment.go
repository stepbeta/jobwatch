package model

import "time"

// AttachmentKind is the role a document plays on one offer. The same bytes
// may carry different kinds on different offers; the tag belongs to the
// attachment, not to the stored file.
type AttachmentKind string

// Attachment kinds accepted by the API and stored in offer_attachments.kind.
const (
	AttachmentCV          AttachmentKind = "cv"
	AttachmentCoverLetter AttachmentKind = "cover_letter"
	AttachmentOther       AttachmentKind = "other"
)

// Valid reports whether k is one of the accepted attachment kinds.
func (k AttachmentKind) Valid() bool {
	switch k {
	case AttachmentCV, AttachmentCoverLetter, AttachmentOther:
		return true
	}
	return false
}

// PDFMetadata is the best-effort intrinsic metadata read from a stored PDF.
// Every field is optional: a missing or unreadable value stays nil (or 0 for
// PageCount) and never fails an upload.
type PDFMetadata struct {
	Title            *string
	Author           *string
	Producer         *string
	CreationDate     *time.Time
	ModificationDate *time.Time
	// PageCount is the number of page objects found; 0 means unknown.
	PageCount int
	// Version is the PDF header version (for example "1.7"); empty means
	// unknown.
	Version string
}
