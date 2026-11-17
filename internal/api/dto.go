package api

import (
	"encoding/json"
	"time"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
)

type sourceJSON struct {
	ID             int64           `json:"id"`
	Name           string          `json:"name"`
	URL            string          `json:"url"`
	SourceType     string          `json:"source_type"`
	Config         json.RawMessage `json:"config"`
	Enabled        bool            `json:"enabled"`
	LastScanAt     *string         `json:"last_scan_at"`
	LastScanStatus *string         `json:"last_scan_status"`
	CreatedAt      *string         `json:"created_at"`
}

func sourceToJSON(s model.Source) sourceJSON {
	cfg := json.RawMessage(s.ConfigJSON)
	if len(cfg) == 0 || !json.Valid(cfg) {
		cfg = json.RawMessage("{}")
	}
	out := sourceJSON{
		ID:         s.ID,
		Name:       s.Name,
		URL:        s.URL,
		SourceType: string(s.Type),
		Config:     cfg,
		Enabled:    s.Enabled,
		CreatedAt:  timeString(s.CreatedAt),
	}
	if s.LastScanAt != nil {
		out.LastScanAt = timeString(*s.LastScanAt)
	}
	if s.LastScanStatus != nil {
		st := string(*s.LastScanStatus)
		out.LastScanStatus = &st
	}
	return out
}

type companyJSON struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	NameNormalized string  `json:"name_normalized"`
	Website        *string `json:"website"`
	CreatedAt      *string `json:"created_at"`
	OfferCount     int     `json:"offer_count"`
	NoteCount      int     `json:"note_count"`
}

func companyToJSON(c model.Company) companyJSON {
	return companyJSON{
		ID:             c.ID,
		Name:           c.Name,
		NameNormalized: c.NameNormalized,
		Website:        c.Website,
		CreatedAt:      timeString(c.CreatedAt),
	}
}

func companyItemToJSON(c db.CompanyListItem) companyJSON {
	out := companyToJSON(c.Company)
	out.OfferCount = c.OfferCount
	out.NoteCount = c.NoteCount
	return out
}

type companyCandidateJSON struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	NameNormalized string `json:"name_normalized"`
	Score          int    `json:"score"`
	Reason         string `json:"reason"`
}

type offerJSON struct {
	ID                 int64           `json:"id"`
	CompanyID          int64           `json:"company_id"`
	CompanyName        string          `json:"company_name"`
	Title              string          `json:"title"`
	Location           *string         `json:"location"`
	RemotePolicy       string          `json:"remote_policy"`
	URL                *string         `json:"url"`
	OfferState         string          `json:"offer_state"`
	ApplicationState   string          `json:"application_state"`
	FirstSeenAt        string          `json:"first_seen_at"`
	LastSeenAt         string          `json:"last_seen_at"`
	UpdatedAt          string          `json:"updated_at"`
	TitleConfidence    *int            `json:"title_confidence"`
	LocationConfidence *int            `json:"location_confidence"`
	MatchReasons       json.RawMessage `json:"match_reasons"`
	LastEventAt        *string         `json:"last_event_at"`
	SavedAt            *string         `json:"saved_at"`
	RawText            *string         `json:"raw_text,omitempty"`
}

func offerToJSON(o db.OfferListItem) offerJSON {
	out := offerModelToJSON(o.JobOffer, o.CompanyName)
	if o.LastEventAt != nil {
		out.LastEventAt = timeString(*o.LastEventAt)
	}
	return out
}

func offerModelToJSON(o model.JobOffer, companyName string) offerJSON {
	return offerJSON{
		ID:                 o.ID,
		CompanyID:          o.CompanyID,
		CompanyName:        companyName,
		Title:              o.Title,
		Location:           o.Location,
		RemotePolicy:       string(o.RemotePolicy),
		URL:                o.URL,
		OfferState:         string(o.OfferState),
		ApplicationState:   string(o.ApplicationState),
		FirstSeenAt:        model.FormatTime(o.FirstSeenAt),
		LastSeenAt:         model.FormatTime(o.LastSeenAt),
		UpdatedAt:          model.FormatTime(o.UpdatedAt),
		TitleConfidence:    o.TitleConfidence,
		LocationConfidence: o.LocationConfidence,
		MatchReasons:       rawJSON(o.MatchReasonsJSON),
		SavedAt:            timeStringPtr(o.SavedAt),
	}
}

// timeStringPtr formats a nullable time, returning nil for nil or zero.
func timeStringPtr(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	s := model.FormatTime(*t)
	return &s
}

// rawJSON returns valid stored JSON or null.
func rawJSON(s *string) json.RawMessage {
	if s == nil || !json.Valid([]byte(*s)) {
		return nil
	}
	return json.RawMessage(*s)
}

type provenanceJSON struct {
	ID          int64   `json:"id"`
	SourceID    *int64  `json:"source_id"`
	SourceName  *string `json:"source_name"`
	ExternalUID *string `json:"external_uid"`
	URL         string  `json:"url"`
	FirstSeenAt *string `json:"first_seen_at"`
	LastSeenAt  *string `json:"last_seen_at"`
}

func provenanceToJSON(p db.ProvenanceItem) provenanceJSON {
	return provenanceJSON{
		ID:          p.ID,
		SourceID:    p.SourceID,
		SourceName:  p.SourceName,
		ExternalUID: p.ExternalUID,
		URL:         p.URL,
		FirstSeenAt: timeString(p.FirstSeenAt),
		LastSeenAt:  timeString(p.LastSeenAt),
	}
}

type eventJSON struct {
	ID          int64   `json:"id"`
	OfferID     int64   `json:"offer_id"`
	Type        string  `json:"type"`
	Description *string `json:"description"`
	OccurredAt  string  `json:"occurred_at"`
	CreatedAt   *string `json:"created_at"`
}

func eventToJSON(e model.Event) eventJSON {
	return eventJSON{
		ID:          e.ID,
		OfferID:     e.OfferID,
		Type:        string(e.Type),
		Description: e.Description,
		OccurredAt:  model.FormatTime(e.OccurredAt),
		CreatedAt:   timeString(e.CreatedAt),
	}
}

type noteJSON struct {
	ID            int64   `json:"id"`
	EntityType    string  `json:"entity_type"`
	EntityID      int64   `json:"entity_id"`
	Body          string  `json:"body"`
	PinnedEventID *int64  `json:"pinned_event_id"`
	CreatedAt     *string `json:"created_at"`
	UpdatedAt     *string `json:"updated_at"`
}

func noteToJSON(n model.Note) noteJSON {
	return noteJSON{
		ID:            n.ID,
		EntityType:    string(n.EntityType),
		EntityID:      n.EntityID,
		Body:          n.Body,
		PinnedEventID: n.PinnedEventID,
		CreatedAt:     timeString(n.CreatedAt),
		UpdatedAt:     timeString(n.UpdatedAt),
	}
}

type interviewEntryJSON struct {
	ID           int64   `json:"id"`
	Question     string  `json:"question"`
	Answer       string  `json:"answer"`
	AnswerFormat string  `json:"answer_format"`
	DisplayOrder *int    `json:"display_order"`
	CreatedAt    *string `json:"created_at"`
	UpdatedAt    *string `json:"updated_at"`
}

func interviewEntryToJSON(e model.InterviewEntry) interviewEntryJSON {
	return interviewEntryJSON{
		ID:           e.ID,
		Question:     e.Question,
		Answer:       e.Answer,
		AnswerFormat: string(e.AnswerFormat),
		DisplayOrder: e.DisplayOrder,
		CreatedAt:    timeString(e.CreatedAt),
		UpdatedAt:    timeString(e.UpdatedAt),
	}
}

type snapshotJSON struct {
	ID         int64  `json:"id"`
	Origin     string `json:"origin"`
	RawText    string `json:"raw_text"`
	CapturedAt string `json:"captured_at"`
}

func snapshotToJSON(s model.Snapshot) snapshotJSON {
	return snapshotJSON{
		ID:         s.ID,
		Origin:     string(s.Origin),
		RawText:    s.RawText,
		CapturedAt: model.FormatTime(s.CapturedAt),
	}
}

type snapshotSummaryJSON struct {
	ID         int64   `json:"id"`
	Origin     string  `json:"origin"`
	CapturedAt string  `json:"captured_at"`
	TextLength int     `json:"text_length"`
	RawText    *string `json:"raw_text,omitempty"`
}

func snapshotInfoToJSON(s db.SnapshotInfo) snapshotSummaryJSON {
	return snapshotSummaryJSON{
		ID:         s.ID,
		Origin:     string(s.Origin),
		CapturedAt: model.FormatTime(s.CapturedAt),
		TextLength: s.TextLength,
		RawText:    s.RawText,
	}
}

type snapshotRefJSON struct {
	ID         int64  `json:"id"`
	CapturedAt string `json:"captured_at"`
}

type diffLineJSON struct {
	Op   string `json:"op"`
	Text string `json:"text"`
}

type snapshotDiffJSON struct {
	From  snapshotRefJSON `json:"from"`
	To    snapshotRefJSON `json:"to"`
	Lines []diffLineJSON  `json:"lines"`
}
