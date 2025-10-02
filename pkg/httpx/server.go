package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	pcfg "goshop/pkg/config"
)

type Option func(*gin.Engine)

type Module interface {
	Mount(r *gin.Engine) error
	Name() string
}

func WithModules(mods ...Module) Option {
	return func(r *gin.Engine) {
		for _, m := range mods {
			if m == nil {
				continue
			}
			if err := m.Mount(r); err != nil {
				panic(err)
			}
		}
	}
}

type Server struct {
	http *http.Server
	log  *slog.Logger
}

func NewServer(cfg pcfg.HTTP, log *slog.Logger, opts ...Option) *Server {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(RequestID())
	r.Use(GinLogger(log))
	r.Use(GinRecovery(log))

	for _, opt := range opts {
		opt(r)
	}

	s := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return &Server{http: s, log: log}
}

func (s *Server) ListenAndServe() error {
	s.log.Info("http: listening", slog.String("addr", s.http.Addr))
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
