// Package logging constructs Alexandryn's structured logger:
// log/slog with JSON formatting, built once at startup and injected into dependencies.
package logging

import (
	"io"
	"log/slog"
	"strings"
)

// New constructs a JSON-handler *slog.Logger writing to w at the specified level
// ("debug", "info", "warn", "error", matched case-insensitively). An unrecognized
// or empty level string defaults safely to info.
func New(level string, w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: parseLevel(level)}))
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
