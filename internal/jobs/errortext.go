package jobs

import (
	"log/slog"
	"unicode/utf8"
)

// maxLastErrorBytes bounds every last_error write (backend-job-queue.md
// FR-6, Security considerations). A handler's error text is attacker-
// influenced in the general case (it can wrap a source's response), so
// the column never grows without limit and never stores more than a
// bounded diagnostic snippet.
const maxLastErrorBytes = 512

// redactedPlaceholder is what RedactedText shows anywhere it would
// otherwise be logged or JSON-encoded.
const redactedPlaceholder = "[redacted]"

// RedactedText holds a last_error value. Go callers of GetJob/ListJobs
// read the real text via String() (FR-9 returns the full row shape to
// this codebase's own code). Every logging and JSON path is redacted:
// the Observability requirement forbids a status-transition log line
// from carrying last_error content, and this makes that a property of
// the type rather than a rule each call site must remember — the same
// dual-interface posture config.RedactedString uses
// (backend-errors-and-logging.md FR-8).
type RedactedText string

// String returns the real, unredacted text.
func (t RedactedText) String() string { return string(t) }

// LogValue satisfies slog.LogValuer.
func (RedactedText) LogValue() slog.Value { return slog.StringValue(redactedPlaceholder) }

// MarshalJSON satisfies json.Marshaler — slog's JSON handler falls back
// to encoding/json when a containing struct is logged as one attribute,
// and encoding/json does not know slog.LogValuer.
func (RedactedText) MarshalJSON() ([]byte, error) {
	return []byte(`"` + redactedPlaceholder + `"`), nil
}

// redactError is the single function every last_error write goes
// through. It truncates the message to maxLastErrorBytes on a rune
// boundary. A nil error yields "".
func redactError(err error) string {
	if err == nil {
		return ""
	}
	return truncateBytes(err.Error(), maxLastErrorBytes)
}

// truncateBytes returns the longest prefix of s that is at most max
// bytes and ends on a complete rune.
func truncateBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	trunc := s[:max]
	for len(trunc) > 0 && !utf8.RuneStart(s[len(trunc)]) {
		trunc = trunc[:len(trunc)-1]
	}
	return trunc
}
