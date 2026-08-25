package testutil

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
)

// DerivePackageDatabaseURL rewrites base's database name to
// "<original>_<pkgName>", leaving every other URL component (user, host,
// port, query parameters) unchanged. It is the pure half of
// backend-test-harness.md FR-3 Variant B's per-package isolation
// mechanism — no I/O, so it's unit-tested directly without a real
// Postgres. EnsurePackageDatabase is the real-I/O half that actually
// creates the database this URL names.
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
// half of FR-3 Variant B's isolation mechanism: a TestMain calls it once,
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
		return "", errors.New("EnsurePackageDatabase: connecting to TEST_DATABASE_URL failed")
	}

	return derivedURL, nil
}
