package testutil

import "testing/fstest"

// FakeFS is an in-memory filesystem for tests, satisfying io/fs.FS (so
// production code written against fs.FS works unmodified against it).
// A path with no matching entry behaves as genuinely absent — Stat and
// ReadFile through it return an error satisfying errors.Is(err,
// fs.ErrNotExist) — without ever touching the real filesystem.
type FakeFS = fstest.MapFS

// NewFakeFS builds a FakeFS from plain file contents, keyed by path.
func NewFakeFS(files map[string]string) FakeFS {
	fake := make(FakeFS, len(files))
	for name, content := range files {
		fake[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return fake
}
