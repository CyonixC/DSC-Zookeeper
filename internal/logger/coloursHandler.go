package logger

// GPT'd implementation of a simple plaintext logger to console. Check README for usage.

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// ColouredTextHandler is the same as ColouredTextHandler, but colours the text based on severity.
type ColouredTextHandler struct {
	minLevel slog.Level
}

// NewColouredTextHandler creates a new handler with a specified minimum log level.
func NewColouredTextHandler(minLevel slog.Level) *ColouredTextHandler {
	return &ColouredTextHandler{minLevel: minLevel}
}

func colourise(text, colour string) string {
	return colour + text + reset
}

// Enabled checks if the log level meets the minimum level requirement.
func (h *ColouredTextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.minLevel
}

// Handle processes the log record, printing only the message in plain text format.
func (h *ColouredTextHandler) Handle(ctx context.Context, record slog.Record) error {
	// Only log if the record level is at or above the minLevel
	if record.Level < h.minLevel {
		return nil
	}

	// Colourise text
	var colour string
	switch record.Level {
	case slog.LevelDebug:
		colour = gray
	case slog.LevelInfo:
		colour = green
	case slog.LevelWarn:
		colour = yellow
	case slog.LevelError:
		colour = red
	default:
		colour = reset
	}

	msg := colourise(record.Message, colour)
	// Format and print the log message
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(os.Stdout, "%s %s: %s\n", timestamp, record.Level, msg)
	return nil
}

// WithAttrs and WithGroup are required to implement the slog.Handler interface but can be no-ops for simplicity.
func (h *ColouredTextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *ColouredTextHandler) WithGroup(name string) slog.Handler {
	return h
}

// ANSI escape sequences for colouring text
const (
	reset  = "\033[0m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	purple = "\033[35m"
	cyan   = "\033[36m"
	gray   = "\033[90m"
)
