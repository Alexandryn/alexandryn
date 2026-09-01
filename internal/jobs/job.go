// Package jobs is Alexandryn's background job queue: a jobs table in the
// shared PostgreSQL instance, a worker pool that claims rows via
// SELECT ... FOR UPDATE SKIP LOCKED, retry with exponential backoff,
// dead-lettering after a bounded attempt count, crash recovery via a
// lease-plus-heartbeat reaper, and an internal status query API
// (backend-job-queue.md, ADR 0014).
//
// This package owns the Job type entirely. Job is a persistence-layer
// concept, not a domain aggregate: internal/domain never imports this
// package (architecture-backend.md FR-2). This package imports
// internal/domain only for IDGenerator and the typed Error.
package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ID is a job's unique identifier — a UUID string produced Go-side by the
// same domain.IDGenerator the rest of the codebase uses.
type ID string

// Kind names a registered handler. A job's kind is matched against an
// in-process registry (Registry); it is never used to load or execute
// code named by a caller (backend-job-queue.md Security considerations).
type Kind string

// State is a job's position in its lifecycle. The set is closed and
// mirrors the jobs_status_check constraint in migration 00006.
type State string

const (
	// StateQueued: enqueued, never yet claimed.
	StateQueued State = "queued"
	// StateRunning: claimed by a worker, handler executing.
	StateRunning State = "running"
	// StateRetrying: a prior attempt failed; waiting for available_at.
	StateRetrying State = "retrying"
	// StateCompleted: handler returned nil. Terminal.
	StateCompleted State = "completed"
	// StateDeadLetter: attempts exhausted, or a Permanent failure.
	// Terminal — this system will not retry it automatically.
	StateDeadLetter State = "dead_letter"
)

// IsTerminal reports whether s is an end state no transition leaves.
func (s State) IsTerminal() bool {
	return s == StateCompleted || s == StateDeadLetter
}

// CanTransitionTo reports whether a job in state s may move to next.
// A job that needs to run again after a terminal state is a new enqueue,
// never a resurrection (backend-job-queue.md State transitions).
func (s State) CanTransitionTo(next State) bool {
	switch s {
	case StateQueued, StateRetrying:
		return next == StateRunning
	case StateRunning:
		return next == StateCompleted || next == StateRetrying || next == StateDeadLetter
	default: // completed, dead_letter, or an unknown value
		return false
	}
}

// Progress is a handler's optional sub-progress report (FR-8). A job that
// never reports has Progress == nil, a legal and common case.
type Progress struct {
	Current int `json:"current"`
	Total   int `json:"total"`
}

// Job is one row of the jobs table.
type Job struct {
	ID          ID
	Kind        Kind
	Payload     json.RawMessage
	State       State
	Attempts    int
	MaxAttempts int
	AvailableAt time.Time
	LockedUntil *time.Time
	LeaseToken  string
	LockedBy    string
	LastError   string
	Progress    *Progress
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
}

// JobFilter narrows ListJobs. A zero-value field means "any".
type JobFilter struct {
	Kind  Kind
	State State
}

// HandlerFunc runs one job. payload is the raw JSON enqueued for this
// kind. report may be called any number of times to record sub-progress;
// a handler with no meaningful sub-progress simply never calls it.
//
// A returned error is a transient failure: the job is retried per FR-6/
// FR-7 until max_attempts, then dead-lettered. Wrap the error in
// Permanent to force immediate dead-lettering instead. A panic is
// recovered by the worker and treated exactly as a returned error.
//
// ctx is cancelled on shutdown and on lease loss (FR-5/FR-10). A handler
// performing non-idempotent, irreversible side effects should check
// ctx.Err() between steps and abort early.
type HandlerFunc func(ctx context.Context, payload json.RawMessage, report ReportProgressFunc) error

// ReportProgressFunc records that current of total units are done.
type ReportProgressFunc func(current, total int)

// permanentError marks a failure a handler knows retrying will not fix
// (a source's 4xx, a malformed payload) — distinct from an ordinary
// transient error.
type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

// Permanent wraps err so the worker dead-letters the job immediately,
// regardless of attempts remaining (FR-6). Permanent(nil) is nil.
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &permanentError{err: err}
}

// IsPermanent reports whether err (or anything it wraps) was marked
// Permanent.
func IsPermanent(err error) bool {
	var p *permanentError
	return errors.As(err, &p)
}
