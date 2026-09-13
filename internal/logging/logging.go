// Package logging provides a thin wrapper around log/slog for structured,
// leveled logging. Writers can be swapped (stderr for the CLI, io.Discard or a
// file inside the TUI) without changing call sites.
package logging

import (
	"io"
	"log/slog"
)

// New returns a structured logger writing to w.
func New(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}

// Discard returns a logger that drops every record, used inside the TUI so
// background workers never corrupt the alternate screen.
func Discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
