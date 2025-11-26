package metrics

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Config struct {
	Service   string
	Namespace string
	Addr      string
	Version   string
	Env       string
}

type Server struct {
	reg *prometheus.Registry
	srv *http.Server

	ServiceUp prometheus.Gauge
	BuildInfo *prometheus.GaugeVec
}

func Init(log *slog.Logger, cfg Config) (*Server, error) {
	if log == nil {
		log = slog.Default()
	}
	if cfg.Namespace == "" {
		cfg.Namespace = "goshop"
	}
	if cfg.Addr == "" {
		cfg.Addr = ":2112"
	}
	if cfg.Version == "" {
		cfg.Version = "dev"
	}
	if cfg.Env == "" {
		cfg.Env = "dev"
	}

	reg := prometheus.NewRegistry()

	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	serviceUp := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: cfg.Namespace,
		Subsystem: cfg.Service,
		Name:      "up",
		Help:      "1 if the service is up",
	})
	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: cfg.Namespace,
		Subsystem: cfg.Service,
		Name:      "build_info",
		Help:      "build information (labels: version, env), value is always 1",
	}, []string{"version", "env"})

	reg.MustRegister(serviceUp, buildInfo)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return nil, err
	}
	go func() {
		log.Info("metrics: listening", "addr", cfg.Addr, "service", cfg.Service)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Error("metrics: serve failed", "err", err)
		}
	}()

	buildInfo.WithLabelValues(cfg.Version, cfg.Env).Set(1)
	serviceUp.Set(1)

	return &Server{
		reg:       reg,
		srv:       srv,
		ServiceUp: serviceUp,
		BuildInfo: buildInfo,
	}, nil
}

func (s *Server) Registry() *prometheus.Registry { return s.reg }

func (s *Server) Shutdown(ctx context.Context) error {
	s.ServiceUp.Set(0)
	return s.srv.Shutdown(ctx)
}
