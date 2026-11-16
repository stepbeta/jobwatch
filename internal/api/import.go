package api

import (
	"net/http"

	"github.com/stepbeta/jobwatch/internal/scan"
)

func (s *Server) importOffer(w http.ResponseWriter, r *http.Request) {
	var in scan.ImportInput
	if !decodeJSON(w, r, &in) {
		return
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
