//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// FR-3, confirmed against reality: a genuine unique-constraint violation
// triggered against a real PostgreSQL instance surfaces as pgx's actual
// *pgconn.PgError shape, and the same TranslateError used by the Unit
// layer's synthetic test correctly categorizes it as Conflict — proof
// the synthetic fixture wasn't testing an imagined shape.
func TestTranslateError_RealUniqueViolationBecomesConflict(t *testing.T) {
	db := testDB(t)
	resetSchema(t, db)

	if _, err := db.ExecContext(context.Background(),
		"CREATE TABLE errors_fr3_fixture (id int PRIMARY KEY)"); err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		"INSERT INTO errors_fr3_fixture (id) VALUES (1)"); err != nil {
		t.Fatalf("first INSERT: %v", err)
	}

	_, dupErr := db.ExecContext(context.Background(),
		"INSERT INTO errors_fr3_fixture (id) VALUES (1)")
	if dupErr == nil {
		t.Fatal("second INSERT with a duplicate key succeeded, want a unique-constraint violation")
	}

	var pgErr *pgconn.PgError
	if !errors.As(dupErr, &pgErr) {
		t.Fatalf("real driver error isn't a *pgconn.PgError: %#v (%T)", dupErr, dupErr)
	}
	if pgErr.Code != "23505" {
		t.Fatalf("real driver error Code = %q, want 23505 (unique_violation) — the Unit-layer synthetic fixture assumed the wrong shape if this fails", pgErr.Code)
	}

	got := postgres.TranslateError(dupErr)
	if domain.CategoryOf(got) != domain.Conflict {
		t.Fatalf("category = %v, want Conflict for a real unique-constraint violation", domain.CategoryOf(got))
	}
}
