package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"

	"try_golang/internal/application"
	"try_golang/internal/domain"
	"try_golang/internal/domain/account"
	"try_golang/internal/domain/task"
)

// maxBodyBytes は、入力を読み取る量の上限です。業務ルールとは別の、HTTP入口の容量制限です。
const maxBodyBytes = 8192

// respondError は、全ハンドラーで共通するJSONエラー形式を返し、後続処理を停止します。
// 401には認証方式を伝えるWWW-Authenticateも付けます。
// Abortはミドルウェアの後続実行を止めますが、現在のGo関数自体をreturnするものではありません。
func respondError(c *gin.Context, status int, code, message string) {
	if status == http.StatusUnauthorized {
		c.Header("WWW-Authenticate", `Bearer realm="tasks"`)
	}
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

// fail は、ドメイン・ユースケースのエラーをHTTPステータスと公開用メッセージへ変換します。
//
// 内側の層はHTTPを知らず、外側のこの関数が通信上の表現を決めます。
// %wで包まれた原因もerrors.Isで判定します。分類していない障害はログへ残し、
// クライアントには内部の詳細を含まない500を返します。
func fail(c *gin.Context, err error, logger *slog.Logger) {
	switch {
	case errors.Is(err, application.ErrUnauthenticated):
		respondError(c, 401, "unauthenticated", "authentication required")
	case errors.Is(err, domain.ErrNotFound):
		respondError(c, 404, "not_found", "resource not found")
	case errors.Is(err, domain.ErrConflict):
		respondError(c, 409, "conflict", "resource already exists")
	case errors.Is(err, task.ErrCompleted):
		respondError(c, 409, "task_completed", "completed task cannot be changed")
	case errors.Is(err, account.ErrInvalidEmail):
		respondError(c, 400, "invalid_email", "valid email is required")
	case errors.Is(err, account.ErrInvalidPassword):
		respondError(c, 400, "invalid_password", "password must be 12 to 72 bytes")
	case errors.Is(err, task.ErrInvalidTitle):
		respondError(c, 400, "invalid_title", "title must contain 1 to 200 characters")
	default:
		logger.Error("request failed", "route", c.FullPath(), "error", err)
		respondError(c, 500, "internal_error", "internal server error")
	}
}

// decode は、Content-Typeと容量を確認し、JSONをdestinationへ読み込みます。
//
// destinationには入力DTOのポインターを渡します。未知の項目と複数のJSON値を拒否し、
// 不正な入力ならHTTPエラーを返してfalseにします。呼び出し側はfalseなら直ちにreturnします。
// JSONの読み取りと業務上の検証は別です。空の項目やnullを読めた場合にも、
// 必須値・メール形式・タイトル長などはユースケースとドメインで検証します。
func decode(c *gin.Context, destination any) bool {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		respondError(c, 415, "unsupported_media_type", "Content-Type must be application/json")
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(destination)
	if err == nil {
		// 一つ目を読めただけでは末尾に別のJSONがある可能性があるため、EOFまで確認します。
		var extra any
		if err = decoder.Decode(&extra); errors.Is(err, io.EOF) {
			return true
		}
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		respondError(c, 413, "body_too_large", "request body exceeds 8192 bytes")
	} else {
		respondError(c, 400, "invalid_json", "a single JSON object with supported fields is required")
	}
	return false
}
