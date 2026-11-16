// Package model holds the domain types shared across packages. Structs
// mirror the SQLite tables defined in migrations/; enum-like TEXT columns
// are represented by typed string constants.
package model

import "time"

// SourceType identifies the adapter used to fetch a source.
type SourceType string

const (
	SourceGreenhouse SourceType = "greenhouse"
	SourceLever      SourceType = "lever"
	SourceAshby      SourceType = "ashby"
	SourcePersonio   SourceType = "personio"
	SourceWorkable   SourceType = "workable"
	SourceBambooHR   SourceType = "bamboohr"
	SourceWTTJ       SourceType = "wttj"
	SourceRSS        SourceType = "rss"
	SourceHTML       SourceType = "html"
	SourceManual     SourceType = "manual"
)

// Valid reports whether t is a known source type.
func (t SourceType) Valid() bool {
	switch t {
	case SourceGreenhouse, SourceLever, SourceAshby, SourcePersonio, SourceWorkable,
		SourceBambooHR, SourceWTTJ, SourceRSS, SourceHTML, SourceManual:
		return true
	}
	return false
}

// ScanStatus is the outcome of the last scan of a source.
type ScanStatus string

const (
	ScanOK      ScanStatus = "ok"
	ScanError   ScanStatus = "error"
	ScanBlocked ScanStatus = "blocked"
)

// OfferState tracks the offer as seen on its sources.
type OfferState string

const (
	OfferActive      OfferState = "active"
	OfferChanged     OfferState = "changed"
	OfferDisappeared OfferState = "disappeared"
)

// Valid reports whether s is a known offer state.
func (s OfferState) Valid() bool {
	switch s {
	case OfferActive, OfferChanged, OfferDisappeared:
		return true
	}
	return false
}

// ApplicationState tracks the user's application lifecycle for an offer.
type ApplicationState string

const (
	AppTodo      ApplicationState = "todo"
	AppApplied   ApplicationState = "applied"
	AppResponded ApplicationState = "responded"
	AppInterview ApplicationState = "interview"
	AppOffer     ApplicationState = "offer"
	AppNegative  ApplicationState = "negative"
	AppMaybe     ApplicationState = "maybe"
	AppPositive  ApplicationState = "positive"
)

// Valid reports whether s is a known application state.
func (s ApplicationState) Valid() bool {
	switch s {
	case AppTodo, AppApplied, AppResponded, AppInterview, AppOffer, AppNegative, AppMaybe, AppPositive:
		return true
	}
	return false
}

// RemotePolicy is the parsed remote policy of an offer.
type RemotePolicy string

const (
	RemoteFull    RemotePolicy = "remote"
	RemoteHybrid  RemotePolicy = "hybrid"
	RemoteOnsite  RemotePolicy = "onsite"
	RemoteUnknown RemotePolicy = "unknown"
)

// Valid reports whether p is a known remote policy.
func (p RemotePolicy) Valid() bool {
	switch p {
	case RemoteFull, RemoteHybrid, RemoteOnsite, RemoteUnknown:
		return true
	}
	return false
}

// EventType is the kind of a timeline event.
type EventType string

const (
	EventCVSent EventType = "cv_sent"
	// EventMessageReceived is either a recruiter's inbound first contact
	// or a reply to an application.
	EventMessageReceived EventType = "message_received"
	// EventMessageSent is a user-initiated ping or follow-up.
	EventMessageSent EventType = "message_sent"
	EventInterview   EventType = "interview"
	EventOffer       EventType = "offer"
	EventRejected    EventType = "rejected"
	EventGhosted     EventType = "ghosted"
	EventOther       EventType = "other"
)

// Valid reports whether t is a known event type.
func (t EventType) Valid() bool {
	switch t {
	case EventCVSent, EventMessageReceived, EventMessageSent, EventInterview,
		EventOffer, EventRejected, EventGhosted, EventOther:
		return true
	}
	return false
}

// EntityType is the kind of entity a note is attached to.
type EntityType string

const (
	EntityOffer   EntityType = "offer"
	EntityCompany EntityType = "company"
)

// Valid reports whether t is a known note entity type.
func (t EntityType) Valid() bool {
	switch t {
	case EntityOffer, EntityCompany:
		return true
	}
	return false
}

// SnapshotOrigin records how a description snapshot was captured.
type SnapshotOrigin string

const (
	// SnapshotOriginScan is written by the scan pipeline or a manual URL
	// import.
	SnapshotOriginScan SnapshotOrigin = "scan"
	// SnapshotOriginManual is written when the user saves a description
	// through PATCH /api/offers/:id.
	SnapshotOriginManual SnapshotOrigin = "manual"
)

// Valid reports whether o is a known snapshot origin.
func (o SnapshotOrigin) Valid() bool {
	switch o {
	case SnapshotOriginScan, SnapshotOriginManual:
		return true
	}
	return false
}

type Company struct {
	ID             int64
	Name           string
	NameNormalized string
	Website        *string
	CreatedAt      time.Time
}

type Source struct {
	ID             int64
	Name           string
	URL            string
	Type           SourceType
	ConfigJSON     string
	Enabled        bool
	LastScanAt     *time.Time
	LastScanStatus *ScanStatus
	CreatedAt      time.Time
}

type JobOffer struct {
	ID                 int64
	CompanyID          int64
	Title              string
	TitleNormalized    string
	Location           *string
	RemotePolicy       RemotePolicy
	URL                *string
	Fingerprint        string
	FirstSeenAt        time.Time
	LastSeenAt         time.Time
	OfferState         OfferState
	ApplicationState   ApplicationState
	RawText            *string
	TextHash           *string
	TitleConfidence    *int
	LocationConfidence *int
	MatchReasonsJSON   *string
	SavedAt            *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// OfferScores is the deterministic classifier output persisted on an
// offer. It is kept separate from JobOffer so the scan pipeline and the
// reclassification command can update it without touching identity or
// lifecycle fields.
type OfferScores struct {
	RemotePolicy       RemotePolicy
	TitleConfidence    int
	LocationConfidence int
	MatchReasonsJSON   string
}

type Provenance struct {
	ID          int64
	OfferID     int64
	SourceID    *int64 // nil for manual imports
	ExternalUID *string
	URL         string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
}

type Event struct {
	ID          int64
	OfferID     int64
	Type        EventType
	Description *string
	OccurredAt  time.Time
	CreatedAt   time.Time
}

type Note struct {
	ID            int64
	EntityType    EntityType
	EntityID      int64
	Body          string
	PinnedEventID *int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Snapshot struct {
	ID         int64
	OfferID    int64
	Origin     SnapshotOrigin
	RawText    string
	CapturedAt time.Time
}
