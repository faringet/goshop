package metrics

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type GRPCMetrics struct {
	reqs *prometheus.CounterVec
	dur  *prometheus.HistogramVec
	infl prometheus.Gauge

	methodLabeler func(fullMethod string) (service string, method string)
}

type grpcConfig struct {
	namespace     string
	service       string
	buckets       []float64
	methodLabeler func(string) (string, string)
}

type GRPCOption func(*grpcConfig)

func WithGRPCBuckets(b []float64) GRPCOption {
	return func(c *grpcConfig) { c.buckets = b }
}

func WithMethodLabeler(fn func(string) (string, string)) GRPCOption {
	return func(c *grpcConfig) { c.methodLabeler = fn }
}

func NewGRPCMetrics(reg *prometheus.Registry, namespace, service string, opts ...GRPCOption) *GRPCMetrics {
	cfg := &grpcConfig{
		namespace:     namespace,
		service:       service,
		buckets:       prometheus.DefBuckets,
		methodLabeler: splitFullMethod,
	}
	for _, o := range opts {
		o(cfg)
	}

	m := &GRPCMetrics{
		reqs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: cfg.namespace,
			Subsystem: cfg.service,
			Name:      "grpc_requests_total",
			Help:      "Total number of gRPC requests.",
		}, []string{"grpc_service", "grpc_method", "code"}),
		dur: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: cfg.namespace,
			Subsystem: cfg.service,
			Name:      "grpc_handling_seconds",
			Help:      "gRPC handling duration in seconds.",
			Buckets:   cfg.buckets,
		}, []string{"grpc_service", "grpc_method", "code"}),
		infl: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: cfg.namespace,
			Subsystem: cfg.service,
			Name:      "grpc_in_flight_requests",
			Help:      "In-flight gRPC requests.",
		}),
		methodLabeler: cfg.methodLabeler,
	}

	reg.MustRegister(m.reqs, m.dur, m.infl)
	return m
}

func (m *GRPCMetrics) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		m.infl.Inc()
		start := time.Now()
		defer func() {
			m.infl.Dec()
			code := status.Code(err).String()
			svc, method := m.methodLabeler(info.FullMethod)
			m.reqs.WithLabelValues(svc, method, code).Inc()
			m.dur.WithLabelValues(svc, method, code).Observe(time.Since(start).Seconds())
		}()
		return handler(ctx, req)
	}
}

func (m *GRPCMetrics) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		m.infl.Inc()
		start := time.Now()
		defer func() {
			m.infl.Dec()
			code := status.Code(err).String()
			svc, method := m.methodLabeler(info.FullMethod)
			m.reqs.WithLabelValues(svc, method, code).Inc()
			m.dur.WithLabelValues(svc, method, code).Observe(time.Since(start).Seconds())
		}()
		return handler(srv, ss)
	}
}

func splitFullMethod(full string) (string, string) {
	if full == "" || full[0] != '/' {
		return "unknown", "unknown"
	}
	full = full[1:]
	i := strings.IndexByte(full, '/')
	if i <= 0 || i >= len(full)-1 {
		return "unknown", "unknown"
	}
	return full[:i], full[i+1:]
}
