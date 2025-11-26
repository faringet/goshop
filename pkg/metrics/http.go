package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	WebFastBuckets = []float64{
		0.005, 0.01, 0.025, 0.05, 0.1,
		0.25, 0.5, 1, 2.5, 5, 10,
	}
	SlowBuckets = []float64{
		0.01, 0.025, 0.05, 0.1, 0.25,
		0.5, 1, 2, 5, 10, 30, 60,
	}
)

type HTTPMetrics struct {
	reqs *prometheus.CounterVec
	dur  *prometheus.HistogramVec
	infl prometheus.Gauge

	pathLabeler func(*http.Request) string
}

type httpConfig struct {
	namespace   string
	service     string
	buckets     []float64
	pathLabeler func(*http.Request) string
}

type HTTPOption func(*httpConfig)

func WithBuckets(b []float64) HTTPOption {
	return func(c *httpConfig) { c.buckets = b }
}

func WithPathLabeler(fn func(*http.Request) string) HTTPOption {
	return func(c *httpConfig) { c.pathLabeler = fn }
}

func NewHTTPMetrics(reg *prometheus.Registry, namespace, service string, opts ...HTTPOption) *HTTPMetrics {
	cfg := &httpConfig{
		namespace:   namespace,
		service:     service,
		buckets:     prometheus.DefBuckets,
		pathLabeler: sanitizePathLabel,
	}
	for _, o := range opts {
		o(cfg)
	}

	h := &HTTPMetrics{
		reqs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: cfg.namespace,
			Subsystem: cfg.service,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests.",
		}, []string{"method", "path", "code"}),
		dur: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: cfg.namespace,
			Subsystem: cfg.service,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   cfg.buckets,
		}, []string{"method", "path", "code"}),
		infl: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: cfg.namespace,
			Subsystem: cfg.service,
			Name:      "http_in_flight_requests",
			Help:      "In-flight HTTP requests.",
		}),
		pathLabeler: cfg.pathLabeler,
	}

	reg.MustRegister(h.reqs, h.dur, h.infl)
	return h
}

func (h *HTTPMetrics) Middleware(next http.Handler) http.Handler {
	if next == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.infl.Inc()
		start := time.Now()

		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		code := strconv.Itoa(sw.status)
		method := r.Method
		path := h.pathLabeler(r)

		h.reqs.WithLabelValues(method, path, code).Inc()
		h.dur.WithLabelValues(method, path, code).Observe(time.Since(start).Seconds())
		h.infl.Dec()
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func sanitizePathLabel(r *http.Request) string {
	p := r.URL.Path
	if p == "" || p == "/" {
		return "/"
	}
	parts := strings.Split(p, "/")
	out := parts[:0]
	for _, s := range parts {
		if s == "" {
			continue
		}
		if isAllDigits(s) || looksLikeUUID(s) {
			out = append(out, ":id")
			continue
		}
		out = append(out, s)
	}
	return "/" + strings.Join(out, "/")
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func looksLikeUUID(s string) bool {
	// Простейшая проверка формата 8-4-4-4-12 (36 символов)
	if len(s) != 36 {
		return false
	}
	dash := []int{8, 13, 18, 23}
	for _, i := range dash {
		if s[i] != '-' {
			return false
		}
	}
	for _, r := range s {
		if r == '-' {
			continue
		}
		if !isHex(r) {
			return false
		}
	}
	return true
}

func isHex(r rune) bool {
	return (r >= '0' && r <= '9') ||
		(r >= 'a' && r <= 'f') ||
		(r >= 'A' && r <= 'F')
}
