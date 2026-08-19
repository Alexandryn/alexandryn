package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// TranslateError converts a raw error from a pgx-based repository call
// into a *domain.Error carrying one of the six FR-1 categories
// (backend-errors-and-logging.md FR-3) — every repository method calls
// this at the point a query fails, so a raw pgx/driver error never
// crosses into domain-typed territory untranslated. The original error
// stays reachable via Unwrap, for server-side logging; the client-facing
// Message is always a fixed, generic string, never the driver error's
// own text, which can name a table, column, or constraint (FR-5: no
// internal identifier not meaningful to the caller).
//
// Only unique_violation is mapped today — the one case
// backend-errors-and-logging.md names by example. Every other SQLSTATE,
// and every non-*pgconn.PgError error (a connection failure, a context
// deadline), falls back to Internal — the same closed, defaulting
// mapping FR-4 requires one layer up. More codes get added here as real
// repository methods (phase 02-gated, T24) actually need them, not
// spec'd in advance of a concrete case.
func TranslateError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return &domain.Error{
				Category: domain.Conflict,
				Message:  "the requested change conflicts with an existing record",
				Err:      err,
			}
		}
	}

	return &domain.Error{
		Category: domain.Internal,
		Message:  "an internal error occurred",
		Err:      err,
	}
}
