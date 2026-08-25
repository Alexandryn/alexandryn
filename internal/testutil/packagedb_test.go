package testutil_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/testutil"
)

func TestDerivePackageDatabaseURL_SuffixesTheDatabaseNamePreservingEverythingElse(t *testing.T) {
	dbName, url, err := testutil.DerivePackageDatabaseURL("postgres://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable", "testutil")
	if err != nil {
		t.Fatalf("DerivePackageDatabaseURL: %v", err)
	}
	if dbName != "postgres_testutil" {
		t.Fatalf("dbName = %q, want %q", dbName, "postgres_testutil")
	}
	if url != "postgres://postgres:postgres@127.0.0.1:54322/postgres_testutil?sslmode=disable" {
		t.Fatalf("url = %q, want the same URL with the dbname suffixed", url)
	}
}

func TestDerivePackageDatabaseURL_DifferentPkgNamesProduceDifferentNames(t *testing.T) {
	const base = "postgres://postgres:postgres@127.0.0.1:54322/alexandryn_test"

	dbA, urlA, err := testutil.DerivePackageDatabaseURL(base, "postgres")
	if err != nil {
		t.Fatalf("DerivePackageDatabaseURL(postgres): %v", err)
	}
	dbB, urlB, err := testutil.DerivePackageDatabaseURL(base, "cmdserver")
	if err != nil {
		t.Fatalf("DerivePackageDatabaseURL(cmdserver): %v", err)
	}

	if dbA == dbB || urlA == urlB {
		t.Fatalf("two different pkgNames produced the same result: %q / %q", dbA, dbB)
	}
}

func TestDerivePackageDatabaseURL_InvalidURLFails(t *testing.T) {
	_, _, err := testutil.DerivePackageDatabaseURL("://not-a-valid-url", "testutil")
	if err == nil {
		t.Fatal("DerivePackageDatabaseURL: err = nil, want an error for an invalid URL")
	}
}

func TestDerivePackageDatabaseURL_EmptyPkgNameFails(t *testing.T) {
	_, _, err := testutil.DerivePackageDatabaseURL("postgres://postgres:postgres@127.0.0.1:54322/postgres", "")
	if err == nil {
		t.Fatal("DerivePackageDatabaseURL: err = nil, want an error for an empty pkgName")
	}
	if !strings.Contains(err.Error(), "pkgName") {
		t.Fatalf("error doesn't name the missing pkgName: %q", err.Error())
	}
}

func TestDerivePackageDatabaseURL_EmptyDatabaseNameInBaseFails(t *testing.T) {
	_, _, err := testutil.DerivePackageDatabaseURL("postgres://postgres:postgres@127.0.0.1:54322/", "testutil")
	if err == nil {
		t.Fatal("DerivePackageDatabaseURL: err = nil, want an error when the base URL names no database")
	}
}

// WithPackageDatabase's error path needs no real Postgres: an empty-path
// base URL fails inside DerivePackageDatabaseURL before EnsurePackageDatabase
// ever opens a connection, so the whole failure/no-run/no-setenv contract
// is provable with fakes alone.
func TestWithPackageDatabase_EnsureFailureSkipsRunAndSetenv(t *testing.T) {
	getenv := func(string) string { return "postgres://postgres:postgres@127.0.0.1:54322/" }
	setenvCalled := false
	setenv := func(string, string) error { setenvCalled = true; return nil }
	ran := false
	run := func() int { ran = true; return 0 }
	var out bytes.Buffer

	code := testutil.WithPackageDatabase("testutil", getenv, setenv, &out, run)()

	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if ran {
		t.Fatal("run() was called even though EnsurePackageDatabase failed")
	}
	if setenvCalled {
		t.Fatal("setenv() was called even though EnsurePackageDatabase failed")
	}
	if !strings.Contains(out.String(), "EnsurePackageDatabase") {
		t.Fatalf("message doesn't name the failing step: %q", out.String())
	}
}
