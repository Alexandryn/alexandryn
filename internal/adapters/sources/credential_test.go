package sources_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
)

const secretUser = "reader-jane"
const secretPass = "hunter2-XYZ"

func mustCredential(t *testing.T) sources.Credential {
	t.Helper()
	c, err := sources.NewCredential(secretUser, secretPass)
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	return c
}

func TestCredential_RevealReturnsRealValues(t *testing.T) {
	u, p := mustCredential(t).Reveal()
	if u != secretUser || p != secretPass {
		t.Fatalf("Reveal = %q/%q, want %q/%q", u, p, secretUser, secretPass)
	}
}

func TestCredential_NeverAppearsInLogs(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	c := mustCredential(t)

	// Single-attribute path.
	logger.Info("single", slog.Any("credential", c))
	// Whole-struct path — the case FR-8 exists to close.
	type wrapper struct {
		Label      string
		Credential sources.Credential
	}
	logger.Info("struct", slog.Any("source", wrapper{Label: "L", Credential: c}))

	out := buf.String()
	if strings.Contains(out, secretUser) || strings.Contains(out, secretPass) {
		t.Fatalf("credential leaked into logs: %s", out)
	}
	if !strings.Contains(out, "[redacted]") {
		t.Fatalf("expected [redacted] placeholder in logs: %s", out)
	}
}

func TestCredential_NeverAppearsInJSON(t *testing.T) {
	c := mustCredential(t)
	b, err := json.Marshal(struct {
		Cred sources.Credential `json:"cred"`
	}{c})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(b), secretUser) || strings.Contains(string(b), secretPass) {
		t.Fatalf("credential leaked into JSON: %s", b)
	}
}

func TestCredential_NeverAppearsInFmt(t *testing.T) {
	c := mustCredential(t)
	for _, s := range []string{
		fmt.Sprintf("%v", c),
		fmt.Sprintf("%s", c),
		fmt.Sprintf("%+v", c),
		fmt.Sprint(c),
	} {
		if strings.Contains(s, secretUser) || strings.Contains(s, secretPass) {
			t.Fatalf("credential leaked via fmt: %s", s)
		}
	}
}

func TestNewCredential_Rejections(t *testing.T) {
	cases := map[string][2]string{
		"empty username":    {"", "p"},
		"empty password":    {"u", ""},
		"whitespace user":   {"   ", "p"},
		"control char user": {"u\x00", "p"},
		"control char pass": {"u", "p\nnewline"},
		"overlong username": {strings.Repeat("a", 256), "p"},
		"overlong password": {"u", strings.Repeat("a", 1025)},
	}
	for name, in := range cases {
		if _, err := sources.NewCredential(in[0], in[1]); err == nil {
			t.Errorf("%s: err = nil, want rejection", name)
		}
	}
}

func TestCredential_ZeroValue(t *testing.T) {
	var c sources.Credential
	if !c.IsZero() {
		t.Fatal("zero Credential should be IsZero")
	}
	if mustCredential(t).IsZero() {
		t.Fatal("a real credential must not be IsZero")
	}
}
