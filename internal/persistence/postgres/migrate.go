package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver goose needs
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// connectionFailure marks a migration failure caused by not being able
// to reach the database at all, distinct from a genuine SQL/schema
// failure once connected — ensuring a connection failure's own error text
// (which can embed the DSN) is never surfaced, while a real SQL failure's
// diagnostic detail (which carries no connection-string risk) is retained.
type connectionFailure struct{ err error }

func (e *connectionFailure) Error() string { return e.err.Error() }
func (e *connectionFailure) Unwrap() error { return e.err }

// ConnectionFailure wraps err as a connection-class migration failure.
// Exported for the regression test constructing a fake one; production
// code reaches this only through goUp below.
func ConnectionFailure(err error) error { return &connectionFailure{err: err} }

// RunMigrations opens a short-lived *sql.DB via open, runs up against it,
// and closes it immediately afterward regardless of outcome — never
// passed to or reused by any repository.
// open and up are injected so this is provable without a real database;
// production callers use Migrate, below.
func RunMigrations(ctx context.Context, databaseURL string, open func(driverName, dataSourceName string) (*sql.DB, error), up func(ctx context.Context, db *sql.DB) error) error {
	db, err := open("pgx", databaseURL)
	if err != nil {
		return errors.New("could not open a connection for migrations")
	}
	defer func() { _ = db.Close() }()

	if err := up(ctx, db); err != nil {
		var cf *connectionFailure
		if errors.As(err, &cf) {
			return errors.New("could not connect to the database for migrations")
		}
		return fmt.Errorf("migration failed: %w", err)
	}

	return nil
}

// Migrate runs the real, embedded migrations against databaseURL. This
// is what cmd/server calls; RunMigrations above is what tests call, with
// open/up faked.
func Migrate(ctx context.Context, databaseURL string) error {
	return RunMigrations(ctx, databaseURL, sql.Open, gooseUp)
}

func gooseUp(ctx context.Context, db *sql.DB) error {
	if err := db.PingContext(ctx); err != nil {
		return ConnectionFailure(err)
	}

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("could not set migration dialect: %w", err)
	}
	return goose.UpContext(ctx, db, "migrations")
}
