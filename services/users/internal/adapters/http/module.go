package httpadp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"

	"goshop/pkg/httpx"
	"goshop/pkg/jwtauth"

	"goshop/services/users/internal/adapters/http/handlers"
	"goshop/services/users/internal/adapters/repo/sessionpg"
	"goshop/services/users/internal/app"
)

type Module struct {
	log   *slog.Logger
	db    *pgxpool.Pool
	svc   *app.Service
	jwtm  *jwtauth.Manager
	srepo *sessionpg.Repo
}

func NewModule(log *slog.Logger, db *pgxpool.Pool, svc *app.Service, jwtm *jwtauth.Manager) *Module {
	return &Module{
		log:   log,
		db:    db,
		svc:   svc,
		jwtm:  jwtm,
		srepo: sessionpg.New(db),
	}
}

func (m *Module) Name() string { return "users.http" }

func (m *Module) Mount(r *gin.Engine) error {
	m.log.Info("http: mounting module", slog.String("module", m.Name()))

	// Health endpoints
	hh := handlers.NewHealthHandlers(m.log, m.db)
	r.GET("/live", hh.Live)
	r.GET("/ready", hh.Ready)

	v1 := r.Group("/v1")
	v1.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	v1.GET("/db/ping", hh.DBPing)

	// Users endpoints
	uh := handlers.NewUsersHandlers(m.log, m.svc, m.jwtm, m.srepo)

	u := v1.Group("/users")
	{
		// public
		u.POST("/register", uh.Register)
		u.POST("/login", uh.Login)
		u.POST("/refresh", uh.Refresh)
		u.POST("/logout", uh.Logout) // по refresh token, без auth

		// protected (Access JWT)
		auth := u.Group("")
		auth.Use(httpx.AuthJWT(m.log, m.jwtm)) // единый мидлвар аутентификации JWT
		auth.GET("/me", uh.Me)
		auth.POST("/logout_all", uh.LogoutAll)
	}

	m.log.Info("http: routes registered",
		slog.String("module", m.Name()),
		slog.String("base", "/v1"),
		slog.String("group", "/v1/users"),
	)

	return nil
}
