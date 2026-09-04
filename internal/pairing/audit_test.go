package pairing_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// backend-network-transport.md FR-9: the pairing-code generator returns a
// crypto/rand failure, it never falls back to a weaker source. Enforced
// as a source audit — no file in internal/pairing imports math/rand.
func TestPairing_NoMathRand(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(".", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(src), `"math/rand"`) {
			t.Errorf("%s imports math/rand — pairing codes must come only from crypto/rand", e.Name())
		}
	}
}
