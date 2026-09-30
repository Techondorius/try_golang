package httpapi

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"try_golang/internal/domain/account"
)

// observe は、処理時間・メソッド・ルート・ステータスを記録し、panicを500応答へ変換します。
//
// Ginのミドルウェアはc.Nextで後続の処理を実行し、その前後に共通処理を置けます。
// deferを使い、通常終了でもpanicでも終了時の記録を行います。
// 実際のURLやリクエスト本文ではなくルートの定義を記録し、トークンなどの秘密値は記録しません。
func observe(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		defer func() {
			if recover() != nil {
				logger.Error("request panicked", "route", c.FullPath())
				respondError(c, 500, "internal_error", "internal server error")
			}
			logger.Info("http request", "method", c.Request.Method, "route", c.FullPath(), "status", c.Writer.Status(), "duration", time.Since(start))
		}()
		c.Next()
	}
}

// bearer は、Authorizationヘッダーの「Bearer トークン」を読み取ります。
// 形式が合わなければ空文字列を返します。ここでは文字列を取り出すだけで、
// トークンの有効性や期限はAuth.Authenticateが確認します。
func bearer(c *gin.Context) string {
	fields := strings.Fields(c.GetHeader("Authorization"))
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
		return ""
	}
	return fields[1]
}

// authenticate は、トークンから利用者を確認し、後続ハンドラーへUserを渡すミドルウェアです。
//
// 標準のcontext.Contextはユースケースの中断伝播用、gin.ContextはHTTP応答や
// このリクエスト内のUser共有用として使います。Gin固有の型はユースケースへ渡しません。
// 認証失敗時はfailが後続処理を止め、成功時だけc.Nextで保護されたハンドラーへ進みます。
func (h handler) authenticate(c *gin.Context) {
	user, err := h.auth.AuthenticateUseCase(c.Request.Context(), bearer(c))
	if err != nil {
		fail(c, err, h.logger)
		return
	}
	c.Set("user", user)
	c.Next()
}

// currentUser は、authenticateが確認して格納したUserを取得します。
// 保護されたルートから呼ぶことが前提で、UserがなければMustGetがpanicします。
// リクエスト本文のUser IDを認証結果として扱わないための共有経路です。
func currentUser(c *gin.Context) account.User { return c.MustGet("user").(account.User) }
