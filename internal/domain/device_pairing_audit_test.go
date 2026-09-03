package domain_test

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"
	"testing"
)

// domain-device-pairing.md FR-10 + Acceptance: no type in the pairing
// domain references net.IP, *http.Request, a raw User-Agent, or any
// hardware / re-identification value. Enforced as an import audit, a
// struct-field-type audit, and a source scan (keep comments here clean
// of those terms too, the same discipline the config bind tests use).
func TestDevicePairing_NoFingerprintingImportsOrFields(t *testing.T) {
	const file = "device_pairing.go"

	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}

	allowed := map[string]bool{
		`"crypto/subtle"`: true,
		`"errors"`:        true,
		`"fmt"`:           true,
		`"strings"`:       true,
		`"time"`:          true,
	}
	for _, imp := range f.Imports {
		if !allowed[imp.Path.Value] {
			t.Errorf("disallowed import %s in %s — the pairing domain has no I/O and stores no fingerprint", imp.Path.Value, file)
		}
	}

	lowered := strings.ToLower(string(src))
	for _, banned := range []string{"user-agent", "useragent", "net.ip", "http.request", "macaddr", "hardwareid", "screenresolution"} {
		if strings.Contains(lowered, banned) {
			t.Errorf("%s references %q — FR-10 forbids any device re-identification value (keep comments clear of it too)", file, banned)
		}
	}

	ast.Inspect(f, func(n ast.Node) bool {
		st, ok := n.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range st.Fields.List {
			var typ strings.Builder
			_ = printer.Fprint(&typ, fset, field.Type)
			switch typ.String() {
			case "net.IP", "*http.Request", "net.HardwareAddr":
				t.Errorf("struct field of type %s in %s violates FR-10", typ.String(), file)
			}
		}
		return true
	})
}
