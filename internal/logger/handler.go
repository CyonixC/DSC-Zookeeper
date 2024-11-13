package logger

// GPT'd implementation of a simple plaintext logger to console. Check README for usage.

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// PlainTextHandler is a custom handler that outputs only the message value in plain text.
type PlainTextHandler struct {
	minLevel slog.Level
}

// NewPlainTextHandler creates a new handler with a specified minimum log level.
func NewPlainTextHandler(minLevel slog.Level) *PlainTextHandler {
	return &PlainTextHandler{minLevel: minLevel}
}

// Enabled checks if the log level meets the minimum level requirement.
func (h *PlainTextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.minLevel
}

// Handle processes the log record, printing only the message in plain text format.
func (h *PlainTextHandler) Handle(ctx context.Context, record slog.Record) error {
	// Only log if the record level is at or above the minLevel
	if record.Level < h.minLevel {
		return nil
	}

	// Format and print the log message
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(os.Stdout, "%s %s: %s\n", timestamp, record.Level, record.Message)
	return nil
}

// WithAttrs and WithGroup are required to implement the slog.Handler interface but can be no-ops for simplicity.
func (h *PlainTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *PlainTextHandler) WithGroup(name string) slog.Handler {
	return h
}
