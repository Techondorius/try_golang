package config

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

type Config struct {
	Addr       string
	SessionTTL time.Duration
}

func Load(getenv func(string) string) (Config, error) {
	cfg := Config{Addr: "127.0.0.1:8080", SessionTTL: time.Hour}
	if addr := getenv("HTTP_ADDR"); addr != "" {
		cfg.Addr = addr
	}
	_, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return Config{}, fmt.Errorf("HTTP_ADDR must be host:port: %w", err)
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return Config{}, fmt.Errorf("HTTP_ADDR port must be 1 to 65535")
	}
	if raw := getenv("SESSION_TTL"); raw != "" {
		cfg.SessionTTL, err = time.ParseDuration(raw)
		if err != nil || cfg.SessionTTL <= 0 || cfg.SessionTTL > 24*time.Hour {
			return Config{}, fmt.Errorf("SESSION_TTL must be a duration greater than 0 and at most 24h")
		}
	}
	return cfg, nil
}
