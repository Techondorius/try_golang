// Package application は、ドメインの操作を組み合わせて利用者の目的を実現するユースケース層です。
//
// ドメインが「タスクを完了できる条件」を持つのに対し、この層は「誰が操作するか」を確認し、
// 対象を取得してドメインの操作を呼び、結果を保存します。Ginや具体的な保存方式には依存しません。
//
// User・Task集約のRepositoryは、それぞれのdomainパッケージが所有するinterfaceです。
// この層は認証セッション・パスワード処理・時刻・ID生成の技術的なポートを定義します。
// infrastructure層の実装（アダプター）が各ポートを満たし、main.goで組み立てられます。
// これが依存性逆転です。テストでは同じポートをFakeで満たし、実時間やDBに頼らず検証できます。
package application

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrUnauthenticated は、認証情報がない・不正・期限切れなどで利用者を確認できないことを表します。
	ErrUnauthenticated = errors.New("authentication required")
)

// Clock は、現在時刻を取得するための関数型のポートです。
// 本番ではtime.Now、テストでは固定・変更可能な時刻を返す関数を渡します。
// 期限切れをテストするために実際の一時間を待つ必要がなくなります。
type Clock func() time.Time

// Generator は、IDや認証トークンを生成するための関数型のポートです。
// 本番では暗号学的な乱数、テストでは順番に値を返すFakeなどを使います。
// 同じポートがトークン生成にも使われるため、本番の値には十分な推測困難性が必要です。
type Generator func() (string, error)

// IPasswordHasher は、パスワードのハッシュ化と照合を抽象化するポートです。
// ハッシュの方式をユースケースから切り離し、本番のbcryptをテストでFakeに置き換えられます。
// 平文を保存する実装はこのポートの意図を満たしません。
type IPasswordHasher interface {
	// Hash は、平文から保存用のハッシュを生成します。暗号処理に失敗したらエラーを返します。
	Hash(password string) (string, error)
	// Verify は、保存済みハッシュと入力パスワードを照合します。
	// 不一致や不正なハッシュにはfalseを返し、認証失敗として扱います。
	Verify(hash, password string) bool
}

// Session は、ログインによって作成されるサーバー側の認証記録です。
//
// UserやTaskの業務上の集約とは別に、application層の認証処理を支えるデータです。
// トークンの平文を保存せず、ハッシュを検索キーとして利用します。
// 期限切れの判定はAuthが行い、ISessionsRepositoryポートは保存・取得・削除を担当します。
type Session struct {
	// TokenHash は、BearerトークンのSHA-256ハッシュです。平文のトークンではありません。
	TokenHash string
	// UserID は、このセッションで認証されるUser集約のIDです。
	UserID string
	// ExpiresAt は有効期限です。この時刻と同じ時刻になった時点で失効します。
	ExpiresAt time.Time
}

// ISessionsRepository は、認証記録を保存・検索・削除するポートです。
// サーバー側に記録を持つ方式なので、記録の削除で個別のログアウトを実現できます。
// このサンプルのSessionは業務集約ではなく認証処理の記録なので、applicationに定義します。
// Repositoryという名前だけで配置を決めず、保存対象の責務で層を選びます。
type ISessionsRepository interface {
	// SaveSession は認証記録を新規保存します。同じTokenHashの重複はdomain.ErrConflictです。
	SaveSession(ctx context.Context, session Session) error
	// FindSession はハッシュで認証記録を取得します。未存在ならdomain.ErrNotFoundです。
	// 取得できても有効期限内とは限らないため、呼び出し側が期限を確認します。
	FindSession(ctx context.Context, tokenHash string) (Session, error)
	// DeleteSession は指定ハッシュの記録を失効させます。既に未存在なら成功として扱います。
	DeleteSession(ctx context.Context, tokenHash string) error
}
