package config

import "log/slog"

// redactedPlaceholder is what a RedactedString shows in place of its real
// value, everywhere: logs, JSON, and default Go formatting.
const redactedPlaceholder = "[redacted]"

// RedactedString holds a sensitive value that must not appear in log lines or
// error strings in plain text (e.g. credentials or connection strings).
// It implements slog.LogValuer, json.Marshaler, and fmt.Stringer, each returning
// the fixed placeholder "[redacted]". This prevents inadvertent leakage across
// formatting, structured logging, and JSON encoding paths.
//
// Reveal returns the underlying unredacted value. Call sites must explicitly
// call Reveal() when passing the value to secure consumers (e.g. database drivers).
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
