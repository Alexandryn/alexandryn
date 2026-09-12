package testutil_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Verifies fail-loud gate behavior when TEST_DATABASE_URL is missing:
// `go test -tags=integration ./...` is invoked as a real subprocess against
// a fixture package that uses testutil.IntegrationTestMain exactly as a
// real integration package will, with TEST_DATABASE_URL unset and,
// separately, set to an empty string. Both must fail the run and name
// the missing variable — not silently skip. To distinguish a real
// fail-fast from a per-test skip that would still report "0 failed, N
// skipped" (a far weaker claim), the fixture's own test writes a
// sentinel file as literally its first action; the outer test asserts
// that file does not exist afterward, proving the fixture's body was
// never entered at all. A third run, with the variable set, proves the
// gate isn't simply always-blocking: the sentinel file is asserted
// present.
//
// The fixture lives inside this module (underscore-prefixed, so `go
// build ./...`/`go vet ./...` at the repo root ignore it while it
// exists) rather than as a standalone module, because it imports
// internal/testutil directly — Go's own internal/ visibility rule
// refuses that import from outside this module's tree even with a
// replace directive. The fixture is created fresh and removed via
// t.Cleanup; it is never committed.
func TestFailLoudNotSkip_MissingTestDatabaseURL(t *testing.T) {
	dir := filepath.Join(moduleRoot(t), fmt.Sprintf("_fixture_failloud_%d", os.Getpid()))
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("RemoveAll(%q) before fixture setup: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("cleanup: RemoveAll(%q): %v", dir, err)
		}
	})

	writeFile(t, dir, "pkg.go", "package fixture\n")
	writeFile(t, dir, "main_integration_test.go", `//go:build integration

package fixture

import (
	"os"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/testutil"
)

func TestMain(m *testing.M) {
	os.Exit(testutil.IntegrationTestMain(os.LookupEnv, m.Run, os.Stderr))
}

func TestWritesSentinel(t *testing.T) {
	path := os.Getenv("SENTINEL_PATH")
	if path == "" {
		t.Fatal("SENTINEL_PATH not set")
	}
	if err := os.WriteFile(path, []byte("entered"), 0o600); err != nil {
		t.Fatal(err)
	}
}
`)

	baseEnv := filteredEnv("TEST_DATABASE_URL")

	t.Run("unset", func(t *testing.T) {
		sentinel := filepath.Join(t.TempDir(), "sentinel")
		env := append(append([]string{}, baseEnv...), "SENTINEL_PATH="+sentinel)

		out, err := runGo(t, dir, env, "test", "-tags=integration", "./...")
		if err == nil {
			t.Fatalf("go test -tags=integration ./... succeeded with TEST_DATABASE_URL unset, want it to fail loudly\n%s", out)
		}
		if !strings.Contains(string(out), "TEST_DATABASE_URL") {
			t.Fatalf("failure output doesn't name TEST_DATABASE_URL:\n%s", out)
		}
		if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
			t.Fatalf("sentinel file exists — the fixture test's body ran despite the missing variable (stat err: %v)", err)
		}
	})

	t.Run("empty", func(t *testing.T) {
		sentinel := filepath.Join(t.TempDir(), "sentinel")
		env := append(append([]string{}, baseEnv...), "TEST_DATABASE_URL=", "SENTINEL_PATH="+sentinel)

		out, err := runGo(t, dir, env, "test", "-tags=integration", "./...")
		if err == nil {
			t.Fatalf("go test -tags=integration ./... succeeded with TEST_DATABASE_URL=\"\", want it to fail the same way as unset\n%s", out)
		}
		if !strings.Contains(string(out), "TEST_DATABASE_URL") {
			t.Fatalf("failure output doesn't name TEST_DATABASE_URL:\n%s", out)
		}
		if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
			t.Fatalf("sentinel file exists — the fixture test's body ran despite the empty variable (stat err: %v)", err)
		}
	})

	t.Run("set", func(t *testing.T) {
		sentinel := filepath.Join(t.TempDir(), "sentinel")
		env := append(append([]string{}, baseEnv...), "TEST_DATABASE_URL=postgres://fixture/proof", "SENTINEL_PATH="+sentinel)

		out, err := runGo(t, dir, env, "test", "-tags=integration", "./...")
		if err != nil {
			t.Fatalf("go test -tags=integration ./... failed with TEST_DATABASE_URL set, want it to run\n%s", out)
		}
		if _, err := os.Stat(sentinel); err != nil {
			t.Fatalf("sentinel file missing — the fixture test's body never ran even though the variable was set: %v", err)
		}
	})
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod walking up from %s", file)
		}
		dir = parent
	}
}

// filteredEnv returns the current process environment with key removed,
// so a subprocess test can prove behavior for key genuinely unset rather
// than merely overridden to empty by a parent process that happens not
// to set it.
func filteredEnv(key string) []string {
	prefix := key + "="
	var out []string
	for _, kv := range os.Environ() {
		if len(kv) >= len(prefix) && kv[:len(prefix)] == prefix {
			continue
		}
		out = append(out, kv)
	}
	return out
}
