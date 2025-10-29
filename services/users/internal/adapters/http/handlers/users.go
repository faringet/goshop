package handlers

import (
	"crypto/sha256"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"log/slog"

	"goshop/pkg/jwtauth"
	"goshop/services/users/internal/adapters/repo/sessionpg"
	"goshop/services/users/internal/adapters/repo/userpg"
	"goshop/services/users/internal/app"
	domain "goshop/services/users/internal/domain/user"
)

type UsersHandlers struct {
	log      *slog.Logger
	svc      *app.Service
	jwtm     *jwtauth.Manager
	sessions *sessionpg.Repo
}

func NewUsersHandlers(log *slog.Logger, svc *app.Service, jwtm *jwtauth.Manager, sess *sessionpg.Repo) *UsersHandlers {
	return &UsersHandlers{log: log, svc: svc, jwtm: jwtm, sessions: sess}
}

type registerReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type registerResp struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type loginResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (h *UsersHandlers) Register(c *gin.Context) {
	noCache(c)

	l := ReqLog(c, h.log)

	var in registerReq
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if in.Email == "" || in.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
		return
	}

	u, err := h.svc.Register(c.Request.Context(), in.Email, in.Password)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidEmail):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email"})
			return
		case errors.Is(err, app.ErrWeakPassword):
			c.JSON(http.StatusBadRequest, gin.H{"error": "weak password"})
			return
		case errors.Is(err, userpg.ErrEmailTaken):
			c.JSON(http.StatusConflict, gin.H{"error": "email already taken"})
			return
		default:
			l.Error("users.register: failed", slog.Any("err", err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
	}

	c.JSON(http.StatusCreated, registerResp{
		ID:        u.ID.String(),
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	})
}

func (h *UsersHandlers) Login(c *gin.Context) {
	noCache(c)

	l := ReqLog(c, h.log)

	var in loginReq
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if in.Email == "" || in.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
		return
	}

	u, err := h.svc.Authenticate(c.Request.Context(), in.Email, in.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	access, refresh, jti, err := h.jwtm.GeneratePair(u.ID.String(), u.Email)
	if err != nil {
		l.Error("users.login: jwt.GeneratePair failed", slog.Any("err", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	claims, err := h.jwtm.ParseAndVerify(refresh)
	if err != nil || claims.ExpiresAt == nil {
		l.Error("users.login: parse refresh failed", slog.Any("err", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	expiresAt := claims.ExpiresAt.Time

	hash := sha256.Sum256([]byte(refresh))
	ua := c.Request.UserAgent()
	ip := net.ParseIP(c.ClientIP())

	if _, err := h.sessions.CreateSession(
		c.Request.Context(),
		jti,
		u.ID,
		hash[:],
		expiresAt,
		ua,
		ip,
	); err != nil {
		l.Error("users.login: sessions.CreateSession failed", slog.Any("err", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, loginResp{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    h.jwtm.ExpiresIn(),
	})
}
