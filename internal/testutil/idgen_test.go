package testutil_test

import (
	"testing"

	"github.com/Alexandryn/alexandryn/internal/testutil"
)

// FR-6 canary (backend-test-harness.md): a FakeIDGenerator constructed
// with a fixed sequence produces exactly that sequence across repeated
// calls, deterministic across repeated test runs.
func TestFakeIDGenerator_ProducesFixedSequence(t *testing.T) {
	want := []string{"id-1", "id-2", "id-3"}
	gen := testutil.NewFakeIDGenerator(want...)

	for i, w := range want {
		if got := gen.NewID(); got != w {
			t.Fatalf("call %d: NewID() = %q, want %q", i, got, w)
		}
	}
}

func TestFakeIDGenerator_ExhaustedSequencePanics(t *testing.T) {
	gen := testutil.NewFakeIDGenerator("only-id")
	gen.NewID()

	defer func() {
		if recover() == nil {
			t.Fatal("NewID() past the fixed sequence should panic, not silently repeat or return an empty string")
		}
	}()
	gen.NewID()
}
