package httpadp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"

	"goshop/pkg/httpx"
	"goshop/pkg/jwtauth"

	"goshop/services/orders/internal/adapters/http/handlers"
	"goshop/services/orders/internal/adapters/repo/orderpg"
)

type Module struct {
	log  *slog.Logger
	db   *pgxpool.Pool
	repo *orderpg.Repository
	jwtm *jwtauth.Manager
}

func NewModule(log *slog.Logger, db *pgxpool.Pool, repo *orderpg.Repository, jwtm *jwtauth.Manager) *Module {
	return &Module{
		log:  log,
		db:   db,
		repo: repo,
		jwtm: jwtm,
	}
}

func (m *Module) Name() string { return "orders.http" }

func (m *Module) Mount(r *gin.Engine) error {
	m.log.Info("http: mounting module", slog.String("module", m.Name()))

	// Health
	hh := handlers.NewHealthHandlers(m.log, m.db)
	r.GET("/live", hh.Live)
	r.GET("/ready", hh.Ready)

	v1 := r.Group("/v1")
	v1.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	v1.GET("/db/ping", hh.DBPing)

	// Orders
	oh := handlers.NewOrdersHandlers(m.log, m.repo)

	// Secured (Access JWT с aud="api")
	secured := v1.Group("")
	secured.Use(httpx.AuthJWTExpectAudience(m.log, m.jwtm, "api"))
	secured.POST("/orders", oh.Create)

	m.log.Info("http: routes registered",
		slog.String("module", m.Name()),
		slog.String("base", "/v1"),
		slog.String("group", "/v1/orders"),
	)

	return nil
}
