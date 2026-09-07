package observability

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EventStore manages persistence and retrieval of system events in PostgreSQL.
type EventStore struct {
	pool  *pgxpool.Pool
	nowFn func() time.Time
}

// NewEventStore constructs an EventStore. If nowFn is nil, time.Now is used.
func NewEventStore(pool *pgxpool.Pool, nowFn func() time.Time) *EventStore {
	if nowFn == nil {
		nowFn = time.Now
	}
	return &EventStore{
		pool:  pool,
		nowFn: nowFn,
	}
}

// RecordEvent inserts an event into system_events with sanitised payload and computed purge_at.
func (s *EventStore) RecordEvent(ctx context.Context, ev NewSystemEvent) error {
	if s.pool == nil {
		return errors.New("event store pool is nil")
	}

	now := s.nowFn().UTC()
	purgeAt := ev.CalculatePurgeAt(now)
	cleanPayload := ev.SanitizedPayload()

	payloadBytes, err := json.Marshal(cleanPayload)
	if err != nil {
		return err
	}

	query := `INSERT INTO system_events
		(event_kind, job_id, library_id, user_id, payload, created_at, purge_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err = s.pool.Exec(ctx, query,
		ev.EventKind,
		ev.JobID,
		ev.LibraryID,
		ev.UserID,
		payloadBytes,
		now,
		purgeAt,
	)
	return err
}

// ListEvents queries the recent events for a library (or host events if libraryID is nil).
func (s *EventStore) ListEvents(ctx context.Context, libraryID *string, limit int) ([]SystemEvent, error) {
	if s.pool == nil {
		return nil, errors.New("event store pool is nil")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	var (
		rows pgx.Rows
		err  error
	)

	if libraryID != nil && *libraryID != "" {
		query := `SELECT id, event_kind, job_id, library_id, user_id, payload, created_at, purge_at
			FROM system_events
			WHERE (library_id = $1 OR library_id IS NULL)
			ORDER BY created_at DESC
			LIMIT $2`
		rows, err = s.pool.Query(ctx, query, *libraryID, limit)
	} else {
		query := `SELECT id, event_kind, job_id, library_id, user_id, payload, created_at, purge_at
			FROM system_events
			WHERE library_id IS NULL
			ORDER BY created_at DESC
			LIMIT $1`
		rows, err = s.pool.Query(ctx, query, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []SystemEvent
	for rows.Next() {
		var (
			ev           SystemEvent
			payloadBytes []byte
		)
		err := rows.Scan(
			&ev.ID,
			&ev.EventKind,
			&ev.JobID,
			&ev.LibraryID,
			&ev.UserID,
			&payloadBytes,
			&ev.CreatedAt,
			&ev.PurgeAt,
		)
		if err != nil {
			return nil, err
		}
		if len(payloadBytes) > 0 {
			if err := json.Unmarshal(payloadBytes, &ev.Payload); err != nil {
				ev.Payload = map[string]any{}
			}
		} else {
			ev.Payload = map[string]any{}
		}
		events = append(events, ev)
	}
	return events, rows.Err()
}

// PurgeExpired deletes events whose purge_at is before now. Returns the number of rows deleted.
func (s *EventStore) PurgeExpired(ctx context.Context, now time.Time) (int64, error) {
	if s.pool == nil {
		return 0, errors.New("event store pool is nil")
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM system_events WHERE purge_at < $1`, now)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
