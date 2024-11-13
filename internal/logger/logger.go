package logger

import (
	"log/slog"
	"os"
)

var logger slog.Logger

func Debug(msg string, args ...any) {
	if logger.Handler() == nil {
		logger = *slog.Default()
	}
	logger.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	if logger.Handler() == nil {
		logger = *slog.Default()
	}
	logger.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	if logger.Handler() == nil {
		logger = *slog.Default()
	}
	logger.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	if logger.Handler() == nil {
		logger = *slog.Default()
	}
	logger.Error(msg, args...)
}

func Fatal(msg string, args ...any) {
	if logger.Handler() == nil {
		logger = *slog.Default()
	}
	logger.Error(msg, args...)
	os.Exit(1)
}

func InitLogger(lg *slog.Logger) {
	logger = *lg
}
