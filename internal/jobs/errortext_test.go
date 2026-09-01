package jobs

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRedactError_Nil(t *testing.T) {
	if got := redactError(nil); got != "" {
		t.Fatalf("redactError(nil) = %q, want empty", got)
	}
}

func TestRedactError_ShortMessagePassesThrough(t *testing.T) {
	err := errors.New("handler failed: connection refused")
	if got := redactError(err); got != err.Error() {
		t.Fatalf("redactError = %q, want %q", got, err.Error())
	}
}

func TestRedactError_TruncatesToByteBound(t *testing.T) {
	long := strings.Repeat("x", 5000)
	got := redactError(errors.New(long))
	if len(got) > maxLastErrorBytes {
		t.Fatalf("len = %d, want <= %d", len(got), maxLastErrorBytes)
	}
	if len(got) != maxLastErrorBytes {
		t.Fatalf("len = %d, want exactly %d for an all-ASCII message", len(got), maxLastErrorBytes)
	}
}

func TestRedactError_TruncationKeepsRunesWhole(t *testing.T) {
	// 3-byte runes: 512 is not a multiple of 3, so a naive s[:512] would
	// split the rune straddling the boundary.
	msg := strings.Repeat("€", 400) // 1200 bytes
	got := redactError(errors.New(msg))
	if len(got) > maxLastErrorBytes {
		t.Fatalf("len = %d, want <= %d", len(got), maxLastErrorBytes)
	}
	if !json.Valid([]byte(`"` + got + `"`)) {
		t.Fatalf("truncated text is not valid UTF-8: %q", got)
	}
	for _, r := range got {
		if r == '�' {
			t.Fatal("truncation produced a replacement character — a rune was split")
		}
	}
}

func TestRedactedText_RedactsUnderLogAndJSON(t *testing.T) {
	secret := RedactedText("dial tcp 10.0.0.5:5432: password=hunter2")

	if secret.String() != "dial tcp 10.0.0.5:5432: password=hunter2" {
		t.Fatalf("String() should return the real value, got %q", secret.String())
	}
	if v := secret.LogValue().String(); v != redactedPlaceholder {
		t.Fatalf("LogValue() = %q, want %q", v, redactedPlaceholder)
	}
	b, err := json.Marshal(secret)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(b) != `"`+redactedPlaceholder+`"` {
		t.Fatalf("MarshalJSON = %s, want %q", b, redactedPlaceholder)
	}

	// A containing struct logged/encoded as one attribute must not leak.
	type wrap struct {
		LastError RedactedText
	}
	b, err = json.Marshal(wrap{LastError: secret})
	if err != nil {
		t.Fatalf("Marshal wrap: %v", err)
	}
	if strings.Contains(string(b), "hunter2") {
		t.Fatalf("containing struct leaked the raw value: %s", b)
	}
}
