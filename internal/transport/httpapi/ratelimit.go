package httpapi

import (
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"try_golang/internal/application"
)

type window struct {
	count int
	until time.Time
}
type authLimiter struct {
	mu      sync.Mutex
	windows map[string]window
	limit   int
	now     application.Clock
}

// A process-local fixed window limits expensive bcrypt operations. Entries are
// bounded so spoofed client addresses cannot grow the limiter indefinitely.
func (l *authLimiter) allow(ip string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	current, exists := l.windows[ip]
	if !exists || !now.Before(current.until) {
		if !exists && len(l.windows) >= 4096 {
			for key, item := range l.windows {
				if !now.Before(item.until) {
					delete(l.windows, key)
				}
			}
			if len(l.windows) >= 4096 {
				return false, 60
			}
		}
		current = window{until: now.Add(time.Minute)}
	}
	if current.count >= l.limit {
		return false, int(math.Ceil(current.until.Sub(now).Seconds()))
	}
	current.count++
	l.windows[ip] = current
	return true, 0
}
func (l *authLimiter) middleware(c *gin.Context) {
	allowed, retryAfter := l.allow(c.ClientIP())
	if !allowed {
		c.Header("Retry-After", strconv.Itoa(retryAfter))
		respondError(c, 429, "rate_limited", "too many authentication requests")
		return
	}
	c.Next()
}
