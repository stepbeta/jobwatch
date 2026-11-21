package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/internal/sources"
)

type testClock struct{ t time.Time }

func (c *testClock) now() time.Time          { return c.t }
func (c *testClock) advance(d time.Duration) { c.t = c.t.Add(d) }

type fakeFetcher struct {
	offers map[int64][]sources.RawOffer
	errs   map[int64]error
}

func (f *fakeFetcher) Fetch(_ context.Context, src model.Source) ([]sources.RawOffer, error) {
	if err := f.errs[src.ID]; err != nil {
		return nil, err
	}
	return f.offers[src.ID], nil
}

type testServer struct {
	Server  *Server
	handler http.Handler
	db      *sql.DB
	fetcher *fakeFetcher
	clock   *testClock
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	sqlDB, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	ff := &fakeFetcher{offers: map[int64][]sources.RawOffer{}, errs: map[int64]error{}}
	clock := &testClock{t: time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)}
	client := sources.NewClient()
	client.DefaultInterval = 0
	s := &Server{DB: sqlDB, Version: "test", Fetcher: ff, HTTPClient: client, Now: clock.now}
	return &testServer{Server: s, handler: s.Handler(), db: sqlDB, fetcher: ff, clock: clock}
}

func (ts *testServer) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	ts.handler.ServeHTTP(rec, r)
	return rec
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return v
}

func wantCode(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("code = %d, want %d (body %s)", rec.Code, want, rec.Body.String())
	}
}

func wantErrorCode(t *testing.T, rec *httptest.ResponseRecorder, code int, errCode string) {
	t.Helper()
	wantCode(t, rec, code)
	body := decodeBody[errorBody](t, rec)
	if body.Error.Code != errCode {
		t.Errorf("error code = %q, want %q (body %s)", body.Error.Code, errCode, rec.Body.String())
	}
	if body.Error.Message == "" {
		t.Errorf("error message is empty")
	}
}

func (ts *testServer) seedCompany(t *testing.T, name string) model.Company {
	t.Helper()
	c, err := db.CreateCompany(context.Background(), ts.db, name, nil, ts.clock.now())
	if err != nil {
		t.Fatalf("seed company %q: %v", name, err)
	}
	return c
}

func (ts *testServer) seedOffer(t *testing.T, c model.Company, title, location string, app model.ApplicationState,
	remote model.RemotePolicy, titleConf, locConf int) model.JobOffer {
	t.Helper()
	ctx := context.Background()
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
	o, err := db.InsertOffer(ctx, ts.db, o, ts.clock.now())
	if err != nil {
		t.Fatalf("seed offer %q: %v", title, err)
	}
	if _, err := ts.db.ExecContext(ctx,
		`UPDATE job_offers SET application_state = ?, remote_policy = ?, title_confidence = ?, location_confidence = ? WHERE id = ?`,
		string(app), string(remote), titleConf, locConf, o.ID); err != nil {
		t.Fatal(err)
	}
	o, err = db.GetOffer(ctx, ts.db, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func (ts *testServer) seedEvent(t *testing.T, offerID int64, typ model.EventType, at time.Time, desc string) model.Event {
	t.Helper()
	var d *string
	if desc != "" {
		d = &desc
	}
	e, err := db.InsertEvent(context.Background(), ts.db, model.Event{
		OfferID: offerID, Type: typ, Description: d, OccurredAt: at,
	}, ts.clock.now())
	if err != nil {
		t.Fatal(err)
	}
	return e
}
