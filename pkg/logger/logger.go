package logger

import (
	"log/slog"
	"os"
	"strings"

	"goshop/pkg/config"
)

func NewLogger(c config.Logger) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(c.Level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var h slog.Handler
	if c.JSON {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}

	l := slog.New(h)
	if c.AppName != "" {
		l = l.With(slog.String("app", c.AppName))
	}
	return l
}
