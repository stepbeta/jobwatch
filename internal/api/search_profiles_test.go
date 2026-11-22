package api

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

type profileListResp struct {
	Profiles []searchProfileJSON `json:"profiles"`
}

type profileResp struct {
	Profile searchProfileJSON `json:"profile"`
}

func TestSearchProfileEndpoints(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.do(t, "GET", "/api/search-profiles", nil)
	wantCode(t, rec, 200)
	list := decodeBody[profileListResp](t, rec)
	if len(list.Profiles) != 1 {
		t.Fatalf("profiles = %+v, want 1", list.Profiles)
	}
	def := list.Profiles[0]
	if !def.IsDefault || def.Name != "Default" || def.BucketScale != "five" {
		t.Errorf("default profile = %+v", def)
	}
	if len(def.Terms) != 129 {
		t.Errorf("seeded terms = %d, want 129", len(def.Terms))
	}
	if len(def.Remote) != 4 {
		t.Errorf("remote rows = %+v", def.Remote)
	}

	rec = ts.do(t, "GET", "/api/search-profiles/1", nil)
	wantCode(t, rec, 200)
	detail := decodeBody[profileResp](t, rec)
	if detail.Profile.ID != 1 || detail.Profile.TitleLabel != "Platform Engineer / SRE" {
		t.Errorf("profile detail = %+v", detail.Profile)
	}
	wantErrorCode(t, ts.do(t, "GET", "/api/search-profiles/9999", nil), 404, "not_found")
	wantErrorCode(t, ts.do(t, "GET", "/api/search-profiles/abc", nil), 400, "invalid_id")

	// Invalid input is rejected and the stored profile is unchanged.
	before := detail.Profile
	wantErrorCode(t, ts.do(t, "PUT", "/api/search-profiles/1", map[string]any{
		"name": "Broken", "bucket_scale": "five",
		"terms": []map[string]any{{"affinity": "title", "bucket": "neutral", "term": "sre", "weight": 10}},
	}), 400, "validation_failed")
	wantErrorCode(t, ts.do(t, "PUT", "/api/search-profiles/1", map[string]any{
		"name": "Broken", "bucket_scale": "five",
		"terms": []map[string]any{{"affinity": "title", "bucket": "positive", "term": "   ", "weight": 10}},
	}), 400, "validation_failed")
	// A non-numeric weight cannot be decoded; the 400 message names the
	// offending field, and the stored profile stays unchanged.
	rec = ts.do(t, "PUT", "/api/search-profiles/1", map[string]any{
		"name": "Broken", "bucket_scale": "five",
		"terms": []map[string]any{{"affinity": "title", "bucket": "positive", "term": "sre", "weight": "sixty"}},
	})
	wantErrorCode(t, rec, 400, "invalid_json")
	if msg := decodeBody[errorBody](t, rec).Error.Message; !strings.Contains(msg, "weight") {
		t.Errorf("invalid weight message = %q, want it to name the weight field", msg)
	}
	rec = ts.do(t, "GET", "/api/search-profiles/1", nil)
	wantCode(t, rec, 200)
	if got := decodeBody[profileResp](t, rec).Profile; got.Name != before.Name || len(got.Terms) != len(before.Terms) {
		t.Errorf("profile changed after rejected update: %+v", got)
	}
	wantErrorCode(t, ts.do(t, "PUT", "/api/search-profiles/9999", map[string]any{
		"name": "Nope", "bucket_scale": "five",
	}), 404, "not_found")

	// A valid replace changes the next offer read without reclassification.
	rec = ts.do(t, "POST", "/api/offers/import", map[string]any{
		"company": "Acme", "title": "Platform Engineer", "location": "Remote - France",
	})
	wantCode(t, rec, 201)
	rec = ts.do(t, "GET", "/api/offers", nil)
	wantCode(t, rec, 200)
	if o := decodeBody[offerListResp](t, rec).Offers[0]; o.TitleAffinity != 85 || o.LocationAffinity != 35 {
		t.Fatalf("default affinity = %d/%d, want 85/35", o.TitleAffinity, o.LocationAffinity)
	}

	rec = ts.do(t, "PUT", "/api/search-profiles/1", map[string]any{
		"name": "Platform only", "title_label": "Platform", "location_label": "France",
		"bucket_scale": "three",
		"terms": []map[string]any{
			{"affinity": "title", "bucket": "positive", "term": "platform engineer", "weight": 40},
			{"affinity": "location", "bucket": "positive", "term": "france", "weight": 10},
		},
		"remote": []map[string]any{
			{"policy": "remote", "accepted": true},
			{"policy": "unknown", "accepted": false},
			{"policy": "hybrid", "accepted": false},
			{"policy": "onsite", "accepted": false},
		},
	})
	wantCode(t, rec, 200)
	updated := decodeBody[profileResp](t, rec).Profile
	if updated.Name != "Platform only" || updated.BucketScale != "three" || len(updated.Terms) != 2 || !updated.IsDefault {
		t.Errorf("updated profile = %+v", updated)
	}
	rec = ts.do(t, "GET", "/api/offers", nil)
	wantCode(t, rec, 200)
	o := decodeBody[offerListResp](t, rec).Offers[0]
	if o.TitleAffinity != 40 || o.LocationAffinity != 10 {
		t.Errorf("affinity after profile change = %d/%d, want 40/10", o.TitleAffinity, o.LocationAffinity)
	}
	if !o.RemotePolicyAccepted {
		t.Error("remote offer should be accepted by the new profile")
	}
	rec = ts.do(t, "GET", "/api/offers?remote_accepted=false", nil)
	wantCode(t, rec, 200)
	if got := decodeBody[offerListResp](t, rec); got.Total != 0 {
		t.Errorf("remote_accepted=false total = %d, want 0", got.Total)
	}
}

func TestSearchProfileSelection(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	if _, err := ts.db.ExecContext(ctx,
		`INSERT INTO search_profiles(name, bucket_scale, is_default, created_at, updated_at)
		 VALUES ('Second', 'five', 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	var secondID int64
	if err := ts.db.QueryRowContext(ctx,
		`SELECT id FROM search_profiles WHERE name = 'Second'`).Scan(&secondID); err != nil {
		t.Fatal(err)
	}

	rec := ts.do(t, "POST", "/api/offers/import", map[string]any{
		"company": "Acme", "title": "Platform Engineer", "location": "Remote - France",
	})
	wantCode(t, rec, 201)

	rec = ts.do(t, "GET", "/api/search-profiles", nil)
	wantCode(t, rec, 200)
	if got := decodeBody[profileListResp](t, rec).Profiles; len(got) != 2 || !got[0].IsDefault {
		t.Errorf("profiles = %+v, want default first then the second", got)
	}

	// The default profile scores 85/35; the empty second profile scores 0/0.
	rec = ts.do(t, "GET", "/api/offers?profile_id="+itoa(secondID), nil)
	wantCode(t, rec, 200)
	o := decodeBody[offerListResp](t, rec).Offers[0]
	if o.TitleAffinity != 0 || o.LocationAffinity != 0 {
		t.Errorf("second profile affinity = %d/%d, want 0/0", o.TitleAffinity, o.LocationAffinity)
	}
	wantErrorCode(t, ts.do(t, "GET", "/api/offers?profile_id=9999", nil), 404, "not_found")

	rec = ts.do(t, "GET", "/api/offers/1?profile_id="+itoa(secondID), nil)
	wantCode(t, rec, 200)
	if detail := decodeBody[offerDetailResp](t, rec); detail.Offer.TitleAffinity != 0 {
		t.Errorf("detail used the wrong profile: %+v", detail.Offer)
	}
}

func TestOfferResponsesOmitLegacyScores(t *testing.T) {
	ts := newTestServer(t)
	seedOfferSet(t, ts)

	for _, path := range []string{"/api/offers", "/api/offers/1"} {
		rec := ts.do(t, "GET", path, nil)
		wantCode(t, rec, 200)
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		offerRaw := raw
		if body, ok := raw["offer"]; ok {
			var single map[string]json.RawMessage
			if err := json.Unmarshal(body, &single); err != nil {
				t.Fatalf("%s offer: %v", path, err)
			}
			offerRaw = single
		}
		if body, ok := raw["offers"]; ok {
			var offers []map[string]json.RawMessage
			if err := json.Unmarshal(body, &offers); err != nil {
				t.Fatalf("%s offers: %v", path, err)
			}
			if len(offers) == 0 {
				t.Fatalf("%s returned no offers", path)
			}
			offerRaw = offers[0]
		}
		for _, legacy := range []string{"title_confidence", "location_confidence", "match_reasons_json"} {
			if _, ok := offerRaw[legacy]; ok {
				t.Errorf("%s still exposes %q", path, legacy)
			}
		}
		for _, want := range []string{"title_affinity", "location_affinity", "remote_policy", "remote_policy_accepted", "match_reasons"} {
			if _, ok := offerRaw[want]; !ok {
				t.Errorf("%s misses %q", path, want)
			}
		}
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
