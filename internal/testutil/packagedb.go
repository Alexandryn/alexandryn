package testutil

import (
	"fmt"
	"net/url"
	"strings"
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
