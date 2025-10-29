package logger

import (
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"goshop/pkg/config"
)

func NewLogger(c config.Logger) *slog.Logger {
	lvl := parseLevel(strings.ToLower(c.Level))

	opts := handlerOptions(lvl)

	var h slog.Handler
	if c.JSON {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}

	host, _ := os.Hostname()
	env := firstNonEmpty(os.Getenv("APP_ENV"), os.Getenv("ENV"), "dev")

	l := slog.New(h).With(
		slog.String("app", c.AppName),
		slog.String("env", env),
		slog.String("host", host),
		slog.Int("pid", os.Getpid()),
		slog.String("goarch", runtime.GOARCH),
		slog.String("goos", runtime.GOOS),
	)

	return l
}

func handlerOptions(lvl slog.Level) *slog.HandlerOptions {
	return &slog.HandlerOptions{
		Level:       lvl,
		AddSource:   lvl <= slog.LevelDebug,
		ReplaceAttr: normalizeCoreAttrs,
	}
}

func normalizeCoreAttrs(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey {
		if t, ok := a.Value.Any().(time.Time); ok {
			a.Value = slog.StringValue(t.UTC().Format(time.RFC3339Nano))
		}
		return a
	}
	if a.Key == slog.LevelKey {
		if lv, ok := a.Value.Any().(slog.Level); ok {
			a.Value = slog.StringValue(strings.ToLower(lv.String()))
		}
		return a
	}
	return a
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
