// Package httpapi は、HTTPリクエストをapplication層の呼び出しへ変換する入口のアダプターです。
//
// JSONをDTOへ読み取り、認証済みUserのIDなどをユースケースへ渡し、結果をHTTP応答へ変換します。
// GinのContextやHTTPステータスはこの層で扱い、ドメインの状態遷移はドメイン層に任せます。
// 入口の役割を分離することで、同じユースケースを別の通信方式から呼ぶこともできます。
package httpapi

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"try_golang/internal/application"
)

// Options は、HTTPアダプターのログと認証APIの回数制限に関する設定です。
// ドメインのルールやセッション期限の設定とは別に、通信の入口で必要な値をまとめています。
type Options struct {
	// Now は回数制限の時計です。nilならtime.Nowを使います。
	// Authのセッション判定の時計はNewAuthUseCaseへ別途渡されるため、テスト時は両方へ同じClockを渡せます。
	Now application.Clock
	// Logger はアクセスログと内部障害の記録先です。nilならslog.Defaultを使います。
	Logger *slog.Logger
	// AuthLimit は、同じ接続元IPに対する一分の固定窓での受付上限です。
	// 登録とログインで同じ枠を共有し、0以下なら10を使います。
	AuthLimit int
}

// handler は、HTTP処理から呼ぶユースケースとログ出力先をまとめた内部の型です。
// NewRouterが依存を注入し、各ハンドラーはHTTP変換の責務に集中します。
type handler struct {
	auth   *application.Auth  // 認証とセッション操作を担当するユースケースです。
	tasks  *application.Tasks // 所有者を確認してタスクを操作するユースケースです。
	logger *slog.Logger       // HTTP変換時に発生した内部障害などの記録先です。
}

// NewRouter は、注入されたユースケースをGinのルートとミドルウェアへ結び付けます。
//
// 認証不要の登録・ログインには回数制限を、認証が必要なルートにはauthenticateを適用します。
// 末尾スラッシュの自動リダイレクトを無効にし、未定義URLや未対応メソッドもJSONで返します。
// 転送元ヘッダーは信頼しないため、ClientIPは接続元を基準に回数制限を行います。
// 戻り値はhttp.Handlerとしても使えるため、HTTPサーバーにもhttptestにも渡せます。
// サーバーの待受開始はこの関数では行わず、main.goの責務です。
func NewRouter(auth *application.Auth, tasks *application.Tasks, options Options) (*gin.Engine, error) {
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if options.AuthLimit <= 0 {
		options.AuthLimit = 10
	}
	router := gin.New()
	router.RedirectTrailingSlash = false
	router.HandleMethodNotAllowed = true
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, err
	}
	router.Use(observe(options.Logger), func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Next()
	})
	h := handler{auth, tasks, options.Logger}
	router.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	v1 := router.Group("/v1")
	limiter := &authLimiter{windows: map[string]window{}, limit: options.AuthLimit, now: options.Now}
	publicAuth := v1.Group("/auth")
	publicAuth.Use(limiter.middleware)
	publicAuth.POST("/register", h.register)
	publicAuth.POST("/login", h.login)
	// 共通ミドルウェアを付けたグループにまとめ、各ハンドラーでの認証の付け忘れを防ぎます。
	protected := v1.Group("")
	protected.Use(h.authenticate)
	protected.POST("/auth/logout", h.logout)
	protected.GET("/me", func(c *gin.Context) { c.JSON(200, userDTO(currentUser(c))) })
	protected.POST("/tasks", h.create)
	protected.GET("/tasks", h.list)
	protected.GET("/tasks/:id", h.get)
	protected.PATCH("/tasks/:id", h.rename)
	protected.POST("/tasks/:id/complete", h.complete)
	protected.DELETE("/tasks/:id", h.delete)
	router.NoRoute(func(c *gin.Context) { respondError(c, 404, "not_found", "resource not found") })
	router.NoMethod(func(c *gin.Context) { respondError(c, 405, "method_not_allowed", "method not allowed") })
	return router, nil
}
