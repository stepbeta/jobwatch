package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
)

// ---------- interview entries ----------

const interviewEntryColumns = `id, question, answer, answer_format, display_order, created_at, updated_at`

func scanInterviewEntry(row interface{ Scan(...any) error }) (model.InterviewEntry, error) {
	var e model.InterviewEntry
	var format string
	var order sql.NullInt64
	var created, updated sql.NullString
	if err := row.Scan(&e.ID, &e.Question, &e.Answer, &format, &order, &created, &updated); err != nil {
		return model.InterviewEntry{}, err
	}
	e.AnswerFormat = model.AnswerFormat(format)
	e.DisplayOrder = intPtr(order)
	e.CreatedAt = parseNullTime(created)
	e.UpdatedAt = parseNullTime(updated)
	return e, nil
}

// InsertInterviewEntry stores a new interview-prep entry. The question is
// trimmed and must not be empty; the answer may be empty at creation.
// An unset answer format defaults to markdown.
func InsertInterviewEntry(ctx context.Context, q Querier, e model.InterviewEntry, now time.Time) (model.InterviewEntry, error) {
	e.Question = strings.TrimSpace(e.Question)
	if e.Question == "" {
		return model.InterviewEntry{}, fmt.Errorf("insert interview entry: empty question")
	}
	if e.AnswerFormat == "" {
		e.AnswerFormat = model.AnswerFormatMarkdown
	}
	if !e.AnswerFormat.Valid() {
		return model.InterviewEntry{}, fmt.Errorf("insert interview entry: invalid answer_format %q", e.AnswerFormat)
	}
	ts := model.FormatTime(now)
	res, err := q.ExecContext(ctx,
		`INSERT INTO interview_entries(question, answer, answer_format, display_order, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.Question, e.Answer, string(e.AnswerFormat), nullIntFromInt(e.DisplayOrder), ts, ts)
	if err != nil {
		return model.InterviewEntry{}, fmt.Errorf("insert interview entry: %w", err)
	}
	e.ID, err = res.LastInsertId()
	if err != nil {
		return model.InterviewEntry{}, fmt.Errorf("insert interview entry: %w", err)
	}
	e.CreatedAt, e.UpdatedAt = now.UTC().Truncate(time.Second), now.UTC().Truncate(time.Second)
	return e, nil
}

// GetInterviewEntry returns the entry with the given ID.
func GetInterviewEntry(ctx context.Context, q Querier, id int64) (model.InterviewEntry, error) {
	e, err := scanInterviewEntry(q.QueryRowContext(ctx,
		`SELECT `+interviewEntryColumns+` FROM interview_entries WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.InterviewEntry{}, ErrNotFound
	}
	if err != nil {
		return model.InterviewEntry{}, fmt.Errorf("get interview entry %d: %w", id, err)
	}
	return e, nil
}

// ListInterviewEntries returns every entry: rows with an explicit
// display_order first (ascending), then rows without one by id.
func ListInterviewEntries(ctx context.Context, q Querier) ([]model.InterviewEntry, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT `+interviewEntryColumns+` FROM interview_entries
		 ORDER BY display_order IS NULL, display_order, id`)
	if err != nil {
		return nil, fmt.Errorf("list interview entries: %w", err)
	}
	defer rows.Close()
	out := []model.InterviewEntry{}
	for rows.Next() {
		e, err := scanInterviewEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("list interview entries: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// InterviewEntryUpdate is a partial update. nil fields are left
// unchanged; ClearDisplayOrder removes the manual position.
type InterviewEntryUpdate struct {
	Question          *string
	Answer            *string
	DisplayOrder      *int
	ClearDisplayOrder bool
}

// UpdateInterviewEntry applies u to an entry and bumps updated_at.
func UpdateInterviewEntry(ctx context.Context, q Querier, id int64, u InterviewEntryUpdate, now time.Time) error {
	var sets []string
	var args []any
	if u.Question != nil {
		question := strings.TrimSpace(*u.Question)
		if question == "" {
			return fmt.Errorf("update interview entry %d: empty question", id)
		}
		sets = append(sets, "question = ?")
		args = append(args, question)
	}
	if u.Answer != nil {
		sets = append(sets, "answer = ?")
		args = append(args, *u.Answer)
	}
	switch {
	case u.ClearDisplayOrder:
		sets = append(sets, "display_order = NULL")
	case u.DisplayOrder != nil:
		sets = append(sets, "display_order = ?")
		args = append(args, *u.DisplayOrder)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, model.FormatTime(now), id)
	res, err := q.ExecContext(ctx,
		`UPDATE interview_entries SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return fmt.Errorf("update interview entry %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteInterviewEntry removes an entry.
func DeleteInterviewEntry(ctx context.Context, q Querier, id int64) error {
	res, err := q.ExecContext(ctx, `DELETE FROM interview_entries WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete interview entry %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
