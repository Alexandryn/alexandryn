package testutil_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// FR-1 structural proof (backend-test-harness.md): a file suffixed
// _integration_test.go, tagged //go:build integration, containing an
// intentionally-broken reference, must not fail `go build ./...` or
// `go vet ./...` when the integration tag isn't passed (negative
// control) — proving the file is genuinely excluded, not merely broken
// in a way nothing happens to touch — and must fail `go vet -tags=
// integration ./...` when the tag is passed (positive control) —
// proving the file's contents are genuinely parsed when the tag is
// supplied, not that the toolchain ignores the directory regardless.
//
// This runs `go` as a real subprocess against a standalone fixture
// module (never the real tree), since the thing under proof is the Go
// toolchain's own build-tag behavior, not application logic.
func TestBuildTagExcludesIntegrationFileByDefault(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "go.mod", "module fixture\n\ngo 1.26\n")
	writeFile(t, dir, "pkg.go", "package fixture\n\nfunc OK() int { return 1 }\n")
	writeFile(t, dir, "broken_integration_test.go", `//go:build integration

package fixture

import "testing"

func TestBroken(t *testing.T) {
	thisSymbolDoesNotExistAnywhere()
}
`)

	// Negative control: no tag, the broken file must be invisible to the
	// toolchain entirely.
	if out, err := runGo(t, dir, nil, "build", "./..."); err != nil {
		t.Fatalf("go build ./... (no tag) failed, want the broken file excluded: %v\n%s", err, out)
	}
	if out, err := runGo(t, dir, nil, "vet", "./..."); err != nil {
		t.Fatalf("go vet ./... (no tag) failed, want the broken file excluded: %v\n%s", err, out)
	}

	// Positive control: with the tag, the broken reference must actually
	// be caught — proving the file was genuinely parsed, not skipped for
	// an unrelated reason.
	out, err := runGo(t, dir, nil, "vet", "-tags=integration", "./...")
	if err == nil {
		t.Fatalf("go vet -tags=integration ./... succeeded, want it to catch the broken reference\n%s", out)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func runGo(t *testing.T, dir string, env []string, args ...string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	return cmd.CombinedOutput()
}
