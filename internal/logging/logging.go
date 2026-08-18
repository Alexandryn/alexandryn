// Package logging constructs Alexandryn's one structured logger
// (backend-errors-and-logging.md FR-6): log/slog, JSON, built once at
// startup and passed by injection — never a package-level default.
package logging

import (
	"io"
	"log/slog"
	"strings"
)

// New constructs a JSON-handler *slog.Logger writing to w, at the level
// named by level ("debug"/"info"/"warn"/"error", matched case-
// insensitively, matching backend-configuration.md FR-4's own LOG_LEVEL
// validation). An unrecognized or empty level string defaults to info,
// the same default backend-configuration.md's own compiled default uses
// — New never panics on a bad level string, it degrades to the safe
// default (FR-9: the default level must not emit debug lines).
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
