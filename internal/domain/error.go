// Package domain holds Alexandryn's business logic: aggregates, value
// types, and repository interfaces (architecture-backend.md FR-1). It
// imports nothing from internal/transport or internal/persistence — the
// dependency direction runs the other way (FR-2/FR-3).
package domain

import "errors"

// Category is one of a fixed, closed set of six failure categories
// (backend-errors-and-logging.md FR-1). A new category MUST NOT be added
// without amending that spec.
type Category string

const (
	NotFound     Category = "NotFound"
	InvalidInput Category = "InvalidInput"
	Unauthorized Category = "Unauthorized"
	Conflict     Category = "Conflict"
	Unavailable  Category = "Unavailable"
	Internal     Category = "Internal"
)

// Error is the typed error every domain operation and repository
// implementation constructs or wraps into before an error crosses the
// transport boundary (FR-3) — transport never receives a bare error and
// has to guess its category. Err carries server-side-only detail (a raw
// driver error, for example); Message is what a client-facing response
// is built from, never Err's own text.
type Error struct {
	Category Category
	Message  string
	Err      error
}

func (e *Error) Error() string { return e.Message }

func (e *Error) Unwrap() error { return e.Err }

// CategoryOf returns err's category if it (or something it wraps) is a
// *Error, and Internal otherwise — FR-4's closed, total mapping on the
// input side: nothing reaches the client without going through one of
// the six categories, including a nil error, an unexpected panic's
// recovered value, or an un-translated third-party error.
func CategoryOf(err error) Category {
	var de *Error
	if errors.As(err, &de) {
		return de.Category
	}
	return Internal
}
