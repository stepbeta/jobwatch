package api

import (
	"strconv"
	"testing"
	"time"
)

type interviewEntryListResp struct {
	Entries []interviewEntryJSON `json:"interview_entries"`
}

type interviewEntryResp struct {
	Entry interviewEntryJSON `json:"interview_entry"`
}

func TestInterviewEntriesCRUD(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.do(t, "POST", "/api/interview-entries", map[string]any{
		"question": "Tell me about a production incident you handled.",
		"answer":   "**Situation**\n\n- alert fired\n- rolled back",
	})
	wantCode(t, rec, 201)
	first := decodeBody[interviewEntryResp](t, rec).Entry
	if first.ID != 1 || first.Question != "Tell me about a production incident you handled." {
		t.Fatalf("created entry = %+v", first)
	}
	if first.AnswerFormat != "markdown" || first.DisplayOrder != nil {
		t.Errorf("created entry = %+v", first)
	}
	if first.CreatedAt == nil || first.UpdatedAt == nil {
		t.Errorf("created timestamps = %v / %v", first.CreatedAt, first.UpdatedAt)
	}

	// An answer may be empty at creation (question only).
	rec = ts.do(t, "POST", "/api/interview-entries", map[string]any{"question": "Why this company?"})
	wantCode(t, rec, 201)
	second := decodeBody[interviewEntryResp](t, rec).Entry
	if second.Answer != "" || second.AnswerFormat != "markdown" {
		t.Errorf("second entry = %+v", second)
	}

	rec = ts.do(t, "GET", "/api/interview-entries", nil)
	wantCode(t, rec, 200)
	list := decodeBody[interviewEntryListResp](t, rec).Entries
	if len(list) != 2 || list[0].ID != 1 || list[1].ID != 2 {
		t.Fatalf("list = %+v", list)
	}

	rec = ts.do(t, "GET", "/api/interview-entries/1", nil)
	wantCode(t, rec, 200)
	if got := decodeBody[interviewEntryResp](t, rec).Entry; got.Answer != first.Answer {
		t.Errorf("get = %+v", got)
	}

	// Partial update: answer only keeps the question.
	ts.clock.advance(time.Minute)
	rec = ts.do(t, "PATCH", "/api/interview-entries/1", map[string]any{
		"answer": "**Situation**\n\n- alert fired\n- rolled back\n- postmortem",
	})
	wantCode(t, rec, 200)
	patched := decodeBody[interviewEntryResp](t, rec).Entry
	if patched.Question != first.Question || patched.Answer == first.Answer {
		t.Errorf("patched entry = %+v", patched)
	}
	if patched.CreatedAt == nil || patched.UpdatedAt == nil || *patched.UpdatedAt == *patched.CreatedAt {
		t.Errorf("updated_at not bumped: created=%v updated=%v", patched.CreatedAt, patched.UpdatedAt)
	}

	// Duplicate questions are allowed: this is a personal notebook.
	rec = ts.do(t, "POST", "/api/interview-entries", map[string]any{"question": first.Question})
	wantCode(t, rec, 201)

	rec = ts.do(t, "DELETE", "/api/interview-entries/2", nil)
	wantCode(t, rec, 204)
	wantErrorCode(t, ts.do(t, "GET", "/api/interview-entries/2", nil), 404, "not_found")
	wantErrorCode(t, ts.do(t, "DELETE", "/api/interview-entries/2", nil), 404, "not_found")
}

func TestInterviewEntriesOrdering(t *testing.T) {
	ts := newTestServer(t)
	for _, q := range []string{"first", "second", "third"} {
		wantCode(t, ts.do(t, "POST", "/api/interview-entries", map[string]any{"question": q}), 201)
	}

	// Rows with an explicit display_order come first, then the others by
	// id (Milestone 9: NULL means id order).
	wantCode(t, ts.do(t, "PATCH", "/api/interview-entries/3", map[string]any{"display_order": 1}), 200)
	wantCode(t, ts.do(t, "PATCH", "/api/interview-entries/1", map[string]any{"display_order": 2}), 200)

	rec := ts.do(t, "GET", "/api/interview-entries", nil)
	wantCode(t, rec, 200)
	list := decodeBody[interviewEntryListResp](t, rec).Entries
	if len(list) != 3 || list[0].ID != 3 || list[1].ID != 1 || list[2].ID != 2 {
		t.Fatalf("ordered list = %+v", list)
	}

	// null clears the manual position and the row falls back to id order.
	rec = ts.do(t, "PATCH", "/api/interview-entries/3", map[string]any{"display_order": nil})
	wantCode(t, rec, 200)
	if got := decodeBody[interviewEntryResp](t, rec).Entry; got.DisplayOrder != nil {
		t.Errorf("display_order not cleared: %+v", got)
	}
	rec = ts.do(t, "GET", "/api/interview-entries", nil)
	list = decodeBody[interviewEntryListResp](t, rec).Entries
	if len(list) != 3 || list[0].ID != 1 || list[1].ID != 2 || list[2].ID != 3 {
		t.Fatalf("ordered list after clearing = %+v", list)
	}
}

func TestInterviewEntryCategory(t *testing.T) {
	ts := newTestServer(t)

	// Omitted category stores NULL.
	rec := ts.do(t, "POST", "/api/interview-entries", map[string]any{"question": "No category"})
	wantCode(t, rec, 201)
	if got := decodeBody[interviewEntryResp](t, rec).Entry; got.Category != nil {
		t.Errorf("omitted category = %q, want null", *got.Category)
	}

	// A supplied category is trimmed and must not be blank.
	rec = ts.do(t, "POST", "/api/interview-entries", map[string]any{
		"question": "Behavioural", "category": "  STAR stories  ",
	})
	wantCode(t, rec, 201)
	created := decodeBody[interviewEntryResp](t, rec).Entry
	if created.Category == nil || *created.Category != "STAR stories" {
		t.Errorf("created category = %v, want STAR stories", created.Category)
	}

	rec = ts.do(t, "GET", "/api/interview-entries", nil)
	wantCode(t, rec, 200)
	list := decodeBody[interviewEntryListResp](t, rec).Entries
	if len(list) != 2 || list[0].ID != 2 || list[0].Category == nil ||
		*list[0].Category != "STAR stories" || list[1].ID != 1 || list[1].Category != nil {
		t.Fatalf("list categories = %+v", list)
	}

	// PATCH replaces the category, trims it and can clear it with null.
	rec = ts.do(t, "PATCH", "/api/interview-entries/2", map[string]any{"category": " Values "})
	wantCode(t, rec, 200)
	if got := decodeBody[interviewEntryResp](t, rec).Entry; got.Category == nil || *got.Category != "Values" {
		t.Errorf("patched category = %v, want Values", got.Category)
	}
	rec = ts.do(t, "PATCH", "/api/interview-entries/2", map[string]any{"category": nil})
	wantCode(t, rec, 200)
	if got := decodeBody[interviewEntryResp](t, rec).Entry; got.Category != nil {
		t.Errorf("cleared category = %q, want null", *got.Category)
	}
}

func TestInterviewEntriesPerSectionOrdering(t *testing.T) {
	ts := newTestServer(t)
	create := func(question string, category any) int64 {
		t.Helper()
		body := map[string]any{"question": question}
		if category != nil {
			body["category"] = category
		}
		rec := ts.do(t, "POST", "/api/interview-entries", body)
		wantCode(t, rec, 201)
		return decodeBody[interviewEntryResp](t, rec).Entry.ID
	}

	uncategorized := create("u1", nil)
	values1 := create("v1", "Values")
	behavioural1 := create("b1", "Behavioural")
	behavioural2 := create("b2", "Behavioural")
	values2 := create("v2", "Values")

	// Inside each section explicit display_order comes first, then id.
	wantCode(t, ts.do(t, "PATCH", "/api/interview-entries/"+strconv.FormatInt(values2, 10), map[string]any{"display_order": 1}), 200)
	wantCode(t, ts.do(t, "PATCH", "/api/interview-entries/"+strconv.FormatInt(values1, 10), map[string]any{"display_order": 2}), 200)
	wantCode(t, ts.do(t, "PATCH", "/api/interview-entries/"+strconv.FormatInt(behavioural2, 10), map[string]any{"display_order": 1}), 200)

	rec := ts.do(t, "GET", "/api/interview-entries", nil)
	wantCode(t, rec, 200)
	list := decodeBody[interviewEntryListResp](t, rec).Entries
	wantIDs := []int64{behavioural2, behavioural1, values2, values1, uncategorized}
	if len(list) != len(wantIDs) {
		t.Fatalf("list = %+v", list)
	}
	for i, want := range wantIDs {
		if list[i].ID != want {
			t.Fatalf("list[%d].id = %d, want %d (list = %+v)", i, list[i].ID, want, list)
		}
	}
}

func TestInterviewEntryValidation(t *testing.T) {
	ts := newTestServer(t)
	wantCode(t, ts.do(t, "POST", "/api/interview-entries", map[string]any{"question": "kept"}), 201)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
		code   int
		err    string
	}{
		{"missing question", "POST", "/api/interview-entries", map[string]any{"answer": "x"}, 400, "validation_failed"},
		{"blank question", "POST", "/api/interview-entries", map[string]any{"question": "   "}, 400, "validation_failed"},
		{"invalid answer format", "POST", "/api/interview-entries", map[string]any{"question": "q", "answer_format": "html"}, 400, "validation_failed"},
		{"blank create category", "POST", "/api/interview-entries", map[string]any{"question": "q", "category": "   "}, 400, "validation_failed"},
		{"unknown create field", "POST", "/api/interview-entries", map[string]any{"question": "q", "z": 1}, 400, "invalid_json"},
		{"empty patch", "PATCH", "/api/interview-entries/1", map[string]any{}, 400, "validation_failed"},
		{"blank patch question", "PATCH", "/api/interview-entries/1", map[string]any{"question": "  "}, 400, "validation_failed"},
		{"blank patch category", "PATCH", "/api/interview-entries/1", map[string]any{"category": ""}, 400, "validation_failed"},
		{"non-string patch category", "PATCH", "/api/interview-entries/1", map[string]any{"category": 3}, 400, "validation_failed"},
		{"unknown patch field", "PATCH", "/api/interview-entries/1", map[string]any{"answer_format": "markdown"}, 400, "invalid_json"},
		{"non-integer display order", "PATCH", "/api/interview-entries/1", map[string]any{"display_order": "one"}, 400, "validation_failed"},
		{"unknown entry", "GET", "/api/interview-entries/99", nil, 404, "not_found"},
		{"unknown entry patch", "PATCH", "/api/interview-entries/99", map[string]any{"answer": "x"}, 404, "not_found"},
		{"unknown entry delete", "DELETE", "/api/interview-entries/99", nil, 404, "not_found"},
		{"invalid id", "GET", "/api/interview-entries/0", nil, 400, "invalid_id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wantErrorCode(t, ts.do(t, tc.method, tc.path, tc.body), tc.code, tc.err)
		})
	}
}
