package handlers

import (
	"crypto/sha256"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"log/slog"

	"goshop/services/users/internal/adapters/repo/sessionpg"
)

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

type refreshResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (h *UsersHandlers) Refresh(c *gin.Context) {
	noCache(c)

	l := ReqLog(c, h.log)

	var in refreshReq
	if err := c.ShouldBindJSON(&in); err != nil || in.RefreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	claims, err := h.jwtm.ParseAndVerify(in.RefreshToken)
	if err != nil || claims == nil || claims.ID == "" || claims.Subject == "" || claims.ExpiresAt == nil {
		l.Warn("users.refresh: token verify failed", slog.Any("err", err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	oldID, err := uuid.Parse(claims.ID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	oldHash := sha256.Sum256([]byte(in.RefreshToken))

	access, newRefresh, newJTI, err := h.jwtm.GeneratePair(userID.String(), claims.Email)
	if err != nil {
		l.Error("users.refresh: GeneratePair failed", slog.Any("err", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	newClaims, err := h.jwtm.ParseAndVerify(newRefresh)
	if err != nil || newClaims.ExpiresAt == nil {
		l.Error("users.refresh: ParseAndVerify(new) failed", slog.Any("err", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	newExp := newClaims.ExpiresAt.Time
	newHash := sha256.Sum256([]byte(newRefresh))

	ua := c.Request.UserAgent()
	ip := net.ParseIP(c.ClientIP())

	if _, err := h.sessions.RotateSession(
		c.Request.Context(),
		oldID,
		oldHash[:],
		newJTI,
		newHash[:],
		newExp,
		ua,
		ip,
	); err != nil {
		switch err {
		case sessionpg.ErrNotFound, sessionpg.ErrRevoked, sessionpg.ErrRotated, sessionpg.ErrExpired:
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired session"})
			return
		case sessionpg.ErrRefreshReuse:
			l.Warn("users.refresh: reuse detected", slog.String("session_id", oldID.String()))
			c.JSON(http.StatusConflict, gin.H{"error": "refresh token reused"})
			return
		default:
			l.Error("users.refresh: RotateSession failed", slog.Any("err", err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
	}

	c.JSON(http.StatusOK, refreshResp{
		AccessToken:  access,
		RefreshToken: newRefresh,
		TokenType:    "Bearer",
		ExpiresIn:    h.jwtm.ExpiresIn(),
	})
}
