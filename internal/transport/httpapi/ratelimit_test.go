package httpapi_test

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"try_golang/internal/application"
	"try_golang/internal/infrastructure/memory"
	"try_golang/internal/infrastructure/security"
	"try_golang/internal/transport/httpapi"
)

// limitedRouter は、Clockと受付上限を注入したGinを作り、接続元IPを指定する要求関数を返します。
// 転送元ヘッダーの値も別に指定し、実際の接続元と偽装できる入力を区別して検証します。
func limitedRouter(t *testing.T, limit int) (*httpapi.Options, func(ip, forwarded string) *httptest.ResponseRecorder, *time.Time) {
	t.Helper()
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	store := memory.New(clock)
	auth, err := application.NewAuthUseCase(store, store, security.Bcrypt{Cost: bcrypt.MinCost}, security.RandomID, clock, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	options := httpapi.Options{Now: clock, AuthLimit: limit, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	router, err := httpapi.NewRouter(auth, application.NewTasksUseCase(store, security.RandomID, clock), options)
	if err != nil {
		t.Fatal(err)
	}
	send := func(ip, forwarded string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/v1/auth/login", strings.NewReader(`{"email":"missing@example.com","password":"wrong-password"}`))
		req.RemoteAddr = ip + ":1234"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", forwarded)
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		return out
	}
	return &options, send, &now
}

func TestAuthRateLimitAndUntrustedProxy(t *testing.T) {
	_, send, now := limitedRouter(t, 2)
	wantStatus(t, send("192.0.2.1", "198.51.100.1"), 401)
	wantStatus(t, send("192.0.2.1", "198.51.100.2"), 401)
	out := send("192.0.2.1", "198.51.100.3")
	wantStatus(t, out, 429)
	if out.Header().Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
	wantStatus(t, send("192.0.2.2", ""), 401)
	*now = now.Add(time.Minute)
	wantStatus(t, send("192.0.2.1", ""), 401)
}

func TestConcurrentRateLimit(t *testing.T) {
	_, send, _ := limitedRouter(t, 5)
	var accepted, limited atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Go(func() {
			out := send("192.0.2.1", "")
			switch out.Code {
			case 401:
				accepted.Add(1)
			case 429:
				limited.Add(1)
			default:
				t.Errorf("unexpected %d", out.Code)
			}
		})
	}
	wg.Wait()
	if accepted.Load() != 5 || limited.Load() != 15 {
		t.Fatalf("accepted %d, limited %d", accepted.Load(), limited.Load())
	}
}
