package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"

	"try_golang/internal/application"
	"try_golang/internal/domain/account"
	"try_golang/internal/domain/task"
)

const maxBodyBytes = 8192

func respondError(c *gin.Context, status int, code, message string) {
	if status == http.StatusUnauthorized {
		c.Header("WWW-Authenticate", `Bearer realm="tasks"`)
	}
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}
func fail(c *gin.Context, err error, logger *slog.Logger) {
	switch {
	case errors.Is(err, application.ErrUnauthenticated):
		respondError(c, 401, "unauthenticated", "authentication required")
	case errors.Is(err, application.ErrNotFound):
		respondError(c, 404, "not_found", "resource not found")
	case errors.Is(err, application.ErrConflict):
		respondError(c, 409, "conflict", "resource already exists")
	case errors.Is(err, task.ErrCompleted):
		respondError(c, 409, "task_completed", "completed task cannot be changed")
	case errors.Is(err, account.ErrInvalidEmail):
		respondError(c, 400, "invalid_email", "valid email is required")
	case errors.Is(err, account.ErrInvalidPassword):
		respondError(c, 400, "invalid_password", "password must be 12 to 72 bytes")
	case errors.Is(err, task.ErrInvalidTitle):
		respondError(c, 400, "invalid_title", "title must contain 1 to 200 characters")
	default:
		logger.Error("request failed", "route", c.FullPath(), "error", err)
		respondError(c, 500, "internal_error", "internal server error")
	}
}

func decode(c *gin.Context, destination any) bool {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		respondError(c, 415, "unsupported_media_type", "Content-Type must be application/json")
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(destination)
	if err == nil {
		var extra any
		if err = decoder.Decode(&extra); errors.Is(err, io.EOF) {
			return true
		}
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		respondError(c, 413, "body_too_large", "request body exceeds 8192 bytes")
	} else {
		respondError(c, 400, "invalid_json", "a single JSON object with supported fields is required")
	}
	return false
}
