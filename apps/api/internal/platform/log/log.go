// Package log configures structured logging (slog, JSON).
package log

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a slog.Logger with a JSON handler and the level from the environment.
func New(level, service string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(h).With(slog.String("service", service))
}
