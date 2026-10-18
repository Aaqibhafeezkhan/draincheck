package main

import (
	"io"
	"log/slog"
)

// newLogger returns the logger the whole run shares. With debug set it writes
// DEBUG lines in text form to w, which is stderr, so the report on stdout stays
// clean. Without it, the logger discards everything.
func newLogger(debug bool, w io.Writer) *slog.Logger {
	if !debug {
		return slog.New(slog.DiscardHandler)
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug}))
}
