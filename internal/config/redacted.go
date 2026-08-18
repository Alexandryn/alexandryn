package config

import "log/slog"

// redactedPlaceholder is what a RedactedString shows in place of its real
// value, everywhere: logs, JSON, and default Go formatting.
const redactedPlaceholder = "[redacted]"

// RedactedString holds a value that must never appear in a log line or
// error string in plain text — a connection string or credential
// (backend-configuration.md FR-7). It implements slog.LogValuer,
// json.Marshaler, and fmt.Stringer, each returning the same fixed
// placeholder, so no logging or formatting call site can leak the real
// value by accident — including the one FR-7 exists specifically to
// close: log/slog's JSON handler falls back to encoding/json's
// reflection-based marshaling when a *containing* struct is logged as a
// single attribute, and encoding/json has no knowledge of
// slog.LogValuer, only json.Marshaler. Implementing only one interface
// leaves that path open; this implements both.
//
// Reveal returns the real value. It is the one method that does,
// deliberately named so every call site is a conscious choice, not an
// accidental one — used only where the real value is actually needed
// (a Postgres connection, not a log line).
type RedactedString string

// String satisfies fmt.Stringer, so even an incidental %v/%s format
// verb shows the placeholder instead of the real value.
func (r RedactedString) String() string { return redactedPlaceholder }

// LogValue satisfies slog.LogValuer.
func (r RedactedString) LogValue() slog.Value { return slog.StringValue(redactedPlaceholder) }

// MarshalJSON satisfies json.Marshaler.
func (r RedactedString) MarshalJSON() ([]byte, error) {
	return []byte(`"` + redactedPlaceholder + `"`), nil
}

// Reveal returns the real, unredacted value.
func (r RedactedString) Reveal() string { return string(r) }
