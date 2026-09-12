package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// reaperLastError is the fixed last_error a reaper reclaim records.
const reaperLastError = "worker lease expired without heartbeat"

// Store is the jobs table's data-access layer. It owns no clock: every
// method that reads or writes a time takes it as an explicit parameter,
// sourced from the engine's injected clock, never SQL's own now() — so a
// test governs claimability and lease expiry by advancing a FakeClock
// with no real sleep.
type Store struct {
	pool          *pgxpool.Pool
	ids           domain.IDGenerator
	leaseDuration time.Duration
}

// NewStore builds a Store over the shared connection pool. ids generates
// fencing lease tokens (one per claim and per reaper reclaim);
// leaseDuration is how far ahead of now a claim or heartbeat sets
// locked_until.
func NewStore(pool *pgxpool.Pool, ids domain.IDGenerator, leaseDuration time.Duration) *Store {
	return &Store{pool: pool, ids: ids, leaseDuration: leaseDuration}
}

// NewJob is the input to Enqueue.
type NewJob struct {
	ID          ID
	Kind        Kind
	Payload     json.RawMessage
	MaxAttempts int
	AvailableAt time.Time
	Now         time.Time
}

const jobColumns = `id, kind, payload, status, attempts, max_attempts,
	available_at, locked_until, lease_token, locked_by, last_error,
	progress, created_at, updated_at, completed_at`

// Enqueue inserts a new job in state queued.
func (s *Store) Enqueue(ctx context.Context, j NewJob) error {
	payload := j.Payload
	if len(payload) == 0 {
		payload = json.RawMessage("null")
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO jobs
		(id, kind, payload, status, attempts, max_attempts, available_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'queued', 0, $4, $5, $6, $6)`,
		string(j.ID), string(j.Kind), []byte(payload), j.MaxAttempts, j.AvailableAt, j.Now)
	if err != nil {
		return translateError(err)
	}
	return nil
}

// ClaimNext claims the single oldest claimable job whose kind is in
// kinds (nil means any), moving it to running with a fresh lease token.
// It returns nil, nil when nothing is claimable.
//
// The correctness property is FOR UPDATE's row lock, held to commit: two
// concurrent workers cannot both read the same row and both write
// status = 'running'. SKIP LOCKED is throughput only — a worker steps
// over a row another worker is mid-claim on rather than blocking.
func (s *Store) ClaimNext(ctx context.Context, workerID string, kinds []Kind, now time.Time) (*Job, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, translateError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful commit

	var id string
	if len(kinds) == 0 {
		err = tx.QueryRow(ctx, `SELECT id FROM jobs
			WHERE status IN ('queued', 'retrying') AND available_at <= $1
			ORDER BY available_at
			LIMIT 1 FOR UPDATE SKIP LOCKED`, now).Scan(&id)
	} else {
		err = tx.QueryRow(ctx, `SELECT id FROM jobs
			WHERE status IN ('queued', 'retrying') AND available_at <= $1 AND kind = ANY($2)
			ORDER BY available_at
			LIMIT 1 FOR UPDATE SKIP LOCKED`, now, kindStrings(kinds)).Scan(&id)
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, translateError(err)
	}

	token := s.ids.NewID()
	_, err = tx.Exec(ctx, `UPDATE jobs SET
		status = 'running',
		locked_until = $2,
		lease_token = $3,
		locked_by = $4,
		attempts = attempts + 1,
		updated_at = $5
		WHERE id = $1`,
		id, now.Add(s.leaseDuration), token, workerID, now)
	if err != nil {
		return nil, translateError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, translateError(err)
	}

	job, err := s.GetJob(ctx, ID(id))
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// Heartbeat extends the lease of a running job the caller still owns.
// It reports false when zero rows matched — the lease has been reclaimed
// (the token no longer matches) and the caller must stop.
func (s *Store) Heartbeat(ctx context.Context, id ID, leaseToken string, now time.Time) (bool, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE jobs SET locked_until = $3, updated_at = $4
		WHERE id = $1 AND lease_token = $2 AND status = 'running'`,
		string(id), leaseToken, now.Add(s.leaseDuration), now)
	if err != nil {
		return false, translateError(err)
	}
	return tag.RowsAffected() == 1, nil
}

// Complete marks a job completed, fenced on the lease token. It
// reports false when the lease was already reclaimed and the write hit
// zero rows — the caller discards its result and logs it as a harmless
// late write, not a new failure.
func (s *Store) Complete(ctx context.Context, id ID, leaseToken string, now time.Time) (bool, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE jobs SET
		status = 'completed', completed_at = $3, locked_until = NULL, updated_at = $3
		WHERE id = $1 AND lease_token = $2 AND status = 'running'`,
		string(id), leaseToken, now)
	if err != nil {
		return false, translateError(err)
	}
	return tag.RowsAffected() == 1, nil
}

// Fail applies the outcome for a handler failure, fenced on the lease
// token. deadLetter chooses dead_letter (attempts exhausted, or a
// Permanent error) over retrying; nextRunAt is the backoff-delayed
// available_at for the retrying case (ignored for dead_letter). cause's
// text is truncated and stored via redactError. Reports false on a
// fenced zero-row write.
func (s *Store) Fail(ctx context.Context, id ID, leaseToken string, now, nextRunAt time.Time, deadLetter bool, cause error) (bool, error) {
	next := StateRetrying
	if deadLetter {
		next = StateDeadLetter
	}
	tag, err := s.pool.Exec(ctx, `UPDATE jobs SET
		status = $3, available_at = $4, last_error = $5, locked_until = NULL, updated_at = $6
		WHERE id = $1 AND lease_token = $2 AND status = 'running'`,
		string(id), leaseToken, string(next), nextRunAt, redactError(cause), now)
	if err != nil {
		return false, translateError(err)
	}
	return tag.RowsAffected() == 1, nil
}

// UpdateProgress writes a handler's sub-progress in its own statement,
// fenced on the lease token so a reclaimed worker cannot write progress
// for a job it no longer owns. A zero-row write is a no-op, not
// an error — progress is best-effort and must never block the job.
func (s *Store) UpdateProgress(ctx context.Context, id ID, leaseToken string, current, total int, now time.Time) error {
	p, err := json.Marshal(Progress{Current: current, Total: total})
	if err != nil {
		return translateError(err)
	}
	_, err = s.pool.Exec(ctx, `UPDATE jobs SET progress = $3, updated_at = $4
		WHERE id = $1 AND lease_token = $2 AND status = 'running'`,
		string(id), leaseToken, p, now)
	if err != nil {
		return translateError(err)
	}
	return nil
}

// RecoverStale is the reaper sweep: every running job whose lease
// expired with no heartbeat is reclaimed in its own short transaction —
// a new lease token (so the original worker's token is stale
// everywhere), retry/dead-letter comparison against the attempts
// value the original claim already set (never re-incremented here), and
// a fixed last_error. backoffFor computes the retrying delay; it is
// passed in so the engine owns the jitter source. Returns how many jobs
// were reclaimed.
func (s *Store) RecoverStale(ctx context.Context, now time.Time, backoffFor func(attempts int) time.Duration) (int, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM jobs
		WHERE status = 'running' AND locked_until < $1`, now)
	if err != nil {
		return 0, translateError(err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, translateError(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, translateError(err)
	}

	recovered := 0
	for _, id := range ids {
		ok, err := s.reclaimOne(ctx, id, now, backoffFor)
		if err != nil {
			return recovered, err
		}
		if ok {
			recovered++
		}
	}
	return recovered, nil
}

func (s *Store) reclaimOne(ctx context.Context, id string, now time.Time, backoffFor func(attempts int) time.Duration) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, translateError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var attempts, maxAttempts int
	err = tx.QueryRow(ctx, `SELECT attempts, max_attempts FROM jobs
		WHERE id = $1 AND status = 'running' AND locked_until < $2
		FOR UPDATE SKIP LOCKED`, id, now).Scan(&attempts, &maxAttempts)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil // another sweep, or the worker itself, got there first
		}
		return false, translateError(err)
	}

	next := StateRetrying
	availableAt := now.Add(backoffFor(attempts))
	if attempts >= maxAttempts {
		next = StateDeadLetter
		availableAt = now
	}

	_, err = tx.Exec(ctx, `UPDATE jobs SET
		status = $2, available_at = $3, lease_token = $4, locked_by = NULL,
		locked_until = NULL, last_error = $5, updated_at = $6
		WHERE id = $1`,
		id, string(next), availableAt, s.ids.NewID(), reaperLastError, now)
	if err != nil {
		return false, translateError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, translateError(err)
	}
	return true, nil
}

// GetJob returns one job or a NotFound *domain.Error.
func (s *Store) GetJob(ctx context.Context, id ID) (Job, error) {
	return scanJob(s.pool.QueryRow(ctx, `SELECT `+jobColumns+` FROM jobs WHERE id = $1`, string(id)))
}

// ListJobs returns jobs matching filter (zero-value fields match any),
// oldest first.
func (s *Store) ListJobs(ctx context.Context, filter JobFilter) ([]Job, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+jobColumns+` FROM jobs
		WHERE ($1 = '' OR kind = $1) AND ($2 = '' OR status = $2)
		ORDER BY created_at, id`,
		string(filter.Kind), string(filter.State))
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	var out []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, translateError(err)
	}
	return out, nil
}

// CountByState returns the count of jobs grouped by status. Every state is present
// in the returned map even if its count is zero.
func (s *Store) CountByState(ctx context.Context) (map[State]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT status, COUNT(*) FROM jobs GROUP BY status`)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	counts := map[State]int{
		StateQueued:     0,
		StateRunning:    0,
		StateRetrying:   0,
		StateCompleted:  0,
		StateDeadLetter: 0,
	}
	for rows.Next() {
		var (
			st    string
			count int
		)
		if err := rows.Scan(&st, &count); err != nil {
			return nil, translateError(err)
		}
		counts[State(st)] = count
	}
	return counts, rows.Err()
}

// CancelJob transitions a queued or running job to dead_letter with last_error = "cancelled_by_admin".
func (s *Store) CancelJob(ctx context.Context, id ID, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return translateError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM jobs WHERE id = $1 FOR UPDATE`, string(id)).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notFound()
		}
		return translateError(err)
	}

	st := State(status)
	if st.IsTerminal() {
		return &domain.Error{Category: domain.Conflict, Message: "job is already in a terminal state"}
	}

	const cancelReason = "cancelled_by_admin"
	_, err = tx.Exec(ctx, `UPDATE jobs
		SET status = 'dead_letter', last_error = $1, completed_at = $2, locked_until = NULL, updated_at = $2
		WHERE id = $3`, cancelReason, now, string(id))
	if err != nil {
		return translateError(err)
	}

	return tx.Commit(ctx)
}

// RetryJob re-enqueues a dead-letter job as a fresh record (new id,
// attempts = 0, available_at = now). Only a dead-letter job is
// retryable: retrying a queued, running, or retrying job would put a
// duplicate copy on the queue, and a completed job has already run.
func (s *Store) RetryJob(ctx context.Context, id ID, newID ID, now time.Time) (ID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", translateError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		kind        string
		payload     []byte
		status      string
		maxAttempts int
	)
	err = tx.QueryRow(ctx, `SELECT kind, payload, status, max_attempts FROM jobs WHERE id = $1`, string(id)).
		Scan(&kind, &payload, &status, &maxAttempts)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", notFound()
		}
		return "", translateError(err)
	}

	if State(status) != StateDeadLetter {
		return "", &domain.Error{
			Category: domain.Conflict,
			Message:  "only a dead-letter job can be retried; this job is " + status,
		}
	}

	if len(payload) == 0 {
		payload = []byte("null")
	}

	_, err = tx.Exec(ctx, `INSERT INTO jobs
		(id, kind, payload, status, attempts, max_attempts, available_at, created_at, updated_at)
		VALUES ($1, $2, $3, 'queued', 0, $4, $5, $6, $6)`,
		string(newID), kind, payload, maxAttempts, now, now)
	if err != nil {
		return "", translateError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", translateError(err)
	}
	return newID, nil
}

// ClearCompleted deletes completed jobs older than cutoff.
func (s *Store) ClearCompleted(ctx context.Context, olderThan time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM jobs
		WHERE status = 'completed' AND (completed_at <= $1 OR (completed_at IS NULL AND updated_at <= $1))`, olderThan)
	if err != nil {
		return 0, translateError(err)
	}
	return tag.RowsAffected(), nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanJob(row scannable) (Job, error) {
	var j Job
	var (
		status                          string
		payload, progressRaw            []byte
		lockedUntil, completedAt        *time.Time
		leaseToken, lockedBy, lastError *string
	)
	err := row.Scan(
		(*string)(&j.ID), (*string)(&j.Kind), &payload, &status,
		&j.Attempts, &j.MaxAttempts, &j.AvailableAt, &lockedUntil,
		&leaseToken, &lockedBy, &lastError, &progressRaw,
		&j.CreatedAt, &j.UpdatedAt, &completedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Job{}, notFound()
		}
		return Job{}, translateError(err)
	}

	j.State = State(status)
	j.Payload = json.RawMessage(payload)
	j.LockedUntil = lockedUntil
	j.CompletedAt = completedAt
	if leaseToken != nil {
		j.LeaseToken = *leaseToken
	}
	if lockedBy != nil {
		j.LockedBy = *lockedBy
	}
	if lastError != nil {
		j.LastError = RedactedText(*lastError)
	}
	if len(progressRaw) > 0 {
		var p Progress
		if err := json.Unmarshal(progressRaw, &p); err != nil {
			return Job{}, translateError(err)
		}
		j.Progress = &p
	}
	return j, nil
}

func kindStrings(kinds []Kind) []string {
	out := make([]string, len(kinds))
	for i, k := range kinds {
		out[i] = string(k)
	}
	return out
}
