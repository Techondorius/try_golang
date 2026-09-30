package config_test

import (
	"testing"
	"time"

	"try_golang/internal/config"
)

func TestConfig(t *testing.T) {
	cfg, err := config.Load(func(string) string { return "" })
	if err != nil || cfg.Addr != "127.0.0.1:8080" || cfg.SessionTTL != time.Hour {
		t.Fatalf("defaults: %+v %v", cfg, err)
	}
	env := map[string]string{"HTTP_ADDR": ":9090", "SESSION_TTL": "30m"}
	cfg, err = config.Load(func(key string) string { return env[key] })
	if err != nil || cfg.Addr != ":9090" || cfg.SessionTTL != 30*time.Minute {
		t.Fatalf("config: %+v %v", cfg, err)
	}
	for _, bad := range []map[string]string{{"HTTP_ADDR": "8080"}, {"HTTP_ADDR": ":0"}, {"HTTP_ADDR": ":70000"}, {"SESSION_TTL": "no"}, {"SESSION_TTL": "0s"}, {"SESSION_TTL": "-1h"}, {"SESSION_TTL": "25h"}} {
		if _, err := config.Load(func(key string) string { return bad[key] }); err == nil {
			t.Fatalf("accepted %v", bad)
		}
	}
}
