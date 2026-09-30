package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"try_golang/internal/application"
	"try_golang/internal/config"
	"try_golang/internal/infrastructure/memory"
	"try_golang/internal/infrastructure/security"
	"try_golang/internal/transport/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err = run(ctx, cfg, logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// run is the composition root: only this layer chooses concrete adapters.
func run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	store := memory.New(time.Now)
	auth, err := application.NewAuth(store, store, security.Bcrypt{Cost: 12}, security.RandomID, time.Now, cfg.SessionTTL)
	if err != nil {
		return err
	}
	tasks := application.NewTasks(store, security.RandomID, time.Now)
	router, err := httpapi.NewRouter(auth, tasks, httpapi.Options{Logger: logger})
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr: cfg.Addr, Handler: router,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10,
	}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	logger.Info("server starting", "address", cfg.Addr)
	select {
	case err = <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err = server.Shutdown(shutdown); err != nil {
			server.Close()
			return err
		}
		err = <-result
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
