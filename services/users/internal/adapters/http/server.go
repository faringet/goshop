package httpserver

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	pcfg "goshop/pkg/config"
	"goshop/pkg/jwtauth"
	"goshop/services/users/internal/adapters/http/handlers"
	"goshop/services/users/internal/adapters/repo/sessionpg"
	"goshop/services/users/internal/app"
	"log/slog"
	"net/http"
	"time"
)

type Builder struct {
	cfg pcfg.HTTP
	log *slog.Logger
	db  *pgxpool.Pool
	r   *gin.Engine
}

func NewBuilder(cfg pcfg.HTTP, log *slog.Logger) *Builder {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(requestID())
	r.Use(ginLogger(log))
	r.Use(ginRecovery(log))
	return &Builder{cfg: cfg, log: log, r: r}
}

type Server struct {
	srv *http.Server
	log *slog.Logger
	db  *pgxpool.Pool
}

func (s *Server) ListenAndServe() error {
	s.log.Info("http: listening", slog.String("addr", s.srv.Addr))
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func (b *Builder) WithDB(db *pgxpool.Pool) *Builder {
	b.db = db
	return b
}

func (b *Builder) WithDefaultEndpoints() *Builder {
	hh := handlers.NewHealthHandlers(b.log, b.db)

	b.r.GET("/live", hh.Live)
	b.r.GET("/ready", hh.Ready)

	v1 := b.r.Group("/v1")
	v1.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	v1.GET("/db/ping", hh.DBPing)

	return b
}

//func (b *Builder) WithUsers(svc *app.Service) *Builder {
//	uh := handlers.NewUsersHandlers(b.log, svc)
//
//	v1 := b.r.Group("/v1")
//	u := v1.Group("/users")
//	{
//		u.POST("/register", uh.Register)
//		u.POST("/login", uh.Login)
//	}
//	return b
//}

func (b *Builder) WithUsersAuth(svc *app.Service, jwtm *jwtauth.Manager) *Builder {
	sessRepo := sessionpg.New(b.db)

	uh := handlers.NewUsersHandlers(b.log, svc, jwtm, sessRepo)

	v1 := b.r.Group("/v1")
	u := v1.Group("/users")
	{
		// public
		u.POST("/register", uh.Register)
		u.POST("/login", uh.Login)
		u.POST("/refresh", uh.Refresh)
		u.POST("/logout", uh.Logout) // по refresh token, без auth

		// protected
		auth := u.Group("")
		auth.Use(handlers.Auth(b.log, jwtm))
		auth.GET("/me", uh.Me)
		auth.POST("/logout_all", uh.LogoutAll)
	}
	return b
}

func (b *Builder) Build() *Server {
	hs := &http.Server{
		Addr:              b.cfg.Addr,
		Handler:           b.r,
		ReadTimeout:       b.cfg.ReadTimeout,
		WriteTimeout:      b.cfg.WriteTimeout,
		IdleTimeout:       b.cfg.IdleTimeout,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return &Server{srv: hs, log: b.log, db: b.db}
}
