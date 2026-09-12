package testutil

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
)

// packageDatabaseTimeout bounds EnsurePackageDatabase's own connect/CREATE
// DATABASE work. It matters specifically because WithPackageDatabase runs
// it before m.Run() — go test's own -timeout watchdog is armed inside
// testing.M.Run() (internal/testutil/harness.go's IntegrationTestMain just
// calls the injected run closure), so without an explicit bound here a
// stalled connection or catalog lock would hang the test binary with no
// timeout at all, worse than every other DB access in the package, which
// -timeout already covers.
const packageDatabaseTimeout = 30 * time.Second

// DerivePackageDatabaseURL rewrites base's database name to
// "<original>_<pkgName>", leaving every other URL component (user, host,
// port, query parameters) unchanged. It is the pure half of
// the per-package isolation mechanism — no I/O, so it's unit-tested
// directly without a real Postgres. EnsurePackageDatabase is the real-I/O
// half that actually creates the database this URL names.
func DerivePackageDatabaseURL(base, pkgName string) (dbName, derivedURL string, err error) {
	if pkgName == "" {
		return "", "", fmt.Errorf("DerivePackageDatabaseURL: pkgName must not be empty")
	}

	parsed, err := url.Parse(base)
	if err != nil {
		return "", "", fmt.Errorf("DerivePackageDatabaseURL: parsing base URL: %w", err)
	}

	original := strings.TrimPrefix(parsed.Path, "/")
	if original == "" {
		return "", "", fmt.Errorf("DerivePackageDatabaseURL: base URL names no database (empty path)")
	}

	dbName = original + "_" + pkgName
	parsed.Path = "/" + dbName
	return dbName, parsed.String(), nil
}

// EnsurePackageDatabase creates the per-package database
// DerivePackageDatabaseURL(baseDatabaseURL, pkgName) names, if it doesn't
// already exist, and returns the URL pointing at it. This is the real-I/O
// half of the isolation mechanism: a TestMain calls it once,
// before m.Run(), then sets the TEST_DATABASE_URL environment variable to
// the returned URL so every existing call site in the package that reads
// it — Migrate, testDB, resetSchema — transparently targets the isolated
// database, with no call site changes needed.
//
// Postgres has no CREATE DATABASE IF NOT EXISTS, so a second call with
// the same pkgName is made idempotent by catching SQLSTATE 42P04
// (duplicate_database) specifically, the same errors.As(&pgconn.PgError{})
// pattern internal/persistence/postgres/errors.go's TranslateError
// already uses. A connection-open failure gets a fixed, generic message —
// never the raw error text, which can embed the DSN (same rule the
// postgres-access-and-migrations skill states for migration-connection
// failures); a genuine SQL/DDL failure's error text carries no such risk
// and is returned as-is.
func EnsurePackageDatabase(ctx context.Context, baseDatabaseURL, pkgName string) (string, error) {
	dbName, derivedURL, err := DerivePackageDatabaseURL(baseDatabaseURL, pkgName)
	if err != nil {
		return "", fmt.Errorf("EnsurePackageDatabase: %w", err)
	}

	admin, err := sql.Open("pgx", baseDatabaseURL)
	if err != nil {
		return "", errors.New("EnsurePackageDatabase: connecting to TEST_DATABASE_URL failed")
	}
	defer func() { _ = admin.Close() }()

	ident := pgx.Identifier{dbName}.Sanitize()
	if _, err := admin.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", ident)); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "42P04" { // duplicate_database
				return derivedURL, nil
			}
			return "", fmt.Errorf("EnsurePackageDatabase: creating database %q: %w", dbName, err)
		}
		// Not a *pgconn.PgError: the admin connection itself failed or
		// dropped mid-statement (a context deadline, a network reset),
		// not a genuine server-side SQL/DDL error — same distinction
		// TranslateError draws. Named separately from the sql.Open
		// failure above so the two aren't reported as the identical
		// text; still generic, never the raw error, which can embed
		// the DSN.
		return "", fmt.Errorf("EnsurePackageDatabase: creating database %q: the admin connection failed", dbName)
	}

	return derivedURL, nil
}

// WithPackageDatabase wraps run so it first gives the calling package its
// own isolated database (EnsurePackageDatabase), rewriting
// TEST_DATABASE_URL via setenv before run executes. This is the one shape
// all three integration-tagged TestMains need
// (internal/testutil, internal/persistence/postgres, cmd/server) — factored
// out here instead of copy-pasted three times, and bounded by
// packageDatabaseTimeout since it runs before go test's own -timeout
// watchdog is armed. getenv/setenv are injected (matching
// IntegrationTestMain's own lookupEnv) so this file never calls os.Getenv/
// os.Setenv directly — architectural boundaries restrict direct env access to
// internal/config and _integration_test.go files, neither of which this
// file is; os.Getenv and os.Setenv satisfy these signatures directly at
// each call site.
func WithPackageDatabase(pkgName string, getenv func(string) string, setenv func(string, string) error, out io.Writer, run func() int) func() int {
	return func() int {
		ctx, cancel := context.WithTimeout(context.Background(), packageDatabaseTimeout)
		defer cancel()

		isolatedURL, err := EnsurePackageDatabase(ctx, getenv("TEST_DATABASE_URL"), pkgName)
		if err != nil {
			_, _ = fmt.Fprintf(out, "harness setup: EnsurePackageDatabase: %v\n", err)
			return 1
		}
		if err := setenv("TEST_DATABASE_URL", isolatedURL); err != nil {
			_, _ = fmt.Fprintf(out, "harness setup: setting TEST_DATABASE_URL: %v\n", err)
			return 1
		}
		return run()
	}
}
