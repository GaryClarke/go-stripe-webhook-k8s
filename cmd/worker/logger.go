package main

import (
	"io"
	"log/slog"
)

func NewJSONLogger(w io.Writer, opts *slog.HandlerOptions) *slog.Logger {
	if opts == nil {
		opts = &slog.HandlerOptions{Level: slog.LevelInfo}
	}
	return slog.New(slog.NewJSONHandler(w, opts))
}
