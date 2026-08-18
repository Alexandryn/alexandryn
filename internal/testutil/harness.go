package testutil

import (
	"fmt"
	"io"
)

// IntegrationTestMain gates an integration-tagged package's TestMain. If
// TEST_DATABASE_URL is absent or empty, it writes a message naming the
// variable to out and returns a non-zero exit code without ever calling
// run — no test function in the package executes, which is what makes
// this a real fail-loud gate rather than a per-test skip that would still
// let other tests in the package run (backend-test-harness.md FR-2).
// Otherwise it calls run (typically m.Run) and returns its result
// unchanged.
//
// lookupEnv and run are injected so the gate is provable directly, without
// a real TEST_DATABASE_URL or a real subprocess.
//
//	func TestMain(m *testing.M) {
//		os.Exit(testutil.IntegrationTestMain(os.LookupEnv, m.Run, os.Stderr))
//	}
func IntegrationTestMain(lookupEnv func(key string) (string, bool), run func() int, out io.Writer) int {
	const key = "TEST_DATABASE_URL"

	val, ok := lookupEnv(key)
	if !ok || val == "" {
		_, _ = fmt.Fprintf(out, "%s is required to run integration tests (go test -tags=integration ./...); it is unset or empty.\n", key)
		return 1
	}
	return run()
}
