package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
)

var testNow = time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)

func mustCompany(t *testing.T, sqlDB *sql.DB, name string) model.Company {
	t.Helper()
	c, err := CreateCompany(context.Background(), sqlDB, name, nil, testNow)
	if err != nil {
		t.Fatalf("CreateCompany(%q): %v", name, err)
	}
	return c
}

func mustOffer(t *testing.T, sqlDB *sql.DB, c model.Company, title, location string) model.JobOffer {
	t.Helper()
	titleNorm := model.NormalizeTitle(title)
	o := model.JobOffer{
		CompanyID:       &c.ID,
		Title:           title,
		TitleNormalized: titleNorm,
		Fingerprint:     model.Fingerprint(c.NameNormalized, titleNorm, ""),
	}
	if location != "" {
		o.Location = &location
	}
	o, err := InsertOffer(context.Background(), sqlDB, o, testNow)
	if err != nil {
		t.Fatalf("InsertOffer(%q): %v", title, err)
	}
	return o
}

func TestCompanyMerge(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)

	acme := mustCompany(t, sqlDB, "Acme")
	acmeSAS := mustCompany(t, sqlDB, "ACME SAS France")
	o1 := mustOffer(t, sqlDB, acme, "Site Reliability Engineer", "Paris")
	o2 := mustOffer(t, sqlDB, acme, "Platform Engineer", "")
	kept := mustOffer(t, sqlDB, acmeSAS, "Backend Engineer", "Lyon")
	if err := linkCompanyKey(ctx, sqlDB, acme.ID, "greenhouse:acme", testNow); err != nil {
		t.Fatal(err)
	}
	if err := linkCompanyKey(ctx, sqlDB, acmeSAS.ID, "greenhouse:acmesas", testNow); err != nil {
		t.Fatal(err)
	}
	fp1, fp2 := o1.Fingerprint, o2.Fingerprint

	note, err := InsertNote(ctx, sqlDB, model.Note{
		EntityType: model.EntityCompany, EntityID: acme.ID, Body: "recruiter is nice",
	}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InsertNote(ctx, sqlDB, model.Note{
		EntityType: model.EntityCompany, EntityID: acmeSAS.ID, Body: "target note",
	}, testNow); err != nil {
		t.Fatal(err)
	}

	res, err := MergeCompanies(ctx, sqlDB, acme.ID, acmeSAS.ID, testNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("MergeCompanies: %v", err)
	}
	if res.MovedOffers != 2 || res.MovedNotes != 1 {
		t.Errorf("moved = %d offers, %d notes; want 2, 1", res.MovedOffers, res.MovedNotes)
	}
	if _, err := GetCompany(ctx, sqlDB, acme.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("merged company still exists: %v", err)
	}
	got, err := GetCompany(ctx, sqlDB, acmeSAS.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "ACME SAS France" {
		t.Errorf("target name = %q", got.Name)
	}
	for id, fp := range map[int64]string{o1.ID: fp1, o2.ID: fp2} {
		o, err := GetOffer(ctx, sqlDB, id)
		if err != nil {
			t.Fatal(err)
		}
		if o.CompanyID == nil || *o.CompanyID != acmeSAS.ID {
			t.Errorf("offer %d company = %v, want %d", id, o.CompanyID, acmeSAS.ID)
		}
		if o.Fingerprint != fp {
			t.Errorf("offer %d fingerprint = %q, want unchanged %q", id, o.Fingerprint, fp)
		}
	}
	if o, err := GetOffer(ctx, sqlDB, kept.ID); err != nil || o.CompanyID == nil || *o.CompanyID != acmeSAS.ID {
		t.Errorf("target offer untouched: %+v, %v", o, err)
	}

	// The absorbed identifiers and names resolve to the target, so a later
	// scan cannot recreate the ghost company.
	if c, err := GetCompanyByKey(ctx, sqlDB, "greenhouse:acme"); err != nil || c.ID != acmeSAS.ID {
		t.Errorf("absorbed key = %+v, %v; want company %d", c, err, acmeSAS.ID)
	}
	if c, err := GetCompanyByKey(ctx, sqlDB, "greenhouse:acmesas"); err != nil || c.ID != acmeSAS.ID {
		t.Errorf("target key = %+v, %v; want company %d", c, err, acmeSAS.ID)
	}
	if c, err := GetCompanyByGlobalAlias(ctx, sqlDB, "acme"); err != nil || c.ID != acmeSAS.ID {
		t.Errorf("absorbed alias = %+v, %v; want company %d", c, err, acmeSAS.ID)
	}
	if c, created, err := ResolveCompany(ctx, sqlDB, CompanyInput{Name: "Acme"}, testNow); err != nil || created || c.ID != acmeSAS.ID {
		t.Errorf("resolve absorbed name = %+v created=%v err=%v; want company %d", c, created, err, acmeSAS.ID)
	}

	moved, err := GetNote(ctx, sqlDB, note.ID)
	if err != nil {
		t.Fatal(err)
	}
	if moved.EntityID != acmeSAS.ID {
		t.Errorf("note entity_id = %d, want %d", moved.EntityID, acmeSAS.ID)
	}
}

func TestCompanyMergeSameTitleOffers(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)
	a := mustCompany(t, sqlDB, "Acme")
	b := mustCompany(t, sqlDB, "Acme France")
	mustOffer(t, sqlDB, a, "SRE", "Paris")
	mustOffer(t, sqlDB, b, "SRE", "Paris")

	res, err := MergeCompanies(ctx, sqlDB, a.ID, b.ID, testNow)
	if err != nil {
		t.Fatalf("merge with same-title offers: %v", err)
	}
	if res.MovedOffers != 1 {
		t.Errorf("moved offers = %d, want 1", res.MovedOffers)
	}
	var n int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM job_offers WHERE company_id = ?`, b.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("target offers = %d, want 2", n)
	}
	if _, err := GetCompany(ctx, sqlDB, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("source company still exists: %v", err)
	}
	if _, err := MergeCompanies(ctx, sqlDB, b.ID, b.ID, testNow); !errors.Is(err, ErrMergeSelf) {
		t.Errorf("self merge error = %v, want ErrMergeSelf", err)
	}
}

func TestCompanyRenameKeepsOffersAndAliases(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)
	c := mustCompany(t, sqlDB, "Acme")
	o := mustOffer(t, sqlDB, c, "SRE", "Paris")
	fp := o.Fingerprint

	name := "Acme Cloud"
	got, err := UpdateCompany(ctx, sqlDB, c.ID, CompanyUpdate{Name: &name}, testNow)
	if err != nil {
		t.Fatalf("UpdateCompany: %v", err)
	}
	if got.NameNormalized != "acme cloud" {
		t.Errorf("normalized = %q", got.NameNormalized)
	}
	updated, err := GetOffer(ctx, sqlDB, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Fingerprint != fp {
		t.Errorf("fingerprint = %q, want unchanged %q", updated.Fingerprint, fp)
	}
	if _, err := GetCompanyByGlobalAlias(ctx, sqlDB, "acme cloud"); err != nil {
		t.Errorf("new name is not an alias: %v", err)
	}
	if _, err := GetCompanyByGlobalAlias(ctx, sqlDB, "acme"); err != nil {
		t.Errorf("old name alias was lost: %v", err)
	}

	// Renaming to a name another company already uses is allowed; names are
	// attributes, not identity.
	other := mustCompany(t, sqlDB, "Other")
	dup := "Other"
	if _, err := UpdateCompany(ctx, sqlDB, c.ID, CompanyUpdate{Name: &dup}, testNow); err != nil {
		t.Errorf("duplicate rename rejected: %v", err)
	}
	if got, err := GetCompany(ctx, sqlDB, c.ID); err != nil || got.Name != "Other" {
		t.Errorf("renamed company = %+v, %v", got, err)
	}
	if got, err := GetCompany(ctx, sqlDB, other.ID); err != nil || got.Name != "Other" {
		t.Errorf("other company = %+v, %v", got, err)
	}
}

func TestIncompleteOfferLifecycle(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)
	c := mustCompany(t, sqlDB, "Acme")

	// Missing company: stored with a NULL company_id.
	o1, err := InsertIncompleteOffer(ctx, sqlDB, model.JobOffer{
		Title: "Mystery Role", TitleNormalized: model.NormalizeTitle("Mystery Role"),
		URL: strPtrOf("https://x/1"),
	}, model.IncompleteMissingCompany, `{"title":"Mystery Role","url":"https://x/1"}`, testNow)
	if err != nil {
		t.Fatalf("insert missing-company offer: %v", err)
	}
	if o1.CompanyID != nil || o1.IncompleteReason != model.IncompleteMissingCompany || o1.IncompleteRaw == nil {
		t.Errorf("missing-company offer = %+v", o1)
	}

	// Missing title: the company is known and kept.
	o2, err := InsertIncompleteOffer(ctx, sqlDB, model.JobOffer{
		CompanyID: &c.ID, URL: strPtrOf("https://x/2"),
	}, model.IncompleteMissingTitle, `{"company":"Acme","url":"https://x/2"}`, testNow)
	if err != nil {
		t.Fatalf("insert missing-title offer: %v", err)
	}
	if o2.CompanyID == nil || *o2.CompanyID != c.ID || o2.IncompleteReason != model.IncompleteMissingTitle {
		t.Errorf("missing-title offer = %+v", o2)
	}

	// The normal list excludes both; the incomplete view lists both and the
	// NULL company comes back as an empty company name.
	list, err := QueryOffers(ctx, sqlDB, OfferQuery{Sort: SortLastSeen})
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 0 {
		t.Errorf("default offers total = %d, want 0", list.Total)
	}
	incomplete, err := QueryOffers(ctx, sqlDB, OfferQuery{IncompleteOnly: true, Sort: SortLastSeen})
	if err != nil {
		t.Fatal(err)
	}
	if incomplete.Total != 2 {
		t.Errorf("incomplete offers total = %d, want 2", incomplete.Total)
	}
	for _, item := range incomplete.Offers {
		if item.ID == o1.ID && item.CompanyName != "" {
			t.Errorf("missing-company offer company name = %q, want empty", item.CompanyName)
		}
	}
	cli, err := ListOffers(ctx, sqlDB, OfferFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(cli) != 0 {
		t.Errorf("CLI list includes %d incomplete offers", len(cli))
	}

	// Repair the missing-company item.
	loc := "Paris"
	repaired, err := RepairOffer(ctx, sqlDB, o1.ID, c.ID, "Site Reliability Engineer", &loc, testNow)
	if err != nil {
		t.Fatalf("RepairOffer: %v", err)
	}
	if repaired.CompanyID == nil || *repaired.CompanyID != c.ID ||
		repaired.Title != "Site Reliability Engineer" || repaired.IncompleteReason != "" ||
		repaired.IncompleteRaw == nil || repaired.Location == nil || *repaired.Location != loc {
		t.Errorf("repaired offer = %+v", repaired)
	}
	list, err = QueryOffers(ctx, sqlDB, OfferQuery{Sort: SortLastSeen})
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || list.Offers[0].ID != o1.ID {
		t.Errorf("default offers after repair = %+v", list)
	}
	incomplete, err = QueryOffers(ctx, sqlDB, OfferQuery{IncompleteOnly: true, Sort: SortLastSeen})
	if err != nil {
		t.Fatal(err)
	}
	if incomplete.Total != 1 || incomplete.Offers[0].ID != o2.ID {
		t.Errorf("incomplete offers after repair = %+v", incomplete)
	}

	// A complete offer cannot be repaired again.
	if _, err := RepairOffer(ctx, sqlDB, o1.ID, c.ID, "Again", nil, testNow); !errors.Is(err, ErrOfferNotIncomplete) {
		t.Errorf("second repair error = %v, want ErrOfferNotIncomplete", err)
	}
	// An invalid reason is rejected before any row is written.
	if _, err := InsertIncompleteOffer(ctx, sqlDB, model.JobOffer{
		Title: "x", URL: strPtrOf("https://x/3"),
	}, model.IncompleteReason("nope"), "{}", testNow); err == nil {
		t.Error("invalid incomplete reason accepted")
	}
}

func TestDeleteCompany(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)
	c := mustCompany(t, sqlDB, "Acme")
	mustOffer(t, sqlDB, c, "SRE", "")
	if err := DeleteCompany(ctx, sqlDB, c.ID); !errors.Is(err, ErrCompanyHasOffers) {
		t.Errorf("delete with offers = %v, want ErrCompanyHasOffers", err)
	}
	empty := mustCompany(t, sqlDB, "Empty")
	if _, err := InsertNote(ctx, sqlDB, model.Note{EntityType: model.EntityCompany, EntityID: empty.ID, Body: "x"}, testNow); err != nil {
		t.Fatal(err)
	}
	if err := linkCompanyKey(ctx, sqlDB, empty.ID, "ashby:empty", testNow); err != nil {
		t.Fatal(err)
	}
	if err := DeleteCompany(ctx, sqlDB, empty.ID); err != nil {
		t.Fatalf("delete empty company: %v", err)
	}
	notes, err := ListNotes(ctx, sqlDB, model.EntityCompany, empty.ID)
	if err != nil || len(notes) != 0 {
		t.Errorf("notes after delete = %v, %v", notes, err)
	}
	var keys, aliases int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM company_keys WHERE company_id = ?`, empty.ID).Scan(&keys); err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM company_aliases WHERE company_id = ?`, empty.ID).Scan(&aliases); err != nil {
		t.Fatal(err)
	}
	if keys != 0 || aliases != 0 {
		t.Errorf("company identity rows after delete = %d keys, %d aliases", keys, aliases)
	}
}

func TestSimilarCompanies(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)
	c := mustCompany(t, sqlDB, "Qonto")
	mustCompany(t, sqlDB, "Qonto France")
	mustCompany(t, sqlDB, "Qontoo")
	mustCompany(t, sqlDB, "Spotify")

	cands, err := SimilarCompanies(ctx, sqlDB, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 2 {
		t.Fatalf("candidates = %+v, want 2", cands)
	}
	if cands[0].NameNormalized != "qonto france" && cands[0].NameNormalized != "qontoo" {
		t.Errorf("unexpected first candidate %+v", cands[0])
	}
}

func TestQueryOffersSortValues(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)
	acme := mustCompany(t, sqlDB, "Acme")
	zeta := mustCompany(t, sqlDB, "Zeta")

	mk := func(c model.Company, title, loc string, tc, lc int) model.JobOffer {
		o := mustOffer(t, sqlDB, c, title, loc)
		if _, err := sqlDB.ExecContext(ctx,
			`UPDATE job_offers SET title_confidence = ?, location_confidence = ? WHERE id = ?`,
			tc, lc, o.ID); err != nil {
			t.Fatal(err)
		}
		return o
	}
	mk(acme, "Beta SRE", "Zurich", 70, 95)
	mk(zeta, "Alpha Platform", "Paris", 95, 60)
	mk(acme, "Gamma Engineer", "", 80, 80)

	tests := []struct {
		name  string
		sort  string
		order string
		want  []string
	}{
		{"title defaults asc", SortTitle, "", []string{"Alpha Platform", "Beta SRE", "Gamma Engineer"}},
		{"title desc", SortTitle, OrderDesc, []string{"Gamma Engineer", "Beta SRE", "Alpha Platform"}},
		{"company asc then title", SortCompany, OrderAsc, []string{"Beta SRE", "Gamma Engineer", "Alpha Platform"}},
		{"company desc", SortCompany, OrderDesc, []string{"Alpha Platform", "Gamma Engineer", "Beta SRE"}},
		{"location asc with empty first", SortLocation, OrderAsc, []string{"Gamma Engineer", "Alpha Platform", "Beta SRE"}},
		{"location desc", SortLocation, OrderDesc, []string{"Beta SRE", "Alpha Platform", "Gamma Engineer"}},
		{"title confidence defaults desc", SortTitleConfidence, "", []string{"Alpha Platform", "Gamma Engineer", "Beta SRE"}},
		{"title confidence asc", SortTitleConfidence, OrderAsc, []string{"Beta SRE", "Gamma Engineer", "Alpha Platform"}},
		{"location confidence desc", SortLocationConfidence, OrderDesc, []string{"Beta SRE", "Gamma Engineer", "Alpha Platform"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := QueryOffers(ctx, sqlDB, OfferQuery{Sort: tt.sort, Order: tt.order})
			if err != nil {
				t.Fatalf("QueryOffers: %v", err)
			}
			var titles []string
			for _, o := range got.Offers {
				titles = append(titles, o.Title)
			}
			if len(titles) != len(tt.want) {
				t.Fatalf("titles = %v, want %v", titles, tt.want)
			}
			for i := range tt.want {
				if titles[i] != tt.want[i] {
					t.Fatalf("titles = %v, want %v", titles, tt.want)
				}
			}
		})
	}
}

func TestValidOfferSortAndOrder(t *testing.T) {
	for _, s := range []string{
		SortLastSeen, SortFirstSeen, SortRecentEvent, SortTitle, SortCompany,
		SortLocation, SortTitleConfidence, SortLocationConfidence, SortSaved,
	} {
		if !ValidOfferSort(s) {
			t.Errorf("ValidOfferSort(%q) = false", s)
		}
	}
	for _, s := range []string{"", "nope", "title_desc"} {
		if ValidOfferSort(s) {
			t.Errorf("ValidOfferSort(%q) = true", s)
		}
	}
	for _, o := range []string{"", OrderAsc, OrderDesc} {
		if !ValidOfferOrder(o) {
			t.Errorf("ValidOfferOrder(%q) = false", o)
		}
	}
	for _, o := range []string{"ASC", "up", "descending"} {
		if ValidOfferOrder(o) {
			t.Errorf("ValidOfferOrder(%q) = true", o)
		}
	}
}

func TestQueryOffersFilters(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)
	acme := mustCompany(t, sqlDB, "Acme")
	other := mustCompany(t, sqlDB, "Other")

	raw := "Kubernetes and Terraform platform work"
	mk := func(c model.Company, title, loc string, app model.ApplicationState, remote model.RemotePolicy, tc, lc int, desc *string) model.JobOffer {
		o := mustOffer(t, sqlDB, c, title, loc)
		if _, err := sqlDB.ExecContext(ctx,
			`UPDATE job_offers SET application_state = ?, remote_policy = ?, title_confidence = ?, location_confidence = ?, raw_text = ? WHERE id = ?`,
			string(app), string(remote), tc, lc, desc, o.ID); err != nil {
			t.Fatal(err)
		}
		return o
	}
	sre := mk(acme, "Site Reliability Engineer", "Paris", model.AppApplied, model.RemoteFull, 95, 90, &raw)
	mk(acme, "Platform Engineer", "Remote - France", model.AppTodo, model.RemoteFull, 80, 85, nil)
	mk(other, "SRE", "Lyon", model.AppInterview, model.RemoteHybrid, 70, 60, nil)
	zero := 0
	mk(other, "Marketing Manager", "", model.AppTodo, model.RemoteOnsite, zero, zero, nil)

	if _, err := InsertEvent(ctx, sqlDB, model.Event{
		OfferID: sre.ID, Type: model.EventCVSent, OccurredAt: testNow.Add(time.Hour),
	}, testNow); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		query OfferQuery
		want  []string
		total int
	}{
		{
			name:  "no filter sorted by last seen",
			query: OfferQuery{Sort: SortLastSeen},
			total: 4,
		},
		{
			name:  "application state",
			query: OfferQuery{ApplicationStates: []model.ApplicationState{model.AppApplied}, Sort: SortLastSeen},
			want:  []string{"Site Reliability Engineer"}, total: 1,
		},
		{
			name:  "multiple offer states",
			query: OfferQuery{OfferStates: []model.OfferState{model.OfferActive}, Sort: SortLastSeen},
			total: 4,
		},
		{
			name:  "remote policy",
			query: OfferQuery{RemotePolicies: []model.RemotePolicy{model.RemoteFull}, Sort: SortLastSeen},
			want:  []string{"Site Reliability Engineer", "Platform Engineer"}, total: 2,
		},
		{
			name:  "min title confidence",
			query: OfferQuery{MinTitleConfidence: intPtrOf(90), Sort: SortLastSeen},
			want:  []string{"Site Reliability Engineer"}, total: 1,
		},
		{
			name:  "min location confidence",
			query: OfferQuery{MinLocationConfidence: intPtrOf(85), Sort: SortLastSeen},
			want:  []string{"Site Reliability Engineer", "Platform Engineer"}, total: 2,
		},
		{
			name:  "company",
			query: OfferQuery{CompanyID: &other.ID, Sort: SortLastSeen},
			total: 2,
		},
		{
			name:  "has application",
			query: OfferQuery{HasApplication: true, Sort: SortRecentEvent},
			want:  []string{"Site Reliability Engineer", "SRE"}, total: 2,
		},
		{
			name:  "full text",
			query: OfferQuery{Search: "kubernetes", Sort: SortLastSeen},
			want:  []string{"Site Reliability Engineer"}, total: 1,
		},
		{
			name:  "pagination",
			query: OfferQuery{Sort: SortTitle, Limit: 2, Offset: 0},
			total: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := QueryOffers(ctx, sqlDB, tt.query)
			if err != nil {
				t.Fatalf("QueryOffers: %v", err)
			}
			if got.Total != tt.total {
				t.Errorf("total = %d, want %d", got.Total, tt.total)
			}
			if tt.want != nil {
				var titles []string
				for _, o := range got.Offers {
					titles = append(titles, o.Title)
				}
				if len(titles) != len(tt.want) {
					t.Fatalf("titles = %v, want %v", titles, tt.want)
				}
				seen := map[string]bool{}
				for _, s := range titles {
					seen[s] = true
				}
				for _, w := range tt.want {
					if !seen[w] {
						t.Errorf("titles = %v, missing %q", titles, w)
					}
				}
			}
			if tt.query.Limit > 0 && len(got.Offers) > tt.query.Limit {
				t.Errorf("page has %d offers, limit %d", len(got.Offers), tt.query.Limit)
			}
		})
	}

	if _, err := QueryOffers(ctx, sqlDB, OfferQuery{Search: `"unterminated`, Sort: SortLastSeen}); !errors.Is(err, ErrInvalidQuery) {
		t.Errorf("invalid FTS query error = %v, want ErrInvalidQuery", err)
	}
	// recent_event ordering: the offer with an event comes first.
	got, err := QueryOffers(ctx, sqlDB, OfferQuery{HasApplication: true, Sort: SortRecentEvent})
	if err != nil {
		t.Fatal(err)
	}
	if got.Offers[0].Title != "Site Reliability Engineer" {
		t.Errorf("recent_event first = %q", got.Offers[0].Title)
	}
	if got.Offers[0].LastEventAt == nil {
		t.Error("last_event_at not populated")
	}
}

func intPtrOf(v int) *int { return &v }

func TestEventsAndNotes(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)
	c := mustCompany(t, sqlDB, "Acme")
	o := mustOffer(t, sqlDB, c, "SRE", "")

	backdated := testNow.Add(-72 * time.Hour)
	e, err := InsertEvent(ctx, sqlDB, model.Event{
		OfferID: o.ID, Type: model.EventCVSent, Description: strPtrOf("sent CV"), OccurredAt: backdated,
	}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	desc := "reply"
	if err := UpdateEvent(ctx, sqlDB, e.ID, EventUpdate{Description: &desc, OccurredAt: &testNow}); err != nil {
		t.Fatal(err)
	}
	got, err := GetEvent(ctx, sqlDB, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.OccurredAt.Equal(testNow) || got.Description == nil || *got.Description != "reply" {
		t.Errorf("event = %+v", got)
	}
	events, err := ListEvents(ctx, sqlDB, o.ID)
	if err != nil || len(events) != 1 {
		t.Fatalf("events = %v, %v", events, err)
	}

	note, err := InsertNote(ctx, sqlDB, model.Note{
		EntityType: model.EntityOffer, EntityID: o.ID, Body: "remember this", PinnedEventID: &e.ID,
	}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if err := DeleteEvent(ctx, sqlDB, e.ID); err != nil {
		t.Fatal(err)
	}
	gotNote, err := GetNote(ctx, sqlDB, note.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotNote.PinnedEventID != nil {
		t.Errorf("pin not cleared: %v", *gotNote.PinnedEventID)
	}
	body := "edited"
	if err := UpdateNote(ctx, sqlDB, note.ID, NoteUpdate{Body: &body, ClearPinned: true}, testNow); err != nil {
		t.Fatal(err)
	}
	if err := DeleteNote(ctx, sqlDB, note.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := GetNote(ctx, sqlDB, note.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleted note = %v, want ErrNotFound", err)
	}
}

func strPtrOf(s string) *string { return &s }

func TestDeleteSourceDetachesProvenance(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)
	src, err := InsertSource(ctx, sqlDB, model.Source{
		Name: "Acme", URL: "https://example.com/acme", Type: model.SourceGreenhouse, Enabled: true,
	}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	c := mustCompany(t, sqlDB, "Acme")
	o := mustOffer(t, sqlDB, c, "SRE", "")
	srcID := src.ID
	if _, err := UpsertProvenance(ctx, sqlDB, o.ID, &srcID, nil, "https://example.com/jobs/1", testNow); err != nil {
		t.Fatal(err)
	}
	if err := DeleteSource(ctx, sqlDB, src.ID); err != nil {
		t.Fatalf("DeleteSource: %v", err)
	}
	if _, err := GetSource(ctx, sqlDB, src.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("source still exists: %v", err)
	}
	prov, err := ListProvenanceDetailed(ctx, sqlDB, o.ID)
	if err != nil || len(prov) != 1 {
		t.Fatalf("provenance = %v, %v", prov, err)
	}
	if prov[0].SourceID != nil {
		t.Errorf("provenance source_id = %v, want nil", *prov[0].SourceID)
	}
}
