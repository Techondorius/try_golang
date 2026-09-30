package httpapi

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"try_golang/internal/domain/account"
)

func observe(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		defer func() {
			if recover() != nil {
				logger.Error("request panicked", "route", c.FullPath())
				respondError(c, 500, "internal_error", "internal server error")
			}
			logger.Info("http request", "method", c.Request.Method, "route", c.FullPath(), "status", c.Writer.Status(), "duration", time.Since(start))
		}()
		c.Next()
	}
}
func bearer(c *gin.Context) string {
	fields := strings.Fields(c.GetHeader("Authorization"))
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return ""
	}
	return fields[1]
}
func (h handler) authenticate(c *gin.Context) {
	user, err := h.auth.Authenticate(c.Request.Context(), bearer(c))
	if err != nil {
		fail(c, err, h.logger)
		return
	}
	c.Set("user", user)
	c.Next()
}
func currentUser(c *gin.Context) account.User { return c.MustGet("user").(account.User) }
