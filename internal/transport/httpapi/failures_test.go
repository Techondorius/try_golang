package httpapi_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"try_golang/internal/application"
	"try_golang/internal/infrastructure/memory"
	"try_golang/internal/infrastructure/security"
	"try_golang/internal/transport/httpapi"
)

type failingSessionLookup struct {
	application.Sessions
	panicInstead bool
}

func (s failingSessionLookup) FindSession(context.Context, string) (application.Session, error) {
	if s.panicInstead {
		panic("sensitive-panic-detail")
	}
	return application.Session{}, errors.New("sensitive-storage-detail")
}

func TestInternalFailuresReturnGenericJSONWithoutCredentials(t *testing.T) {
	for _, panicInstead := range []bool{false, true} {
		t.Run(map[bool]string{false: "storage failure", true: "panic"}[panicInstead], func(t *testing.T) {
			store := memory.New(time.Now)
			auth, err := application.NewAuth(store, failingSessionLookup{store, panicInstead}, security.Bcrypt{Cost: bcrypt.MinCost}, security.RandomID, time.Now, time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			var logs strings.Builder
			router, err := httpapi.NewRouter(auth, application.NewTasks(store, security.RandomID, time.Now), httpapi.Options{Logger: slog.New(slog.NewTextHandler(&logs, nil))})
			if err != nil {
				t.Fatal(err)
			}
			out := request(router, "GET", "/v1/me", "", "sensitive-bearer-token")
			wantStatus(t, out, 500)
			if strings.Contains(out.Body.String(), "sensitive") {
				t.Fatal("internal details exposed")
			}
			if object(t, out)["error"].(map[string]any)["code"] != "internal_error" {
				t.Fatal("wrong error contract")
			}
			if strings.Contains(logs.String(), "sensitive-bearer-token") || strings.Contains(logs.String(), "sensitive-panic-detail") {
				t.Fatal("credentials or panic payload logged")
			}
		})
	}
}
