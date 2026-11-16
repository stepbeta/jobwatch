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

// ---------- events ----------

func scanEventRow(row interface{ Scan(...any) error }) (model.Event, error) {
	var e model.Event
	var typ, occurred string
	var description, created sql.NullString
	if err := row.Scan(&e.ID, &e.OfferID, &typ, &description, &occurred, &created); err != nil {
		return model.Event{}, err
	}
	e.Type = model.EventType(typ)
	e.Description = strPtr(description)
	e.OccurredAt = parseTime(occurred)
	e.CreatedAt = parseNullTime(created)
	return e, nil
}

// InsertEvent stores a timeline event. OccurredAt is user-editable and
// may be backdated.
func InsertEvent(ctx context.Context, q Querier, e model.Event, now time.Time) (model.Event, error) {
	if e.OccurredAt.IsZero() {
		return model.Event{}, fmt.Errorf("insert event: occurred_at is required")
	}
	res, err := q.ExecContext(ctx,
		`INSERT INTO events(offer_id, type, description, occurred_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		e.OfferID, string(e.Type), nullStr(e.Description), model.FormatTime(e.OccurredAt), model.FormatTime(now))
	if err != nil {
		return model.Event{}, fmt.Errorf("insert event for offer %d: %w", e.OfferID, err)
	}
	e.ID, err = res.LastInsertId()
	if err != nil {
		return model.Event{}, fmt.Errorf("insert event for offer %d: %w", e.OfferID, err)
	}
	e.CreatedAt = now.UTC().Truncate(time.Second)
	return e, nil
}

// GetEvent returns the event with the given ID.
func GetEvent(ctx context.Context, q Querier, id int64) (model.Event, error) {
	e, err := scanEventRow(q.QueryRowContext(ctx,
		`SELECT id, offer_id, type, description, occurred_at, created_at FROM events WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Event{}, ErrNotFound
	}
	if err != nil {
		return model.Event{}, fmt.Errorf("get event %d: %w", id, err)
	}
	return e, nil
}

// ListEvents returns the timeline of an offer in chronological order.
func ListEvents(ctx context.Context, q Querier, offerID int64) ([]model.Event, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id, offer_id, type, description, occurred_at, created_at
		 FROM events WHERE offer_id = ? ORDER BY occurred_at, id`, offerID)
	if err != nil {
		return nil, fmt.Errorf("list events of offer %d: %w", offerID, err)
	}
	defer rows.Close()
	out := []model.Event{}
	for rows.Next() {
		e, err := scanEventRow(rows)
		if err != nil {
			return nil, fmt.Errorf("list events: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// EventUpdate is a partial update of an event. nil fields are left
// unchanged; an empty Description clears it.
type EventUpdate struct {
	Type        *model.EventType
	Description *string
	OccurredAt  *time.Time
}

// UpdateEvent applies u to an event.
func UpdateEvent(ctx context.Context, q Querier, id int64, u EventUpdate) error {
	var sets []string
	var args []any
	if u.Type != nil {
		sets = append(sets, "type = ?")
		args = append(args, string(*u.Type))
	}
	if u.Description != nil {
		sets = append(sets, "description = ?")
		var desc *string
		if *u.Description != "" {
			desc = u.Description
		}
		args = append(args, nullStr(desc))
	}
	if u.OccurredAt != nil {
		sets = append(sets, "occurred_at = ?")
		args = append(args, model.FormatTime(*u.OccurredAt))
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	res, err := q.ExecContext(ctx, `UPDATE events SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return fmt.Errorf("update event %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteEvent removes an event. Notes pinned to it have their pin
// cleared by the ON DELETE SET NULL foreign key.
func DeleteEvent(ctx context.Context, q Querier, id int64) error {
	res, err := q.ExecContext(ctx, `DELETE FROM events WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete event %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- notes ----------

func scanNoteRow(row interface{ Scan(...any) error }) (model.Note, error) {
	var n model.Note
	var entityType string
	var pinned sql.NullInt64
	var created, updated sql.NullString
	if err := row.Scan(&n.ID, &entityType, &n.EntityID, &n.Body, &pinned, &created, &updated); err != nil {
		return model.Note{}, err
	}
	n.EntityType = model.EntityType(entityType)
	n.PinnedEventID = int64Ptr(pinned)
	n.CreatedAt = parseNullTime(created)
	n.UpdatedAt = parseNullTime(updated)
	return n, nil
}

// InsertNote stores a note on an offer or a company.
func InsertNote(ctx context.Context, q Querier, n model.Note, now time.Time) (model.Note, error) {
	n.Body = strings.TrimSpace(n.Body)
	if n.Body == "" {
		return model.Note{}, fmt.Errorf("insert note: empty body")
	}
	ts := model.FormatTime(now)
	res, err := q.ExecContext(ctx,
		`INSERT INTO notes(entity_type, entity_id, body, pinned_event_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		string(n.EntityType), n.EntityID, n.Body, nullInt(n.PinnedEventID), ts, ts)
	if err != nil {
		return model.Note{}, fmt.Errorf("insert note: %w", err)
	}
	n.ID, err = res.LastInsertId()
	if err != nil {
		return model.Note{}, fmt.Errorf("insert note: %w", err)
	}
	n.CreatedAt, n.UpdatedAt = now.UTC().Truncate(time.Second), now.UTC().Truncate(time.Second)
	return n, nil
}

// GetNote returns the note with the given ID.
func GetNote(ctx context.Context, q Querier, id int64) (model.Note, error) {
	n, err := scanNoteRow(q.QueryRowContext(ctx,
		`SELECT id, entity_type, entity_id, body, pinned_event_id, created_at, updated_at FROM notes WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Note{}, ErrNotFound
	}
	if err != nil {
		return model.Note{}, fmt.Errorf("get note %d: %w", id, err)
	}
	return n, nil
}

// ListNotes returns the notes of an entity in creation order.
func ListNotes(ctx context.Context, q Querier, entityType model.EntityType, entityID int64) ([]model.Note, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id, entity_type, entity_id, body, pinned_event_id, created_at, updated_at
		 FROM notes WHERE entity_type = ? AND entity_id = ? ORDER BY created_at, id`,
		string(entityType), entityID)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()
	out := []model.Note{}
	for rows.Next() {
		n, err := scanNoteRow(rows)
		if err != nil {
			return nil, fmt.Errorf("list notes: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// NoteUpdate is a partial update of a note. nil fields are left
// unchanged; ClearPinned removes the event pin.
type NoteUpdate struct {
	Body          *string
	PinnedEventID *int64
	ClearPinned   bool
}

// UpdateNote applies u to a note.
func UpdateNote(ctx context.Context, q Querier, id int64, u NoteUpdate, now time.Time) error {
	var sets []string
	var args []any
	if u.Body != nil {
		body := strings.TrimSpace(*u.Body)
		if body == "" {
			return fmt.Errorf("update note %d: empty body", id)
		}
		sets = append(sets, "body = ?")
		args = append(args, body)
	}
	switch {
	case u.ClearPinned:
		sets = append(sets, "pinned_event_id = NULL")
	case u.PinnedEventID != nil:
		sets = append(sets, "pinned_event_id = ?")
		args = append(args, *u.PinnedEventID)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, model.FormatTime(now), id)
	res, err := q.ExecContext(ctx, `UPDATE notes SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return fmt.Errorf("update note %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteNote removes a note.
func DeleteNote(ctx context.Context, q Querier, id int64) error {
	res, err := q.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete note %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
