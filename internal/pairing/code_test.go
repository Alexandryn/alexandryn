package pairing_test

import (
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/pairing"
)

func TestGeneratePairingCode(t *testing.T) {
	const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

	seen := make(map[string]struct{})
	for i := 0; i < 500; i++ {
		c, err := pairing.GeneratePairingCode()
		if err != nil {
			t.Fatalf("GeneratePairingCode: %v", err)
		}
		n := c.Normalized()
		if len(n) != 8 {
			t.Fatalf("normalized code %q is %d chars, want 8", n, len(n))
		}
		for _, r := range n {
			if !strings.ContainsRune(crockford, r) {
				t.Fatalf("code %q has a non-Crockford char %q", n, r)
			}
		}
		if c.Display() != n[:4]+"-"+n[4:] {
			t.Fatalf("Display() = %q", c.Display())
		}
		seen[n] = struct{}{}
	}
	// 40 bits of entropy over 500 draws — a collision here would be a
	// ~2e-9 fluke or a broken generator.
	if len(seen) != 500 {
		t.Fatalf("only %d distinct codes in 500 draws — entropy is broken", len(seen))
	}
}

// The generator must not import math/rand — a crypto/rand failure is
// returned, never downgraded. Proven structurally in
// internal/pairing/audit_test.go.
