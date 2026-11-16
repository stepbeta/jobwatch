package api

import (
	"context"
	"strconv"
	"testing"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
)

type companyResp struct {
	Company companyJSON `json:"company"`
}

type companyListResp struct {
	Companies []companyJSON `json:"companies"`
	Total     int           `json:"total"`
}

type companyDetailResp struct {
	Company            companyJSON            `json:"company"`
	Offers             []offerJSON            `json:"offers"`
	Notes              []noteJSON             `json:"notes"`
	PossibleDuplicates []companyCandidateJSON `json:"possible_duplicates"`
}

type mergeResp struct {
	Company         companyJSON `json:"company"`
	MergedCompanyID int64       `json:"merged_company_id"`
	MovedOffers     int         `json:"moved_offers"`
	MovedNotes      int         `json:"moved_notes"`
}

func (ts *testServer) createCompany(t *testing.T, body map[string]any) companyJSON {
	t.Helper()
	rec := ts.do(t, "POST", "/api/companies", body)
	wantCode(t, rec, 201)
	return decodeBody[companyResp](t, rec).Company
}

func TestCompaniesCRUD(t *testing.T) {
	ts := newTestServer(t)
	qonto := ts.createCompany(t, map[string]any{"name": "Qonto", "website": "https://qonto.com"})
	qontoFR := ts.createCompany(t, map[string]any{"name": "Qonto France"})

	wantErrorCode(t, ts.do(t, "POST", "/api/companies", map[string]any{"name": "QONTO SAS"}),
		409, "duplicate_company")
	wantErrorCode(t, ts.do(t, "POST", "/api/companies", map[string]any{"name": "  "}),
		400, "validation_failed")

	rec := ts.do(t, "GET", "/api/companies?q=qonto", nil)
	wantCode(t, rec, 200)
	list := decodeBody[companyListResp](t, rec)
	if list.Total != 2 || len(list.Companies) != 2 {
		t.Errorf("companies = %+v", list)
	}

	rec = ts.do(t, "POST", "/api/notes", map[string]any{
		"entity_type": "company", "entity_id": qonto.ID, "body": "HQ in Paris",
	})
	wantCode(t, rec, 201)

	rec = ts.do(t, "GET", "/api/companies/1", nil)
	wantCode(t, rec, 200)
	detail := decodeBody[companyDetailResp](t, rec)
	if len(detail.Notes) != 1 || detail.Company.NoteCount != 1 {
		t.Errorf("company notes = %+v", detail.Notes)
	}
	if len(detail.PossibleDuplicates) != 1 || detail.PossibleDuplicates[0].ID != qontoFR.ID {
		t.Errorf("possible duplicates = %+v", detail.PossibleDuplicates)
	}
	if detail.PossibleDuplicates[0].Reason != "one name extends the other" {
		t.Errorf("duplicate reason = %q", detail.PossibleDuplicates[0].Reason)
	}

	rec = ts.do(t, "PATCH", "/api/companies/1", map[string]any{"name": "Qonto Bank", "website": nil})
	wantCode(t, rec, 200)
	patched := decodeBody[companyResp](t, rec).Company
	if patched.Name != "Qonto Bank" || patched.NameNormalized != "qonto bank" || patched.Website != nil {
		t.Errorf("patched company = %+v", patched)
	}
	wantErrorCode(t, ts.do(t, "PATCH", "/api/companies/1", map[string]any{"name": "Qonto France"}),
		409, "duplicate_company")
	wantErrorCode(t, ts.do(t, "PATCH", "/api/companies/99", map[string]any{"name": "Nope"}),
		404, "not_found")
	wantErrorCode(t, ts.do(t, "PATCH", "/api/companies/1", map[string]any{}), 400, "validation_failed")
}

func TestCompanyMerge(t *testing.T) {
	ts := newTestServer(t)
	source := ts.createCompany(t, map[string]any{"name": "Acme"})
	target := ts.createCompany(t, map[string]any{"name": "ACME SAS France"})
	offer := ts.seedOffer(t, model.Company{ID: source.ID, Name: source.Name, NameNormalized: source.NameNormalized},
		"SRE", "Paris", model.AppTodo, model.RemoteUnknown, 0, 0)
	if _, err := db.UpsertProvenance(context.Background(), ts.db, offer.ID, nil, nil,
		"https://source.example/sre", ts.clock.now()); err != nil {
		t.Fatal(err)
	}
	rec := ts.do(t, "POST", "/api/notes", map[string]any{
		"entity_type": "company", "entity_id": source.ID, "body": "to move",
	})
	wantCode(t, rec, 201)

	rec = ts.do(t, "POST", "/api/companies/1/merge", map[string]any{"into_id": target.ID})
	wantCode(t, rec, 200)
	merged := decodeBody[mergeResp](t, rec)
	if merged.MergedCompanyID != source.ID || merged.MovedOffers != 1 || merged.MovedNotes != 1 {
		t.Errorf("merge = %+v", merged)
	}
	if merged.Company.ID != target.ID {
		t.Errorf("target = %+v", merged.Company)
	}
	if merged.Company.OfferCount != 1 || merged.Company.NoteCount != 1 {
		t.Errorf("merged counts = %d offers, %d notes", merged.Company.OfferCount, merged.Company.NoteCount)
	}

	wantErrorCode(t, ts.do(t, "GET", "/api/companies/1", nil), 404, "not_found")
	rec = ts.do(t, "GET", "/api/companies/2", nil)
	wantCode(t, rec, 200)
	detail := decodeBody[companyDetailResp](t, rec)
	if len(detail.Offers) != 1 || detail.Offers[0].ID != offer.ID {
		t.Errorf("offers after merge = %+v", detail.Offers)
	}
	if len(detail.Notes) != 1 || detail.Notes[0].Body != "to move" {
		t.Errorf("notes after merge = %+v", detail.Notes)
	}
	prov, err := db.ListProvenance(context.Background(), ts.db, offer.ID)
	if err != nil || len(prov) != 1 || prov[0].URL != "https://source.example/sre" {
		t.Errorf("provenance after company merge = %+v, err %v", prov, err)
	}
	// The moved offer must keep deduplicating under the target company name.
	var fp string
	if err := ts.db.QueryRow(`SELECT fingerprint FROM job_offers WHERE id = ?`, offer.ID).Scan(&fp); err != nil {
		t.Fatal(err)
	}
	if want := model.Fingerprint(target.NameNormalized, model.NormalizeTitle("SRE"), ""); fp != want {
		t.Errorf("fingerprint = %q, want %q", fp, want)
	}

	wantErrorCode(t, ts.do(t, "POST", "/api/companies/2/merge", map[string]any{"into_id": 2}),
		400, "validation_failed")
	wantErrorCode(t, ts.do(t, "POST", "/api/companies/2/merge", map[string]any{"into_id": 99}),
		404, "not_found")
	wantErrorCode(t, ts.do(t, "POST", "/api/companies/2/merge", map[string]any{}),
		400, "validation_failed")
}

func TestCompanyMergeConflictAndDelete(t *testing.T) {
	ts := newTestServer(t)
	a := ts.createCompany(t, map[string]any{"name": "Acme"})
	b := ts.createCompany(t, map[string]any{"name": "Acme France"})
	ts.seedOffer(t, model.Company{ID: a.ID, Name: a.Name, NameNormalized: a.NameNormalized},
		"SRE", "Paris", model.AppTodo, model.RemoteUnknown, 0, 0)
	ts.seedOffer(t, model.Company{ID: b.ID, Name: b.Name, NameNormalized: b.NameNormalized},
		"SRE", "Paris", model.AppTodo, model.RemoteUnknown, 0, 0)

	wantErrorCode(t, ts.do(t, "POST", "/api/companies/1/merge", map[string]any{"into_id": 2}),
		409, "merge_conflict")

	wantErrorCode(t, ts.do(t, "DELETE", "/api/companies/1", nil), 409, "company_has_offers")

	empty := ts.createCompany(t, map[string]any{"name": "Empty"})
	rec := ts.do(t, "DELETE", "/api/companies/"+strconv.FormatInt(empty.ID, 10), nil)
	wantCode(t, rec, 204)
	wantErrorCode(t, ts.do(t, "GET", "/api/companies/"+strconv.FormatInt(empty.ID, 10), nil), 404, "not_found")
	wantErrorCode(t, ts.do(t, "DELETE", "/api/companies/99", nil), 404, "not_found")
}
