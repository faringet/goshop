package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func GinMiddleware(h *HTTPMetrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		h.infl.Inc()
		start := time.Now()

		c.Next()

		status := c.Writer.Status()
		method := c.Request.Method
		path := h.pathLabeler(c.Request)

		h.reqs.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
		h.dur.WithLabelValues(method, path, strconv.Itoa(status)).Observe(time.Since(start).Seconds())
		h.infl.Dec()
	}
}
