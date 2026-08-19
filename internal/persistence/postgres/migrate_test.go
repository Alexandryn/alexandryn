package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// FR-6: the short-lived *sql.DB opened for migrations is closed
// immediately after goose.Up returns, regardless of success or failure —
// never passed to or reused by any repository.
func TestRunMigrations_ClosesTheDBAfterward(t *testing.T) {
	var opened *sql.DB
	open := func(driverName, dsn string) (*sql.DB, error) {
		db, err := sql.Open(driverName, dsn)
		opened = db
		return db, err
	}
	up := func(context.Context, *sql.DB) error { return nil }

	if err := postgres.RunMigrations(context.Background(), "postgres://fixture/proof", open, up); err != nil {
		t.Fatalf("RunMigrations() error = %v, want nil", err)
	}

	if opened == nil {
		t.Fatal("open() was never called")
	}
	// database/sql's own closed-database sentinel is unexported; any
	// error here is sufficient proof Close() was actually called.
	if err := opened.PingContext(context.Background()); err == nil {
		t.Fatal("db.Ping() after RunMigrations succeeded, want an error (the db must be closed)")
	}
}

func TestRunMigrations_ClosesTheDBEvenOnFailure(t *testing.T) {
	var opened *sql.DB
	open := func(driverName, dsn string) (*sql.DB, error) {
		db, err := sql.Open(driverName, dsn)
		opened = db
		return db, err
	}
	up := func(context.Context, *sql.DB) error { return errors.New("migration failed") }

	if err := postgres.RunMigrations(context.Background(), "postgres://fixture/proof", open, up); err == nil {
		t.Fatal("RunMigrations() error = nil, want the up() error")
	}

	if err := opened.PingContext(context.Background()); err == nil {
		t.Fatal("db.Ping() after a failed RunMigrations succeeded, want an error (the db must still be closed)")
	}
}

func TestConnectionFailure_UnwrapsToTheUnderlyingError(t *testing.T) {
	underlying := errors.New("dial tcp: connection refused")
	err := postgres.ConnectionFailure(underlying)

	if err.Error() != underlying.Error() {
		t.Fatalf("Error() = %q, want %q", err.Error(), underlying.Error())
	}
	if !errors.Is(err, underlying) {
		t.Fatal("errors.Is(err, underlying) = false, want true")
	}
}

// FR-6 DSN-redaction regression guard.
const fakeDSNMarker = "postgres://fixture-marker:s3cr3t@host/db"

func TestRunMigrations_ConnectionFailureRedactsTheDSN(t *testing.T) {
	open := func(driverName, dsn string) (*sql.DB, error) { return sql.Open(driverName, dsn) }
	up := func(context.Context, *sql.DB) error {
		return postgres.ConnectionFailure(errors.New("dial tcp " + fakeDSNMarker + ": connection refused"))
	}

	err := postgres.RunMigrations(context.Background(), "postgres://fixture/proof", open, up)
	if err == nil {
		t.Fatal("RunMigrations() error = nil, want a connection-failure error")
	}
	if strings.Contains(err.Error(), fakeDSNMarker) {
		t.Fatalf("error leaked the DSN: %q", err.Error())
	}
}

func TestRunMigrations_GenuineSQLFailureRetainsDetail(t *testing.T) {
	open := func(driverName, dsn string) (*sql.DB, error) { return sql.Open(driverName, dsn) }
	up := func(context.Context, *sql.DB) error {
		return errors.New(`migration 00002_bad.sql: pq: syntax error at or near "CRATE"`)
	}

	err := postgres.RunMigrations(context.Background(), "postgres://fixture/proof", open, up)
	if err == nil {
		t.Fatal("RunMigrations() error = nil, want the SQL-failure error")
	}
	if !strings.Contains(err.Error(), "00002_bad.sql") || !strings.Contains(err.Error(), "syntax error") {
		t.Fatalf("error dropped real diagnostic detail for a genuine SQL failure: %q", err.Error())
	}
}

func TestRunMigrations_OpenFailureErrors(t *testing.T) {
	open := func(string, string) (*sql.DB, error) { return nil, errors.New("driver not registered") }
	up := func(context.Context, *sql.DB) error { return nil }

	if err := postgres.RunMigrations(context.Background(), "postgres://fixture/proof", open, up); err == nil {
		t.Fatal("RunMigrations() error = nil, want an error when open() itself fails")
	}
}
