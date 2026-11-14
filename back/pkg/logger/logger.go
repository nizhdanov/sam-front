package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

func New(level, format string) *slog.Logger {
	return newLogger(os.Stdout, level, format)
}

func newLogger(w io.Writer, level, format string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	if strings.EqualFold(format, "text") {
		return slog.New(slog.NewTextHandler(w, opts))
	}
	return slog.New(slog.NewJSONHandler(w, opts))
}
