package testutil_test

import (
	"testing"

	"github.com/Alexandryn/alexandryn/internal/testutil"
)

// FakeIDGenerator produces a deterministic sequence across repeated calls
// when constructed with a fixed sequence.
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
