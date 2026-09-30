package httpapi

import (
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"try_golang/internal/application"
)

// window は、一つの接続元IPに割り当てる固定窓の受付回数と終了時刻です。
// 最初の要求で一分の窓を開始し、窓内で上限まで受け付けます。
type window struct {
	count int       // この窓で受け付けた登録・ログイン要求の合計です。
	until time.Time // この時刻になったら新しい窓へ切り替えます。
}

// authLimiter は、重いbcrypt処理へ進む要求を接続元IPごとに制限するHTTP層の仕組みです。
//
// User集約の業務ルールではなく、通信の入口を保護する責務として配置します。
// プロセス内のmapとmutexを使うため、複数インスタンスでは受付回数を共有しません。
// NewRouterが初期化し、登録とログインに同じインスタンスを使います。
type authLimiter struct {
	mu      sync.Mutex        // 上限確認と加算を一体として保護します。
	windows map[string]window // 接続元IPごとの窓です。最大4096件を保持します。
	limit   int               // 一分の窓内での受付上限です。
	now     application.Clock // 窓の切り替え判定に使います。
}

// allow は、ipの要求を受け付けるかと、拒否時に再試行するまでの秒数を返します。
//
// 上限確認と回数加算を同じロック内で行い、並行要求で上限を越えることを防ぎます。
// 新しいIPの枠が足りなければ期限切れの枠を掃除し、それでも満杯なら拒否します。
// 固定窓方式なので、窓の終わりと次の窓の始まりに要求が集中する場合は、
// 一分間を常にさかのぼって数える方式とは異なる受付結果になります。
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

// middleware は、allowの判定をGinの処理継続または429応答へ変換します。
// ClientIPはNewRouterのプロキシ設定に従います。拒否時にはRetry-Afterを付け、
// 登録・ログインのハンドラーやbcrypt処理へ進ませません。
func (l *authLimiter) middleware(c *gin.Context) {
	allowed, retryAfter := l.allow(c.ClientIP())
	if !allowed {
		c.Header("Retry-After", strconv.Itoa(retryAfter))
		respondError(c, 429, "rate_limited", "too many authentication requests")
		return
	}
	c.Next()
}
