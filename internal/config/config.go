// Package config は、環境変数を起動設定へ変換し、起動前に値を検証します。
// ドメインの業務ルールではなく、アプリケーションの動かし方に関する設定を扱います。
package config

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

// Config は、依存を組み立ててHTTPサーバーを起動するための設定値です。
// Loadが入力を検証し、main.goが具体的なサーバーやAuthへ値を渡します。
type Config struct {
	// Addr はHTTPサーバーの待受先です。既定値は127.0.0.1:8080です。
	Addr string
	// SessionTTL はログイン時刻から有効期限までの期間です。既定値は一時間です。
	SessionTTL time.Duration
}

// Load は、getenvで環境変数を取得し、既定値の補完と形式の検証を行います。
//
// HTTP_ADDRはhost:port形式で、ポートは1〜65535です。
// SESSION_TTLはtime.ParseDurationが扱える形式で、0より大きく24時間以下です。
// getenvを引数にすることで、本番ではos.Getenv、テストではmapを読む関数を渡せます。
// 不正な設定はエラーとして返し、サーバー起動後に気づくことを防ぎます。
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
