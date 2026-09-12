// Package domain holds Alexandryn's business logic: aggregates, value
// types, and repository interfaces. It maintains strict independence from
// transport and persistence implementations.
package domain

import "errors"

// Category represents one of a fixed, closed set of failure categories
// across the domain model.
type Category string

const (
	NotFound     Category = "NotFound"
	InvalidInput Category = "InvalidInput"
	Unauthorized Category = "Unauthorized"
	Conflict     Category = "Conflict"
	Unavailable  Category = "Unavailable"
	Internal     Category = "Internal"
)

// Error is the typed error that domain operations and repository
// implementations construct or wrap into before errors cross subsystem
// boundaries. Err carries server-side detail (such as a raw driver error);
// Message is safe for client-facing presentation.
type Error struct {
	Category Category
	Message  string
	Err      error
}

func (e *Error) Error() string { return e.Message }

func (e *Error) Unwrap() error { return e.Err }

// CategoryOf returns err's category if it (or an error it wraps) is a
// *Error, and Internal otherwise. This ensures every error maps cleanly
// to a known category.
func CategoryOf(err error) Category {
	var de *Error
	if errors.As(err, &de) {
		return de.Category
	}
	return Internal
}
