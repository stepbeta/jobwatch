package api

import (
	"fmt"
	"testing"

	"github.com/stepbeta/jobwatch/internal/model"
)

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

func TestEventMessageTypes(t *testing.T) {
	ts := newTestServer(t)
	seedOfferSet(t, ts)

	create := func(body map[string]any) eventJSON {
		t.Helper()
		rec := ts.do(t, "POST", "/api/offers/1/events", body)
		wantCode(t, rec, 201)
		return decodeBody[eventResp](t, rec).Event
	}
	// The two message types are accepted and the old reply_received value
	// is gone. Ordering still follows occurred_at, then id.
	create(map[string]any{"type": "message_received", "occurred_at": "2026-11-02T10:00:00Z"})
	create(map[string]any{"type": "message_sent", "occurred_at": "2026-11-01T10:00:00Z"})
	create(map[string]any{"type": "message_received", "occurred_at": "2026-11-03T10:00:00Z"})

	rec := ts.do(t, "GET", "/api/offers/1/events", nil)
	wantCode(t, rec, 200)
	events := decodeBody[eventListResp](t, rec).Events
	if len(events) != 3 {
		t.Fatalf("events = %+v", events)
	}
	wantOrder := []struct {
		typ  string
		date string
	}{
		{"message_sent", "2026-11-01T10:00:00Z"},
		{"message_received", "2026-11-02T10:00:00Z"},
		{"message_received", "2026-11-03T10:00:00Z"},
	}
	for i, want := range wantOrder {
		if events[i].Type != want.typ || events[i].OccurredAt != want.date {
			t.Errorf("event %d = %+v, want %s at %s", i, events[i], want.typ, want.date)
		}
	}

	wantErrorCode(t, ts.do(t, "POST", "/api/offers/1/events", map[string]any{"type": "reply_received"}),
		400, "validation_failed")

	// PATCH accepts the new values as well.
	rec = ts.do(t, "PATCH", "/api/offers/1/events/2", map[string]any{"type": "message_received"})
	wantCode(t, rec, 200)
	if e := decodeBody[eventResp](t, rec).Event; e.Type != "message_received" {
		t.Errorf("patched event type = %q, want message_received", e.Type)
	}
	wantErrorCode(t, ts.do(t, "PATCH", "/api/offers/1/events/2", map[string]any{"type": "reply_received"}),
		400, "validation_failed")
}

func TestEventApplicationTransitions(t *testing.T) {
	ts := newTestServer(t)
	acme := ts.seedCompany(t, "Acme")
	o := ts.seedOffer(t, acme, "SRE", "Paris", model.AppNone, model.RemoteUnknown)
	base := fmt.Sprintf("/api/offers/%d", o.ID)

	type createResp struct {
		Event            eventJSON `json:"event"`
		ApplicationState string    `json:"application_state"`
		StateChanged     bool      `json:"state_changed"`
	}
	setState := func(s model.ApplicationState) {
		rec := ts.do(t, "PATCH", base, map[string]any{"application_state": string(s)})
		wantCode(t, rec, 200)
	}
	create := func(typ model.EventType) createResp {
		rec := ts.do(t, "POST", base+"/events", map[string]any{"type": string(typ)})
		wantCode(t, rec, 201)
		return decodeBody[createResp](t, rec)
	}
	state := func() model.ApplicationState {
		rec := ts.do(t, "GET", base, nil)
		wantCode(t, rec, 200)
		return model.ApplicationState(decodeBody[struct {
			Offer offerJSON `json:"offer"`
		}](t, rec).Offer.ApplicationState)
	}

	t.Run("cv_sent", func(t *testing.T) {
		cases := []struct {
			from    model.ApplicationState
			want    model.ApplicationState
			changed bool
		}{
			{model.AppNone, model.AppApplied, true},
			{model.AppSaved, model.AppApplied, true},
			{model.AppNegative, model.AppApplied, true},
			{model.AppApplied, model.AppApplied, false},
			{model.AppInterview, model.AppInterview, false},
			{model.AppOffer, model.AppOffer, false},
			{model.AppAccepted, model.AppAccepted, false},
			{model.AppTurnedDown, model.AppTurnedDown, false},
		}
		for _, tc := range cases {
			setState(tc.from)
			got := create(model.EventCVSent)
			if got.ApplicationState != string(tc.want) || got.StateChanged != tc.changed {
				t.Errorf("cv_sent from %q: state=%q changed=%v, want %q/%v",
					tc.from, got.ApplicationState, got.StateChanged, tc.want, tc.changed)
			}
			if s := state(); s != tc.want {
				t.Errorf("cv_sent from %q stored state = %q, want %q", tc.from, s, tc.want)
			}
		}
	})

	t.Run("cv_sent from saved clears saved_at", func(t *testing.T) {
		setState(model.AppSaved)
		rec := ts.do(t, "GET", base, nil)
		wantCode(t, rec, 200)
		if got := decodeBody[struct {
			Offer offerJSON `json:"offer"`
		}](t, rec).Offer; got.SavedAt == nil {
			t.Fatal("saved offer has no saved_at")
		}
		if got := create(model.EventCVSent); !got.StateChanged {
			t.Error("cv_sent from saved did not report a state change")
		}
		rec = ts.do(t, "GET", base, nil)
		wantCode(t, rec, 200)
		if got := decodeBody[struct {
			Offer offerJSON `json:"offer"`
		}](t, rec).Offer; got.SavedAt != nil {
			t.Errorf("saved_at after cv_sent = %v, want null", got.SavedAt)
		}
	})

	t.Run("rejected", func(t *testing.T) {
		all := []model.ApplicationState{
			model.AppNone, model.AppSaved, model.AppApplied, model.AppInterview,
			model.AppOffer, model.AppAccepted, model.AppTurnedDown, model.AppNegative,
		}
		for _, from := range all {
			setState(from)
			got := create(model.EventRejected)
			wantChanged := from != model.AppNegative
			if got.ApplicationState != string(model.AppNegative) || got.StateChanged != wantChanged {
				t.Errorf("rejected from %q: state=%q changed=%v, want negative/%v",
					from, got.ApplicationState, got.StateChanged, wantChanged)
			}
			if s := state(); s != model.AppNegative {
				t.Errorf("rejected from %q stored state = %q, want negative", from, s)
			}
		}
	})

	t.Run("ghosted", func(t *testing.T) {
		cases := []struct {
			from    model.ApplicationState
			want    model.ApplicationState
			changed bool
		}{
			{model.AppApplied, model.AppNegative, true},
			{model.AppInterview, model.AppNegative, true},
			{model.AppOffer, model.AppOffer, false},
			{model.AppNone, model.AppNone, false},
			{model.AppSaved, model.AppSaved, false},
			{model.AppAccepted, model.AppAccepted, false},
			{model.AppTurnedDown, model.AppTurnedDown, false},
			{model.AppNegative, model.AppNegative, false},
		}
		for _, tc := range cases {
			setState(tc.from)
			got := create(model.EventGhosted)
			if got.ApplicationState != string(tc.want) || got.StateChanged != tc.changed {
				t.Errorf("ghosted from %q: state=%q changed=%v, want %q/%v",
					tc.from, got.ApplicationState, got.StateChanged, tc.want, tc.changed)
			}
		}
	})

	t.Run("other events do not change the state", func(t *testing.T) {
		setState(model.AppInterview)
		for _, typ := range []model.EventType{
			model.EventMessageReceived, model.EventMessageSent, model.EventInterview,
			model.EventOffer, model.EventOther,
		} {
			got := create(typ)
			if got.ApplicationState != string(model.AppInterview) || got.StateChanged {
				t.Errorf("%s changed state to %q (changed=%v)", typ, got.ApplicationState, got.StateChanged)
			}
		}
	})

	t.Run("edits and deletes do not recompute the state", func(t *testing.T) {
		setState(model.AppNone)
		created := create(model.EventCVSent)
		if s := state(); s != model.AppApplied {
			t.Fatalf("cv_sent stored state = %q, want applied", s)
		}
		eventURL := fmt.Sprintf("%s/events/%d", base, created.Event.ID)

		rec := ts.do(t, "PATCH", eventURL, map[string]any{"type": "rejected"})
		wantCode(t, rec, 200)
		if s := state(); s != model.AppApplied {
			t.Errorf("state after PATCH to rejected = %q, want applied", s)
		}

		rec = ts.do(t, "DELETE", eventURL, nil)
		wantCode(t, rec, 204)
		if s := state(); s != model.AppApplied {
			t.Errorf("state after DELETE = %q, want applied", s)
		}
	})

	t.Run("editing a message into a rejection does not close", func(t *testing.T) {
		setState(model.AppNone)
		created := create(model.EventMessageReceived)
		if s := state(); s != model.AppNone {
			t.Fatalf("message_received stored state = %q, want none", s)
		}
		eventURL := fmt.Sprintf("%s/events/%d", base, created.Event.ID)
		rec := ts.do(t, "PATCH", eventURL, map[string]any{"type": "rejected"})
		wantCode(t, rec, 200)
		if s := state(); s != model.AppNone {
			t.Errorf("state after editing message into rejected = %q, want none", s)
		}
	})

	t.Run("deleting the rejection keeps the offer negative", func(t *testing.T) {
		setState(model.AppApplied)
		created := create(model.EventRejected)
		if s := state(); s != model.AppNegative {
			t.Fatalf("rejected stored state = %q, want negative", s)
		}
		eventURL := fmt.Sprintf("%s/events/%d", base, created.Event.ID)
		rec := ts.do(t, "DELETE", eventURL, nil)
		wantCode(t, rec, 204)
		if s := state(); s != model.AppNegative {
			t.Errorf("state after deleting the rejection = %q, want negative", s)
		}
	})
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
