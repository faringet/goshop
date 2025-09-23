package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"log/slog"

	"goshop/pkg/jwtauth"
)

const (
	ctxClaimsKey = "jwt_claims"
	ctxUIDKey    = "uid"
	ctxEmailKey  = "email"
)

func Auth(log *slog.Logger, jwtm *jwtauth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		noCache(c)

		authz := c.GetHeader("Authorization")
		parts := strings.SplitN(authz, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid Authorization header"})
			c.Abort()
			return
		}

		claims, err := jwtm.ParseAndVerify(parts[1])
		if err != nil {
			log.Warn("auth: token verify failed", slog.String("err", err.Error()))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		if claims.ExpiresAt != nil && time.Until(claims.ExpiresAt.Time) <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
			c.Abort()
			return
		}

		c.Set(ctxClaimsKey, claims)
		c.Set(ctxUIDKey, claims.UserID)
		c.Set(ctxEmailKey, claims.Email)

		c.Next()
	}
}

func (h *UsersHandlers) Me(c *gin.Context) {
	noCache(c)

	claims, ok := GetClaims(c)
	if !ok {
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

func GetClaims(c *gin.Context) (*jwtauth.Claims, bool) {
	v, ok := c.Get(ctxClaimsKey)
	if !ok || v == nil {
		return nil, false
	}
	claims, ok := v.(*jwtauth.Claims)
	return claims, ok
}
