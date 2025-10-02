package httpx

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"log/slog"

	"goshop/pkg/jwtauth"
)

const (
	CtxKeyJWTClaims = "jwt_claims"
	CtxKeyUID       = "uid"
	CtxKeyEmail     = "email"
)

func AuthJWT(rootLog *slog.Logger, jwtm *jwtauth.Manager) gin.HandlerFunc {
	return AuthJWTWith(rootLog, jwtm)
}

type verifier interface {
	ParseAndVerify(token string) (*jwtauth.Claims, error)
}

func AuthJWTExpectAudience(rootLog *slog.Logger, v verifier, expectAudience string) gin.HandlerFunc {
	return authJWTInternal(rootLog, v, expectAudience)
}

func AuthJWTWith(rootLog *slog.Logger, v verifier) gin.HandlerFunc {
	return authJWTInternal(rootLog, v, "")
}

func authJWTInternal(rootLog *slog.Logger, v verifier, expectAudience string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")

		l := rootLog
		if rl, ok := c.Get(CtxKeyLogger); ok {
			if reqLog, ok := rl.(*slog.Logger); ok && reqLog != nil {
				l = reqLog
			}
		}

		authz := c.GetHeader("Authorization")
		parts := strings.SplitN(authz, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			l.Warn("auth: missing/invalid Authorization")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid Authorization header"})
			c.Abort()
			return
		}

		claims, err := v.ParseAndVerify(parts[1])
		if err != nil {
			l.Warn("auth: token verify failed", slog.String("err", err.Error()))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		if claims.ExpiresAt != nil && time.Until(claims.ExpiresAt.Time) <= 0 {
			l.Warn("auth: token expired")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
			c.Abort()
			return
		}

		if expectAudience != "" {
			okAud := false
			for _, a := range claims.Audience {
				if a == expectAudience {
					okAud = true
					break
				}
			}
			if !okAud {
				l.Warn("auth: wrong audience", slog.Any("aud", claims.Audience), slog.String("need", expectAudience))
				c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				c.Abort()
				return
			}
		}

		c.Set(CtxKeyJWTClaims, claims)
		c.Set(CtxKeyUID, claims.UserID)
		c.Set(CtxKeyEmail, claims.Email)
		c.Next()
	}
}

func GetJWTClaims(c *gin.Context) (*jwtauth.Claims, bool) {
	v, ok := c.Get(CtxKeyJWTClaims)
	if !ok || v == nil {
		return nil, false
	}
	claims, ok := v.(*jwtauth.Claims)
	return claims, ok
}
