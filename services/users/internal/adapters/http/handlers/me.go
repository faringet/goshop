package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"goshop/pkg/httpx"
)

func (h *UsersHandlers) Me(c *gin.Context) {
	noCache(c)

	l := ReqLog(c, h.log)

	claims, ok := httpx.GetJWTClaims(c)
	if !ok {
		l.Warn("users.me: missing jwt claims")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"uid":        claims.UserID,
		"email":      claims.Email,
		"issuer":     claims.Issuer,
		"subject":    claims.Subject,
		"issued_at":  claims.IssuedAt,
		"expires_at": claims.ExpiresAt,
	})
}
