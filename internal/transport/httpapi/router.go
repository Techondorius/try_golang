// Package httpapi adapts HTTP DTOs to use cases. Domain aggregates never receive
// a Gin context and credential hashes are never serialized.
package httpapi

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"try_golang/internal/application"
)

type Options struct {
	Now       application.Clock
	Logger    *slog.Logger
	AuthLimit int
}
type handler struct {
	auth   *application.Auth
	tasks  *application.Tasks
	logger *slog.Logger
}

func NewRouter(auth *application.Auth, tasks *application.Tasks, options Options) (*gin.Engine, error) {
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if options.AuthLimit <= 0 {
		options.AuthLimit = 10
	}
	router := gin.New()
	router.RedirectTrailingSlash = false
	router.HandleMethodNotAllowed = true
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, err
	}
	router.Use(observe(options.Logger), func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Next()
	})
	h := handler{auth, tasks, options.Logger}
	router.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	v1 := router.Group("/v1")
	limiter := &authLimiter{windows: map[string]window{}, limit: options.AuthLimit, now: options.Now}
	publicAuth := v1.Group("/auth")
	publicAuth.Use(limiter.middleware)
	publicAuth.POST("/register", h.register)
	publicAuth.POST("/login", h.login)
	protected := v1.Group("")
	protected.Use(h.authenticate)
	protected.POST("/auth/logout", h.logout)
	protected.GET("/me", func(c *gin.Context) { c.JSON(200, userDTO(currentUser(c))) })
	protected.POST("/tasks", h.create)
	protected.GET("/tasks", h.list)
	protected.GET("/tasks/:id", h.get)
	protected.PATCH("/tasks/:id", h.rename)
	protected.POST("/tasks/:id/complete", h.complete)
	protected.DELETE("/tasks/:id", h.delete)
	router.NoRoute(func(c *gin.Context) { respondError(c, 404, "not_found", "resource not found") })
	router.NoMethod(func(c *gin.Context) { respondError(c, 405, "method_not_allowed", "method not allowed") })
	return router, nil
}
