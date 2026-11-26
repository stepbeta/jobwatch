package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/stepbeta/jobwatch/internal/classify"
	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/diff"
	"github.com/stepbeta/jobwatch/internal/model"
)

const (
	defaultOfferLimit = 50
	maxOfferLimit     = 200
)

// Affinity sort values are applied in Go because the score is computed
// at read time, not stored in SQL.
const (
	sortTitleAffinity    = "title_affinity"
	sortLocationAffinity = "location_affinity"
)

// offerListQuery is the full offers-list request. Fact filters, non-score
// sorts and pagination go to SQL; the affinity filters and sorts are
// applied in Go after the facts are loaded.
type offerListQuery struct {
	Filter              db.OfferQuery
	ProfileID           *int64
	MinTitleAffinity    *int
	MinLocationAffinity *int
	RemoteAccepted      *bool
	Sort                string
	Order               string
	Limit               int
	Offset              int
}

// hasScoreWork reports whether the request needs every matching fact row
// loaded, so affinity can be computed before filtering, sorting and
// paginating.
func (q offerListQuery) hasScoreWork() bool {
	return q.MinTitleAffinity != nil || q.MinLocationAffinity != nil ||
		q.RemoteAccepted != nil || q.Sort == sortTitleAffinity || q.Sort == sortLocationAffinity
}

// parseOfferQuery builds an offerListQuery from the request parameters.
func parseOfferQuery(r *http.Request) (offerListQuery, error) {
	var q offerListQuery
	f := &q.Filter
	f.Search = strings.TrimSpace(r.URL.Query().Get("q"))

	apps, err := queryEnumList(r, "application_state", model.ApplicationState.Valid)
	if err != nil {
		return q, err
	}
	f.ApplicationStates = apps
	offers, err := queryEnumList(r, "offer_state", model.OfferState.Valid)
	if err != nil {
		return q, err
	}
	f.OfferStates = offers
	remotes, err := queryEnumList(r, "remote_policy", model.RemotePolicy.Valid)
	if err != nil {
		return q, err
	}
	f.RemotePolicies = remotes
	if q.ProfileID, err = queryInt64(r, "profile_id"); err != nil {
		return q, err
	}
	if q.MinTitleAffinity, err = queryInt(r, "min_title_affinity", 0, 100); err != nil {
		return q, err
	}
	if q.MinLocationAffinity, err = queryInt(r, "min_location_affinity", 0, 100); err != nil {
		return q, err
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("remote_accepted")); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return q, fmt.Errorf("remote_accepted must be true or false")
		}
		q.RemoteAccepted = &v
	}
	if f.CompanyID, err = queryInt64(r, "company_id"); err != nil {
		return q, err
	}
	f.HasApplication = queryBool(r, "has_application")
	if raw := strings.TrimSpace(r.URL.Query().Get("incomplete")); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return q, fmt.Errorf("incomplete must be true or false")
		}
		f.IncompleteOnly = v
	}

	q.Sort = strings.TrimSpace(r.URL.Query().Get("sort"))
	if q.Sort == "" {
		q.Sort = db.SortLastSeen
	}
	f.Sort = q.Sort
	if q.Sort == sortTitleAffinity || q.Sort == sortLocationAffinity {
		// The Go sort below is authoritative; SQL only needs a stable base.
		f.Sort = db.SortLastSeen
	} else if !db.ValidOfferSort(q.Sort) {
		return q, fmt.Errorf("sort must be one of last_seen, first_seen, recent_event, title, company, location, title_affinity, location_affinity, saved")
	}
	q.Order = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("order")))
	if !db.ValidOfferOrder(q.Order) {
		return q, fmt.Errorf("order must be asc or desc")
	}
	f.Order = q.Order
	q.Limit = defaultOfferLimit
	if v, err := queryInt(r, "limit", 1, maxOfferLimit); err != nil {
		return q, err
	} else if v != nil {
		q.Limit = *v
	}
	if v, err := queryInt(r, "offset", 0, 1_000_000); err != nil {
		return q, err
	} else if v != nil {
		q.Offset = *v
	}
	f.Limit = q.Limit
	f.Offset = q.Offset
	return q, nil
}

// scoredOffer is one fact row with its read-time affinity.
type scoredOffer struct {
	item db.OfferListItem
	res  classify.Result
}

// profile returns the profile named by id, or the default profile when id
// is nil.
func (s *Server) profile(ctx context.Context, id *int64) (model.SearchProfile, error) {
	if id != nil {
		return db.GetSearchProfile(ctx, s.DB, *id)
	}
	return db.DefaultSearchProfile(ctx, s.DB)
}

// writeProfileError maps a profile lookup failure to an API response.
func (s *Server) writeProfileError(w http.ResponseWriter, id *int64, err error) {
	if errors.Is(err, db.ErrNotFound) {
		if id != nil {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("search profile %d not found", *id))
			return
		}
		writeError(w, http.StatusNotFound, "not_found", "no default search profile")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal", err.Error())
}

func (s *Server) listOffers(w http.ResponseWriter, r *http.Request) {
	q, err := parseOfferQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	ctx := r.Context()
	profile, err := s.profile(ctx, q.ProfileID)
	if err != nil {
		s.writeProfileError(w, q.ProfileID, err)
		return
	}

	// Fast path: no affinity filter or sort, so SQL filters, orders and
	// paginates and the total is the SQL count.
	if !q.hasScoreWork() {
		res, err := db.QueryOffers(ctx, s.DB, q.Filter)
		if errors.Is(err, db.ErrInvalidQuery) {
			writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		out := make([]offerJSON, 0, len(res.Offers))
		for _, o := range res.Offers {
			out = append(out, offerToJSON(o, profile))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"offers": out,
			"total":  res.Total,
			"limit":  q.Limit,
			"offset": q.Offset,
		})
		return
	}

	// Score path: load every fact row that passes the SQL filters, compute
	// affinity once, then filter, sort and paginate in Go.
	q.Filter.Limit = 0
	q.Filter.Offset = 0
	res, err := db.QueryOffers(ctx, s.DB, q.Filter)
	if errors.Is(err, db.ErrInvalidQuery) {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	scored := make([]scoredOffer, 0, len(res.Offers))
	for _, o := range res.Offers {
		scored = append(scored, scoredOffer{
			item: o,
			res:  classify.Score(o.Title, derefString(o.Location), derefString(o.RawText), profile),
		})
	}
	filtered := scored[:0]
	for _, o := range scored {
		if q.MinTitleAffinity != nil && o.res.TitleAffinity.Score < *q.MinTitleAffinity {
			continue
		}
		if q.MinLocationAffinity != nil && o.res.LocationAffinity.Score < *q.MinLocationAffinity {
			continue
		}
		if q.RemoteAccepted != nil && profile.Accepts(o.item.RemotePolicy) != *q.RemoteAccepted {
			continue
		}
		filtered = append(filtered, o)
	}
	scored = filtered
	sortScoredOffers(scored, q.Sort, q.Order)

	total := len(scored)
	start := q.Offset
	if start > total {
		start = total
	}
	end := total
	if q.Limit > 0 && start+q.Limit < end {
		end = start + q.Limit
	}
	out := make([]offerJSON, 0, end-start)
	for _, o := range scored[start:end] {
		out = append(out, offerToJSON(o.item, profile))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"offers": out,
		"total":  total,
		"limit":  q.Limit,
		"offset": q.Offset,
	})
}

// sortScoredOffers orders by the requested affinity. The default
// direction is descending, like the other score sorts used to be. Ties
// break on offer id in the same direction so paging is stable.
func sortScoredOffers(out []scoredOffer, sortBy, order string) {
	if sortBy != sortTitleAffinity && sortBy != sortLocationAffinity {
		return
	}
	desc := order != db.OrderAsc
	sort.SliceStable(out, func(i, j int) bool {
		vi, vj := out[i].res.LocationAffinity.Score, out[j].res.LocationAffinity.Score
		if sortBy == sortTitleAffinity {
			vi, vj = out[i].res.TitleAffinity.Score, out[j].res.TitleAffinity.Score
		}
		if vi != vj {
			if desc {
				return vi > vj
			}
			return vi < vj
		}
		if desc {
			return out[i].item.ID > out[j].item.ID
		}
		return out[i].item.ID < out[j].item.ID
	})
}

func (s *Server) getOffer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "offer id must be a positive integer")
		return
	}
	ctx := r.Context()
	snapshotID, err := queryInt64(r, "snapshot_id")
	if err != nil || (snapshotID != nil && *snapshotID <= 0) {
		writeError(w, http.StatusBadRequest, "validation_failed", "snapshot_id must be a positive integer")
		return
	}
	profileID, err := queryInt64(r, "profile_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	profile, err := s.profile(ctx, profileID)
	if err != nil {
		s.writeProfileError(w, profileID, err)
		return
	}
	o, err := db.GetOffer(ctx, s.DB, id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("offer %d not found", id))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	var companyOut any
	companyName := ""
	if o.CompanyID != nil {
		company, err := db.GetCompany(ctx, s.DB, *o.CompanyID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		companyName = company.Name
		out := companyToJSON(company)
		if out.OfferCount, out.NoteCount, err = db.CompanyCounts(ctx, s.DB, company.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		companyOut = out
	}
	prov, err := db.ListProvenanceDetailed(ctx, s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	events, err := db.ListEvents(ctx, s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	notes, err := db.ListNotes(ctx, s.DB, model.EntityOffer, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	attachments, err := db.ListOfferAttachments(ctx, s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	snapCount, err := db.CountSnapshots(ctx, s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	var rawTextID int64
	if snapshotID != nil {
		rawTextID = *snapshotID
	}
	infos, err := db.ListSnapshotInfos(ctx, s.DB, id, rawTextID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if snapshotID != nil {
		found := false
		for _, s := range infos {
			if s.ID == *snapshotID {
				found = true
				break
			}
		}
		if !found {
			writeError(w, http.StatusNotFound, "not_found",
				fmt.Sprintf("snapshot %d not found on offer %d", *snapshotID, id))
			return
		}
	}
	snaps, err := db.LatestSnapshots(ctx, s.DB, id, 2)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	var snapshot, snapshotDiff any
	if len(snaps) > 0 {
		snapshot = snapshotToJSON(snaps[0])
	}
	if len(snaps) > 1 {
		snapshotDiff = diffToJSON(snaps[1], snaps[0])
	}
	snapshotsOut := make([]snapshotSummaryJSON, 0, len(infos))
	for _, s := range infos {
		snapshotsOut = append(snapshotsOut, snapshotInfoToJSON(s))
	}

	provOut := make([]provenanceJSON, 0, len(prov))
	for _, p := range prov {
		provOut = append(provOut, provenanceToJSON(p))
	}
	eventsOut := make([]eventJSON, 0, len(events))
	for _, e := range events {
		eventsOut = append(eventsOut, eventToJSON(e))
	}
	notesOut := make([]noteJSON, 0, len(notes))
	for _, n := range notes {
		notesOut = append(notesOut, noteToJSON(n))
	}
	attachmentsOut := make([]attachmentJSON, 0, len(attachments))
	for _, a := range attachments {
		attachmentsOut = append(attachmentsOut, attachmentToJSON(a))
	}
	offerOut := offerModelToJSON(o, companyName, profile)
	offerOut.RawText = o.RawText
	if len(events) > 0 {
		offerOut.LastEventAt = timeString(events[len(events)-1].OccurredAt)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"offer":           offerOut,
		"company":         companyOut,
		"provenance":      provOut,
		"events":          eventsOut,
		"notes":           notesOut,
		"attachments":     attachmentsOut,
		"snapshots":       snapshotsOut,
		"latest_snapshot": snapshot,
		"snapshot_diff":   snapshotDiff,
		"snapshot_count":  snapCount,
	})
}

// offerCompanyName returns the display name of the offer's company, or an
// empty string for an incomplete offer with no company yet.
func (s *Server) offerCompanyName(ctx context.Context, o model.JobOffer) (string, error) {
	if o.CompanyID == nil {
		return "", nil
	}
	c, err := db.GetCompany(ctx, s.DB, *o.CompanyID)
	if err != nil {
		return "", err
	}
	return c.Name, nil
}

type offerPatch struct {
	ApplicationState *string         `json:"application_state"`
	OfferState       *string         `json:"offer_state"`
	Location         json.RawMessage `json:"location"`
	Description      json.RawMessage `json:"description"`
}

func (s *Server) patchOffer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "offer id must be a positive integer")
		return
	}
	var in offerPatch
	if !decodeJSON(w, r, &in) {
		return
	}
	var u db.OfferUpdate
	if in.ApplicationState != nil {
		st := model.ApplicationState(strings.TrimSpace(*in.ApplicationState))
		if !st.Valid() {
			writeError(w, http.StatusBadRequest, "validation_failed", fmt.Sprintf("invalid application_state %q", st))
			return
		}
		u.ApplicationState = &st
	}
	if in.OfferState != nil {
		st := model.OfferState(strings.TrimSpace(*in.OfferState))
		if !st.Valid() {
			writeError(w, http.StatusBadRequest, "validation_failed", fmt.Sprintf("invalid offer_state %q", st))
			return
		}
		u.OfferState = &st
	}
	if len(in.Location) > 0 {
		var loc string
		if err := json.Unmarshal(in.Location, &loc); err != nil {
			if string(in.Location) != "null" {
				writeError(w, http.StatusBadRequest, "validation_failed", "location must be a string or null")
				return
			}
			loc = ""
		}
		loc = strings.TrimSpace(loc)
		u.Location = &loc
	}
	if len(in.Description) > 0 {
		if string(in.Description) == "null" {
			writeError(w, http.StatusBadRequest, "validation_failed",
				"description cannot be null; send a string to replace it")
			return
		}
		var desc string
		if err := json.Unmarshal(in.Description, &desc); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", "description must be a string")
			return
		}
		desc = strings.TrimSpace(desc)
		u.Description = &desc
	}
	if u.ApplicationState == nil && u.OfferState == nil && u.Location == nil && u.Description == nil {
		writeError(w, http.StatusBadRequest, "validation_failed",
			"nothing to update: provide application_state, offer_state, location or description")
		return
	}

	ctx := r.Context()
	if err := db.UpdateOfferFields(ctx, s.DB, id, u, s.now()); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("offer %d not found", id))
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	o, err := db.GetOffer(ctx, s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	companyName, err := s.offerCompanyName(ctx, o)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	profile, err := db.DefaultSearchProfile(ctx, s.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	offerOut := offerModelToJSON(o, companyName, profile)
	offerOut.RawText = o.RawText
	writeJSON(w, http.StatusOK, map[string]any{"offer": offerOut})
}

type offerRepair struct {
	CompanyID int64   `json:"company_id"`
	Title     string  `json:"title"`
	Location  *string `json:"location"`
}

// repairOffer assigns a company and title to an incomplete offer. The raw
// reported values stay in incomplete_raw, which later scans use to avoid
// overwriting the user's title and location.
func (s *Server) repairOffer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "offer id must be a positive integer")
		return
	}
	var in offerRepair
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.CompanyID <= 0 || in.Title == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "company_id and title are required")
		return
	}
	var location *string
	if in.Location != nil {
		if v := strings.TrimSpace(*in.Location); v != "" {
			location = &v
		}
	}
	ctx := r.Context()
	o, err := db.RepairOffer(ctx, s.DB, id, in.CompanyID, in.Title, location, s.now())
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "offer or company not found")
		return
	case errors.Is(err, db.ErrOfferNotIncomplete):
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	companyName, err := s.offerCompanyName(ctx, o)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	profile, err := db.DefaultSearchProfile(ctx, s.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"offer": offerModelToJSON(o, companyName, profile)})
}

func diffToJSON(from, to model.Snapshot) snapshotDiffJSON {
	lines := diff.Lines(from.RawText, to.RawText)
	out := snapshotDiffJSON{
		From:  snapshotRefJSON{ID: from.ID, CapturedAt: model.FormatTime(from.CapturedAt)},
		To:    snapshotRefJSON{ID: to.ID, CapturedAt: model.FormatTime(to.CapturedAt)},
		Lines: make([]diffLineJSON, 0, len(lines)),
	}
	for _, line := range lines {
		out.Lines = append(out.Lines, diffLineJSON{Op: string(line.Op), Text: line.Text})
	}
	return out
}

// snapshotDiff returns the line diff between any two snapshots of an
// offer. The pair is ordered oldest -> newest server-side.
func (s *Server) snapshotDiff(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "offer id must be a positive integer")
		return
	}
	fromID, err := queryInt64(r, "from")
	if err != nil || fromID == nil || *fromID <= 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", "from must be a positive integer")
		return
	}
	toID, err := queryInt64(r, "to")
	if err != nil || toID == nil || *toID <= 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", "to must be a positive integer")
		return
	}
	if *fromID == *toID {
		writeError(w, http.StatusBadRequest, "validation_failed", "from and to must be different snapshots")
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
	from, err := db.GetSnapshotForOffer(ctx, s.DB, id, *fromID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found",
			fmt.Sprintf("snapshot %d not found on offer %d", *fromID, id))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	to, err := db.GetSnapshotForOffer(ctx, s.DB, id, *toID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found",
			fmt.Sprintf("snapshot %d not found on offer %d", *toID, id))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	older, newer := from, to
	if newer.CapturedAt.Before(older.CapturedAt) ||
		(newer.CapturedAt.Equal(older.CapturedAt) && newer.ID < older.ID) {
		older, newer = newer, older
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshot_diff": diffToJSON(older, newer)})
}

type mergeOfferInput struct {
	IntoID int64 `json:"into_id"`
}

// mergeOffer merges :id into into_id after an explicit user confirmation.
func (s *Server) mergeOffer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "offer id must be a positive integer")
		return
	}
	var in mergeOfferInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.IntoID <= 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", "into_id must be a positive integer")
		return
	}
	ctx := r.Context()
	res, err := db.MergeOffers(ctx, s.DB, id, in.IntoID, s.now())
	switch {
	case errors.Is(err, db.ErrOfferMergeSelf):
		writeError(w, http.StatusBadRequest, "validation_failed", "an offer cannot be merged into itself")
		return
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "one of the offers does not exist")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	companyName, err := s.offerCompanyName(ctx, res.Offer)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	profile, err := db.DefaultSearchProfile(ctx, s.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"offer":             offerModelToJSON(res.Offer, companyName, profile),
		"merged_offer_id":   res.MergedOfferID,
		"moved_provenance":  res.MovedProvenance,
		"moved_events":      res.MovedEvents,
		"moved_notes":       res.MovedNotes,
		"moved_snapshots":   res.MovedSnapshots,
		"moved_attachments": res.MovedAttachments,
	})
}

type splitOfferInput struct {
	ProvenanceIDs []int64         `json:"provenance_ids"`
	Title         string          `json:"title"`
	Location      json.RawMessage `json:"location"`
}

// splitOffer moves selected provenance rows to a new offer in the same
// company. location is optional: omitted copies the original location,
// null clears it.
func (s *Server) splitOffer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "offer id must be a positive integer")
		return
	}
	var in splitOfferInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if len(in.ProvenanceIDs) == 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", "provenance_ids must contain at least one row")
		return
	}
	var location *string
	if len(in.Location) > 0 {
		if string(in.Location) == "null" {
			empty := ""
			location = &empty
		} else {
			var value string
			if err := json.Unmarshal(in.Location, &value); err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", "location must be a string or null")
				return
			}
			value = strings.TrimSpace(value)
			location = &value
		}
	}
	ctx := r.Context()
	res, err := db.SplitOffer(ctx, s.DB, id, db.OfferSplitInput{
		ProvenanceIDs: in.ProvenanceIDs,
		Title:         in.Title,
		Location:      location,
	}, s.now())
	switch {
	case errors.Is(err, db.ErrOfferSplitNoProvenance),
		errors.Is(err, db.ErrOfferSplitProvenance),
		errors.Is(err, db.ErrOfferSplitLastLink):
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("offer %d not found", id))
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	companyName, err := s.offerCompanyName(ctx, res.Offer)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	profile, err := db.DefaultSearchProfile(ctx, s.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"offer":            offerModelToJSON(res.Offer, companyName, profile),
		"moved_provenance": res.MovedProvenance,
	})
}
