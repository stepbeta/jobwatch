package api

import (
	"testing"

	"github.com/stepbeta/jobwatch/internal/model"
)

type noteListResp struct {
	Notes []noteJSON `json:"notes"`
}

type noteResp struct {
	Note noteJSON `json:"note"`
}

func TestNotesCRUD(t *testing.T) {
	ts := newTestServer(t)
	sre, acme := seedOfferSet(t, ts)
	e := ts.seedEvent(t, sre.ID, model.EventCVSent, ts.clock.now(), "sent")

	rec := ts.do(t, "POST", "/api/notes", map[string]any{
		"entity_type": "offer", "entity_id": sre.ID, "body": "call back on Monday", "pinned_event_id": e.ID,
	})
	wantCode(t, rec, 201)
	n := decodeBody[noteResp](t, rec).Note
	if n.EntityType != "offer" || n.PinnedEventID == nil || *n.PinnedEventID != e.ID {
		t.Errorf("note = %+v", n)
	}

	rec = ts.do(t, "GET", "/api/notes?entity_type=offer&entity_id=1", nil)
	wantCode(t, rec, 200)
	if got := decodeBody[noteListResp](t, rec).Notes; len(got) != 1 {
		t.Fatalf("notes = %+v", got)
	}

	rec = ts.do(t, "PATCH", "/api/notes/1", map[string]any{"body": "call back on Tuesday"})
	wantCode(t, rec, 200)
	n = decodeBody[noteResp](t, rec).Note
	if n.Body != "call back on Tuesday" || n.PinnedEventID == nil {
		t.Errorf("patched note = %+v", n)
	}

	rec = ts.do(t, "PATCH", "/api/notes/1", map[string]any{"pinned_event_id": nil})
	wantCode(t, rec, 200)
	if n := decodeBody[noteResp](t, rec).Note; n.PinnedEventID != nil {
		t.Errorf("pin not cleared: %+v", n)
	}

	// A company note may be pinned to an event of one of its offers.
	rec = ts.do(t, "POST", "/api/notes", map[string]any{
		"entity_type": "company", "entity_id": acme.ID, "body": "company-level note", "pinned_event_id": e.ID,
	})
	wantCode(t, rec, 201)

	// ... but not to an event of another company's offer.
	other := ts.seedCompany(t, "Another Co")
	otherOffer := ts.seedOffer(t, other, "SRE", "", model.AppTodo, model.RemoteUnknown)
	otherEvent := ts.seedEvent(t, otherOffer.ID, model.EventOther, ts.clock.now(), "")
	wantErrorCode(t, ts.do(t, "POST", "/api/notes", map[string]any{
		"entity_type": "company", "entity_id": acme.ID, "body": "wrong pin", "pinned_event_id": otherEvent.ID,
	}), 400, "validation_failed")

	rec = ts.do(t, "DELETE", "/api/notes/1", nil)
	wantCode(t, rec, 204)
	wantErrorCode(t, ts.do(t, "GET", "/api/notes/1", nil), 404, "not_found")
}

func TestNoteValidation(t *testing.T) {
	ts := newTestServer(t)
	seedOfferSet(t, ts)
	rec := ts.do(t, "POST", "/api/notes", map[string]any{
		"entity_type": "offer", "entity_id": 1, "body": "existing note",
	})
	wantCode(t, rec, 201)

	cases := []struct {
		name string
		body map[string]any
		code int
		err  string
	}{
		{"missing entity", map[string]any{"entity_type": "offer", "entity_id": 99, "body": "x"}, 404, "not_found"},
		{"missing entity id", map[string]any{"entity_type": "offer", "body": "x"}, 400, "validation_failed"},
		{"invalid entity type", map[string]any{"entity_type": "nope", "entity_id": 1, "body": "x"}, 400, "validation_failed"},
		{"missing body", map[string]any{"entity_type": "offer", "entity_id": 1}, 400, "validation_failed"},
		{"unknown field", map[string]any{"entity_type": "offer", "entity_id": 1, "body": "x", "z": 1}, 400, "invalid_json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wantErrorCode(t, ts.do(t, "POST", "/api/notes", tc.body), tc.code, tc.err)
		})
	}
	wantErrorCode(t, ts.do(t, "GET", "/api/notes", nil), 400, "validation_failed")
	wantErrorCode(t, ts.do(t, "GET", "/api/notes?entity_type=offer&entity_id=99", nil), 404, "not_found")
	wantErrorCode(t, ts.do(t, "PATCH", "/api/notes/1", map[string]any{}), 400, "validation_failed")
}
