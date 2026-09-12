package jobs

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// translateError converts a raw pgx/driver error into a typed
// *domain.Error, matching the mapping used by internal/persistence/postgres.
// The raw error stays reachable via Unwrap for server-side logging; the
// client-facing Message is always a fixed generic string.
func translateError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return &domain.Error{
			Category: domain.Conflict,
			Message:  "the job conflicts with an existing record",
			Err:      err,
		}
	}
	return &domain.Error{
		Category: domain.Internal,
		Message:  "an internal job store error occurred",
		Err:      err,
	}
}

// notFound builds a NotFound *domain.Error for a missing job row.
func notFound() error {
	return &domain.Error{Category: domain.NotFound, Message: "job not found"}
}
