package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"try_golang/internal/domain/account"
)

// Auth は、登録・ログイン・認証・ログアウトを調整するユースケースです。
//
// 入力をドメインのルールで検証し、保存や暗号処理をポートへ依頼します。
// HTTPステータスやGinのContextを知らないため、CLIなど別の入口からも呼べます。
// コンストラクターへ依存を渡す形が依存性の注入（DI）です。
type Auth struct {
	users     Users         // ユーザー集約の保存・検索先です。
	sessions  Sessions      // セッションを保存し、ログアウト時に削除する窓口です。
	passwords Passwords     // ハッシュの方式を隠す窓口です。
	generate  Generator     // ユーザーIDと推測困難なトークンを生成します。
	now       Clock         // 登録日時と期限の判定に使う時刻取得関数です。
	ttl       time.Duration // ログイン時刻から有効期限までの期間です。
	dummyHash string        // 未登録ユーザーにもハッシュ照合を行うための照合用データです。
}

// Token は、ログインに成功した呼び出し側へ返す認証結果です。
// 保存用のSessionとは役割が異なり、こちらには利用者へ渡す平文トークンを含めます。
type Token struct {
	// AccessToken はBearer認証で提示する秘密の値です。ログや保存層へ平文で出力しません。
	AccessToken string
	// ExpiresAt はセッションの有効期限です。期限後は再ログインが必要です。
	ExpiresAt time.Time
}

// NewAuthUseCase は、各ポートと時刻・生成処理・セッション期間を受け取り、Authを組み立てます。
//
// ttlが正でない場合や、ダミーハッシュの準備に失敗した場合はエラーを返します。
// 依存は全て利用可能な実装を渡してください。nil依存の検査は行いません。
// ダミーハッシュは初期化時に一度作り、未登録ユーザーのログインでも照合処理を実行できるようにします。
func NewAuthUseCase(users Users, sessions Sessions, passwords Passwords, generate Generator, now Clock, ttl time.Duration) (*Auth, error) {
	if ttl <= 0 {
		return nil, fmt.Errorf("session TTL must be positive")
	}
	dummy, err := passwords.Hash("dummy-password-for-timing")
	if err != nil {
		return nil, fmt.Errorf("prepare password verifier: %w", err)
	}
	return &Auth{users: users, sessions: sessions, passwords: passwords, generate: generate, now: now, ttl: ttl, dummyHash: dummy}, nil
}

// RegisterUseCase は、メールとパスワードを検証し、新しいUserを生成して保存します。
//
// メール・パスワードの不正はドメインのエラー、重複はErrConflictです。
// 保存層で一意性を保証するため、事前検索だけに頼らず同時登録にも対応します。
// 暗号処理やID生成に失敗した場合は保存せず、原因を%wで包んで返します。
// 呼び出し側はerrors.Isで原因を判定できます。登録だけではセッションを発行しません。
func (a *Auth) RegisterUseCase(ctx context.Context, rawEmail, password string) (account.User, error) {
	email, err := account.NewEmail(rawEmail)
	if err != nil {
		return account.User{}, err
	}
	if err = account.ValidatePassword(password); err != nil {
		return account.User{}, err
	}
	hash, err := a.passwords.Hash(password)
	if err != nil {
		return account.User{}, fmt.Errorf("hash password: %w", err)
	}
	id, err := a.generate()
	if err != nil {
		return account.User{}, fmt.Errorf("generate user ID: %w", err)
	}
	user, err := account.NewUser(id, email, hash, a.now())
	if err != nil {
		return account.User{}, fmt.Errorf("create user: %w", err)
	}
	if err = a.users.InsertUser(ctx, user); err != nil {
		return account.User{}, fmt.Errorf("register user: %w", err)
	}
	return user, nil
}

// LoginUseCase は、資格情報を照合し、期限付きの新しいセッションとTokenを作成します。
//
// ユーザー未存在とパスワード不一致はいずれもErrUnauthenticatedを返します。
// 有効な形式のメール・パスワードでは、未登録ユーザーにもダミーハッシュを照合して、
// 検索結果だけで照合を省略する場合の応答時間の差を小さくします。時間が完全に同一になる保証はありません。
// セッションの保存に成功してからTokenを返すため、保存失敗時に利用できないトークンを発行しません。
func (a *Auth) LoginUseCase(ctx context.Context, rawEmail, password string) (Token, error) {
	email, err := account.NewEmail(rawEmail)
	if err != nil || account.ValidatePassword(password) != nil {
		return Token{}, ErrUnauthenticated
	}
	user, err := a.users.UserByEmail(ctx, email)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return Token{}, fmt.Errorf("find user: %w", err)
	}
	hash := a.dummyHash
	if err == nil {
		hash = user.PasswordHash()
	}
	verified := a.passwords.Verify(hash, password)
	if !verified || err != nil {
		return Token{}, ErrUnauthenticated
	}
	rawToken, err := a.generate()
	if err != nil {
		return Token{}, fmt.Errorf("generate token: %w", err)
	}
	expiresAt := a.now().Add(a.ttl)
	// 保存用データと応答用データを分け、保存層に生のBearerトークンを渡さないようにします。
	session := Session{TokenHash: tokenHash(rawToken), UserID: user.ID(), ExpiresAt: expiresAt}
	if err = a.sessions.SaveSession(ctx, session); err != nil {
		return Token{}, fmt.Errorf("save session: %w", err)
	}
	return Token{AccessToken: rawToken, ExpiresAt: expiresAt}, nil
}

// AuthenticateUseCase は、提示されたトークンからセッションとUserを取得し、利用者を確認します。
//
// 空・過大なトークン、未知のセッション、期限切れ、Userの未存在はErrUnauthenticatedです。
// 有効期限と同じ時刻になった時点で失効し、期限切れの記録を削除します。
// 保存層の障害は認証失敗と混同せず原因を返し、認証できたUserも返しません。
// この認証と、各タスクの所有者かを確認する認可は別の処理です。
func (a *Auth) AuthenticateUseCase(ctx context.Context, rawToken string) (account.User, error) {
	if rawToken == "" || len(rawToken) > 1024 {
		return account.User{}, ErrUnauthenticated
	}
	session, err := a.sessions.FindSession(ctx, tokenHash(rawToken))
	if errors.Is(err, ErrNotFound) {
		return account.User{}, ErrUnauthenticated
	}
	if err != nil {
		return account.User{}, fmt.Errorf("find session: %w", err)
	}
	if !a.now().Before(session.ExpiresAt) {
		if err = a.sessions.DeleteSession(ctx, session.TokenHash); err != nil {
			return account.User{}, fmt.Errorf("delete expired session: %w", err)
		}
		return account.User{}, ErrUnauthenticated
	}
	user, err := a.users.UserByID(ctx, session.UserID)
	if errors.Is(err, ErrNotFound) {
		return account.User{}, ErrUnauthenticated
	}
	if err != nil {
		return account.User{}, fmt.Errorf("find session user: %w", err)
	}
	return user, nil
}

// LogoutUseCase は、指定トークンのセッションだけを削除します。同じUserの別セッションは維持します。
//
// 空のトークンはErrUnauthenticatedです。未存在の記録の削除はSessionsの契約上成功します。
// このメソッド自体は事前認証を行わないため、HTTP層ではauthenticateを通してから呼びます。
// 保存層が削除に失敗した場合は、成功扱いにせずエラーを返します。
func (a *Auth) LogoutUseCase(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return ErrUnauthenticated
	}
	if err := a.sessions.DeleteSession(ctx, tokenHash(rawToken)); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

// tokenHash は、提示されたトークンをSessionの検索キーへ変換します。
// トークンは十分な乱数を使う前提なのでSHA-256で扱い、利用者が選ぶパスワードの
// ハッシュ化（Passwordsポート）とは方式を分けています。平文には戻せません。
func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
