// このmainパッケージは、サンプルAPIの起動と終了を担当します。
// ドメインのルールはここに置かず、設定と具体的なアダプターを組み立ててユースケースへ渡します。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"try_golang/internal/application"
	"try_golang/internal/config"
	"try_golang/internal/infrastructure/memory"
	"try_golang/internal/infrastructure/security"
	"try_golang/internal/transport/httpapi"
)

// main は、ログ・設定・終了シグナルを準備してrunへ制御を渡すエントリーポイントです。
// SIGINT（Ctrl+C）やSIGTERMをContextのキャンセルに変換し、正常な終了処理を開始します。
// 起動に必要な設定やサーバー処理が失敗した場合は、ログを記録して終了コード1で停止します。
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err = run(ctx, cfg, logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// run は、具体的な実装を選んで依存を組み立てるComposition Rootです。
//
// メモリ保存、bcrypt、時刻、乱数をユースケースへ渡し、そのユースケースをHTTP層へ渡します。
// 外側のmainが全体を知る一方、内側のドメインはGinや保存実装を知りません。
// HTTPサーバーの実行中は、起動失敗またはctxのキャンセルを待ちます。
// キャンセル時は最大5秒間リクエストの完了を待ち、間に合わなければ接続を閉じてエラーを返します。
func run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	// 同じStoreを三つのポートとして注入します。ポートごとに別の実装を使うこともできます。
	store := memory.New(time.Now)
	auth, err := application.NewAuthUseCase(store, store, security.Bcrypt{Cost: 12}, security.RandomID, time.Now, cfg.SessionTTL)
	if err != nil {
		return err
	}
	tasks := application.NewTasksUseCase(store, security.RandomID, time.Now)
	router, err := httpapi.NewRouter(auth, tasks, httpapi.Options{Logger: logger})
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr: cfg.Addr, Handler: router,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10,
	}
	// ListenAndServeは待ち続けるため、別goroutineで動かして終了結果を受け取ります。
	// バッファを一つ持たせ、結果の送信側が受信待ちで止まらないようにします。
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	logger.Info("server starting", "address", cfg.Addr)
	select {
	case err = <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		// 起動Contextは既にキャンセル済みなので、終了待ち専用のContextを新しく作ります。
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err = server.Shutdown(shutdown); err != nil {
			server.Close()
			return err
		}
		err = <-result
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
