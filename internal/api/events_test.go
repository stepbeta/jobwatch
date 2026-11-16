package api

import "testing"

type eventListResp struct {
	Events []eventJSON `json:"events"`
}

type eventResp struct {
	Event eventJSON `json:"event"`
}

func TestEventsCRUD(t *testing.T) {
	ts := newTestServer(t)
	seedOfferSet(t, ts)

	rec := ts.do(t, "POST", "/api/offers/1/events", map[string]any{
		"type":        "cv_sent",
		"description": "CV sent to the recruiter",
		"occurred_at": "2026-11-01T10:30:00Z",
	})
	wantCode(t, rec, 201)
	e := decodeBody[eventResp](t, rec).Event
	if e.Type != "cv_sent" || e.OfferID != 1 || e.OccurredAt != "2026-11-01T10:30:00Z" {
		t.Errorf("event = %+v", e)
	}

	rec = ts.do(t, "GET", "/api/offers/1/events", nil)
	wantCode(t, rec, 200)
	if got := decodeBody[eventListResp](t, rec).Events; len(got) != 1 {
		t.Fatalf("events = %+v", got)
	}

	rec = ts.do(t, "PATCH", "/api/offers/1/events/1", map[string]any{
		"type": "interview", "description": "moved", "occurred_at": "2026-11-02",
	})
	wantCode(t, rec, 200)
	e = decodeBody[eventResp](t, rec).Event
	if e.Type != "interview" || e.OccurredAt != "2026-11-02T00:00:00Z" || e.Description == nil || *e.Description != "moved" {
		t.Errorf("patched event = %+v", e)
	}

	// Backdating is supported by PATCH too.
	rec = ts.do(t, "PATCH", "/api/offers/1/events/1", map[string]any{"occurred_at": "2026-10-01T08:15"})
	wantCode(t, rec, 200)
	if e := decodeBody[eventResp](t, rec).Event; e.OccurredAt != "2026-10-01T08:15:00Z" {
		t.Errorf("backdated event = %+v", e)
	}

	// The event of offer 1 is not reachable through offer 2.
	wantErrorCode(t, ts.do(t, "PATCH", "/api/offers/2/events/1", map[string]any{"type": "offer"}),
		404, "not_found")
	wantErrorCode(t, ts.do(t, "DELETE", "/api/offers/2/events/1", nil), 404, "not_found")

	rec = ts.do(t, "DELETE", "/api/offers/1/events/1", nil)
	wantCode(t, rec, 204)
	rec = ts.do(t, "GET", "/api/offers/1/events", nil)
	wantCode(t, rec, 200)
	if got := decodeBody[eventListResp](t, rec).Events; len(got) != 0 {
		t.Errorf("events after delete = %+v", got)
	}
}

func TestEventValidation(t *testing.T) {
	ts := newTestServer(t)
	seedOfferSet(t, ts)

	cases := []struct {
		name string
		body map[string]any
		code int
		err  string
	}{
		{"missing type", map[string]any{"description": "x"}, 400, "validation_failed"},
		{"invalid type", map[string]any{"type": "nope"}, 400, "validation_failed"},
		{"invalid timestamp", map[string]any{"type": "other", "occurred_at": "yesterday"}, 400, "validation_failed"},
		{"unknown field", map[string]any{"type": "other", "bogus": 1}, 400, "invalid_json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wantErrorCode(t, ts.do(t, "POST", "/api/offers/1/events", tc.body), tc.code, tc.err)
		})
	}
	wantErrorCode(t, ts.do(t, "POST", "/api/offers/99/events", map[string]any{"type": "other"}),
		404, "not_found")
	wantErrorCode(t, ts.do(t, "GET", "/api/offers/99/events", nil), 404, "not_found")
}
