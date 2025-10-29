package handlers

import (
	"log/slog"

	"github.com/gin-gonic/gin"
)

func ReqLog(c *gin.Context, fallback *slog.Logger) *slog.Logger {
	if rl, ok := c.Get("req_logger"); ok {
		if l, ok := rl.(*slog.Logger); ok && l != nil {
			return l
		}
	}
	return fallback
}
