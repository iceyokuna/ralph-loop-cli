// Package logger builds the slog.Logger ralph uses for run progress, following
// the structured-logging convention used across the codebase.
package logger

import (
	"io"
	"log/slog"
)

// New returns a text slog.Logger writing to w at info level.
func New(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
