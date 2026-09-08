package observability

import (
	"context"
	"strings"
	"time"
)

// Standard system event kind constants (ADR 0031).
const (
	EventJobEnqueued    = "job.enqueued"
	EventJobRunning     = "job.running"
	EventJobCompleted   = "job.completed"
	EventJobFailed      = "job.failed"
	EventJobDeadLetter  = "job.dead_letter"
	EventImportStarted  = "import.started"
	EventImportFinished = "import.finished"
	EventSourceSync     = "source.sync"
)

// DefaultRetentionDays is the fallback retention window if unspecified.
const DefaultRetentionDays = 30

var prohibitedPayloadKeys = []string{
	"password", "passwd", "token", "secret", "apikey", "api_key",
	"authorization", "position", "cfi", "location", "percentage",
	"pct", "chapter", "note",
}

// SystemEvent represents an immutable row in the system_events ledger.
type SystemEvent struct {
	ID        int64          `json:"id"`
	EventKind string         `json:"event_kind"`
	JobID     *string        `json:"job_id,omitempty"`
	LibraryID *string        `json:"library_id,omitempty"`
	UserID    *string        `json:"user_id,omitempty"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
	PurgeAt   time.Time      `json:"purge_at"`
}

// NewSystemEvent holds input data for inserting a system event.
type NewSystemEvent struct {
	EventKind     string
	JobID         *string
	LibraryID     *string
	UserID        *string
	Payload       map[string]any
	RetentionDays int
}

// CalculatePurgeAt returns the retention timestamp given the baseline time.
func (e NewSystemEvent) CalculatePurgeAt(now time.Time) time.Time {
	days := e.RetentionDays
	if days <= 0 {
		days = DefaultRetentionDays
	}
	return now.Add(time.Duration(days) * 24 * time.Hour)
}

// SanitizedPayload returns a deep copy of the payload with every
// prohibited sensitive key removed at any nesting depth. Recursion
// matters: a reading-progress event carries its cfi/percentage inside a
// nested "detail" object, and a highlight event carries the highlighted
// passage under "note" — a top-level-only strip let both through into
// system_events and the activity feed (audit 0016 #296).
func (e NewSystemEvent) SanitizedPayload() map[string]any {
	if e.Payload == nil {
		return map[string]any{}
	}
	return sanitizeMap(e.Payload)
}

func sanitizeMap(m map[string]any) map[string]any {
	clean := make(map[string]any, len(m))
	for k, v := range m {
		if isProhibitedKey(k) {
			continue
		}
		clean[k] = sanitizeValue(v)
	}
	return clean
}

func sanitizeValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return sanitizeMap(t)
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = sanitizeValue(e)
		}
		return out
	default:
		return v
	}
}

func isProhibitedKey(key string) bool {
	lower := strings.ToLower(key)
	for _, p := range prohibitedPayloadKeys {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// SystemEventWriter writes audit and activity events into storage.
type SystemEventWriter interface {
	RecordEvent(ctx context.Context, ev NewSystemEvent) error
}
