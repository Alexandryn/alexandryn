package postgres_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// A Postgres-specific error MUST be translated to a domain category inside
// internal/persistence/postgres, before it crosses into internal/domain-typed
// territory — the raw driver error MUST NOT be returned from a repository method
// un-translated. These are synthetic *pgconn.PgError values; the integration
// test confirms pgx actually produces this shape for real.

func TestTranslateError_UniqueViolationBecomesConflict(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}

	got := postgres.TranslateError(pgErr)

	if domain.CategoryOf(got) != domain.Conflict {
		t.Fatalf("category = %v, want Conflict", domain.CategoryOf(got))
	}
}

// Translation surviving standard-library wrapping: a repository method might
// wrap the driver error with additional context before the category is
// extracted — proving TranslateError works even when wrapped.
func TestTranslateError_SurvivesWrapping(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}
	wrapped := fmt.Errorf("insert into works: %w", pgErr)

	got := postgres.TranslateError(wrapped)

	if domain.CategoryOf(got) != domain.Conflict {
		t.Fatalf("category = %v, want Conflict even through a wrapped error", domain.CategoryOf(got))
	}
}

// An unrecognized SQLSTATE falls back to Internal, never a guessed category.
func TestTranslateError_UnrecognizedCodeDefaultsToInternal(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "99999", Message: "some code this package doesn't map"}

	got := postgres.TranslateError(pgErr)

	if domain.CategoryOf(got) != domain.Internal {
		t.Fatalf("category = %v, want Internal for an unrecognized SQLSTATE", domain.CategoryOf(got))
	}
}

// A non-pgx error (a connection failure, a context deadline) isn't a
// *pgconn.PgError at all — also defaults to Internal, never a panic or a
// type assertion failure inside the translator itself.
func TestTranslateError_NonPgErrorDefaultsToInternal(t *testing.T) {
	got := postgres.TranslateError(errors.New("connection reset by peer"))

	if domain.CategoryOf(got) != domain.Internal {
		t.Fatalf("category = %v, want Internal for a non-PgError", domain.CategoryOf(got))
	}
}

// The translated error's client-facing Message must never be the raw
// driver error's own text — that can carry table/column/constraint names.
// The raw error is still reachable server-side via Unwrap, for logging.
func TestTranslateError_MessageNeverEchoesTheRawDriverError(t *testing.T) {
	pgErr := &pgconn.PgError{
		Code:           "23505",
		Message:        "duplicate key value violates unique constraint \"works_isbn_key\"",
		ConstraintName: "works_isbn_key",
		TableName:      "works",
	}

	got := postgres.TranslateError(pgErr)

	var de *domain.Error
	if !errors.As(got, &de) {
		t.Fatalf("TranslateError did not return a *domain.Error: %v", got)
	}
	if de.Message == pgErr.Message {
		t.Fatalf("translated Message echoes the raw driver error verbatim: %q", de.Message)
	}
	if !errors.Is(got, pgErr) {
		t.Fatal("the raw driver error is no longer reachable via errors.Is/Unwrap — server-side logging needs it")
	}
}
