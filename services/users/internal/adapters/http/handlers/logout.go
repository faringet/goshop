package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"log/slog"

	"goshop/pkg/httpx"
)

type logoutReq struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *UsersHandlers) Logout(c *gin.Context) {
	noCache(c)

	l := ReqLog(c, h.log)

	var in logoutReq
	if err := c.ShouldBindJSON(&in); err != nil || in.RefreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	claims, err := h.jwtm.ParseAndVerify(in.RefreshToken)
	if err != nil || claims == nil || claims.ID == "" {
		l.Warn("users.logout: invalid refresh", slog.Any("err", err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	sessID, err := uuid.Parse(claims.ID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	if err := h.sessions.RevokeSession(c.Request.Context(), sessID); err != nil {
		l.Error("users.logout: revoke session failed", slog.Any("err", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *UsersHandlers) LogoutAll(c *gin.Context) {
	noCache(c)

	l := ReqLog(c, h.log)

	claims, ok := httpx.GetJWTClaims(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	n, err := h.sessions.RevokeAll(c.Request.Context(), userID)
	if err != nil {
		l.Error("users.logout_all: revoke all failed", slog.Any("err", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"revoked": n})
}
