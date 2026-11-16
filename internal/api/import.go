package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/stepbeta/jobwatch/internal/importmeta"
	"github.com/stepbeta/jobwatch/internal/scan"
)

// importMetadataTimeout caps the page fetch performed for a URL-only
// import; a slow site must not hang the request.
const importMetadataTimeout = 20 * time.Second

// importError is the error shape of a URL import that could not be
// completed automatically. It carries whatever metadata was extracted so
// the UI can prefill its manual form.
type importError struct {
	Error    errorDetail         `json:"error"`
	Metadata importmeta.Metadata `json:"metadata"`
}

func (s *Server) importOffer(w http.ResponseWriter, r *http.Request) {
	var in scan.ImportInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Company = strings.TrimSpace(in.Company)
	in.Title = strings.TrimSpace(in.Title)
	in.Location = strings.TrimSpace(in.Location)
	in.URL = strings.TrimSpace(in.URL)
	in.Description = strings.TrimSpace(in.Description)
	if err := scan.ValidateImportURL(in.URL); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}

	var meta importmeta.Metadata
	var metaErr error
	if in.URL != "" && (in.Company == "" || in.Title == "") {
		ctx, cancel := context.WithTimeout(r.Context(), importMetadataTimeout)
		defer cancel()
		meta, metaErr = importmeta.Extract(ctx, s.httpClient(), in.URL)
		in.FillMissing(meta.Company, meta.Title, meta.Location, meta.URL, meta.Description)
		if in.Company == "" || in.Title == "" {
			code := "metadata_incomplete"
			msg := "could not extract company and title from the page; fill them in and retry"
			if metaErr != nil {
				code = "metadata_unavailable"
				msg = fmt.Sprintf("could not fetch page metadata (%v); fill the fields in and retry", metaErr)
			}
			writeJSON(w, http.StatusUnprocessableEntity, importError{
				Error:    errorDetail{Code: code, Message: msg},
				Metadata: meta,
			})
			return
		}
	}
	if err := in.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	res, err := scan.Import(r.Context(), s.DB, in, s.now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	code := http.StatusOK
	if res.Created {
		code = http.StatusCreated
	}
	writeJSON(w, code, map[string]any{
		"offer_id": res.OfferID,
		"created":  res.Created,
	})
}
