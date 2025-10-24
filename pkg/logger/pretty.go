package logger

//func NewPrettyLogger(c pcfg.Logger) *slog.Logger {
//	h := &prettyHandler{
//		w:     os.Stdout,
//		level: parseLevel(strings.ToLower(c.Level)),
//		app:   c.AppName,
//		color: true,
//	}
//	return slog.New(h)
//}
//
//type prettyHandler struct {
//	w      io.Writer
//	level  slog.Level
//	app    string
//	color  bool
//	attrs  []slog.Attr
//	groups []string
//}
//
//func (h *prettyHandler) Enabled(_ context.Context, lvl slog.Level) bool {
//	return lvl >= h.level
//}
//
//func (h *prettyHandler) Handle(_ context.Context, r slog.Record) error {
//	ts := r.Time
//	if ts.IsZero() {
//		ts = time.Now()
//	}
//	lvlTxt, lvlClr := levelText(r.Level), levelColor(r.Level, h.color)
//
//	var b strings.Builder
//	fmt.Fprintf(&b, "%s %s", ts.Format(time.RFC3339Nano), lvlClr(lvlTxt))
//
//	if h.app != "" {
//		b.WriteString(" app=")
//		b.WriteString(h.app)
//	}
//
//	if r.Message != "" {
//		fmt.Fprintf(&b, " msg=%q", r.Message)
//	}
//
//	for _, a := range h.attrs {
//		writeAttr(&b, h.groups, a)
//	}
//	r.Attrs(func(a slog.Attr) bool {
//		writeAttr(&b, h.groups, a)
//		return true
//	})
//
//	b.WriteByte('\n')
//	_, err := io.WriteString(h.w, b.String())
//	return err
//}
//
//func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
//	cp := *h
//	cp.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
//	return &cp
//}
//
//func (h *prettyHandler) WithGroup(name string) slog.Handler {
//	if name == "" {
//		return h
//	}
//	cp := *h
//	cp.groups = append(append([]string{}, h.groups...), name)
//	return &cp
//}
//
//func parseLevel(s string) slog.Level {
//	switch s {
//	case "debug":
//		return slog.LevelDebug
//	case "warn", "warning":
//		return slog.LevelWarn
//	case "error":
//		return slog.LevelError
//	default:
//		return slog.LevelInfo
//	}
//}
//
//func levelText(l slog.Level) string {
//	switch l {
//	case slog.LevelDebug:
//		return "[DEBUG]"
//	case slog.LevelWarn:
//		return "[WARN]"
//	case slog.LevelError:
//		return "[ERROR]"
//	default:
//		return "[INFO]"
//	}
//}
//
//func levelColor(l slog.Level, enabled bool) func(string) string {
//	if !enabled {
//		return func(s string) string { return s }
//	}
//	const (
//		grey  = "\x1b[90m"
//		red   = "\x1b[31m"
//		yel   = "\x1b[33m"
//		blue  = "\x1b[34m"
//		reset = "\x1b[0m"
//	)
//	switch l {
//	case slog.LevelDebug:
//		return func(s string) string { return grey + s + reset }
//	case slog.LevelWarn:
//		return func(s string) string { return yel + s + reset }
//	case slog.LevelError:
//		return func(s string) string { return red + s + reset }
//	default:
//		return func(s string) string { return blue + s + reset }
//	}
//}
//
//func writeAttr(b *strings.Builder, groups []string, a slog.Attr) {
//	a.Value = a.Value.Resolve()
//	key := a.Key
//	if len(groups) > 0 {
//		key = strings.Join(groups, ".") + "." + key
//	}
//	switch v := a.Value.Any().(type) {
//	case string:
//		fmt.Fprintf(b, " %s=%q", key, v)
//	default:
//		fmt.Fprintf(b, " %s=%v", key, a.Value)
//	}
//}
