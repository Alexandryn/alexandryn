package testutil_test

import (
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/testutil"
)

// FR-6 canary (backend-test-harness.md): a FakeFS constructed with no
// files present observes a not-exists result for Stat without touching
// the real filesystem at all. Proven by using a path containing a NUL
// byte — invalid for a real Stat call (which fails with a distinct,
// non-"not exist" error), but a perfectly ordinary fstest.MapFS key. If
// the fake ever fell through to the real filesystem, this assertion
// would fail with the wrong error rather than pass by accident.
func TestFakeFS_StatOnAbsentPathNeverTouchesRealDisk(t *testing.T) {
	real, realErr := os.Stat("bad\x00path")
	if realErr == nil {
		t.Fatalf("real os.Stat unexpectedly succeeded on %q: %v", "bad\x00path", real)
	}
	if errors.Is(realErr, fs.ErrNotExist) {
		t.Fatalf("test assumption broken: the real filesystem already treats this path as merely absent (%v); pick a path the real OS rejects some other way", realErr)
	}

	fake := testutil.FakeFS{}
	_, fakeErr := fs.Stat(fake, "bad\x00path")
	if !errors.Is(fakeErr, fs.ErrNotExist) {
		t.Fatalf("fake FS Stat error = %v, want fs.ErrNotExist", fakeErr)
	}
}

func TestFakeFS_ReadFilePresent(t *testing.T) {
	fake := testutil.NewFakeFS(map[string]string{
		"config.toml": `bind_address = "127.0.0.1"`,
	})

	data, err := fs.ReadFile(fake, "config.toml")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got := string(data); got != `bind_address = "127.0.0.1"` {
		t.Fatalf("ReadFile content = %q, want the fixture content", got)
	}
}
