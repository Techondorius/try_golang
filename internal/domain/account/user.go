// Package account は、ユーザーの集約とメールアドレスの値オブジェクトを定義します。
//
// この層は「有効なユーザーとは何か」という業務上のルールを扱います。
// HTTP、データベース、bcryptの具体的な処理は外側の層が担当するため、
// ドメインを単体でテストしたり、保存方法を変更したりできます。
package account

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

var (
	// ErrInvalidEmail は、正規化後の入力をメールアドレスとして受け付けられない場合のエラーです。
	ErrInvalidEmail = errors.New("invalid email")
	// ErrInvalidPassword は、パスワードの長さが12〜72バイトの範囲外であることを表します。
	ErrInvalidPassword = errors.New("password must be 12 to 72 bytes")
	// ErrInvalidUser は、ユーザー生成に必要なID・Email・ハッシュが欠けていることを表します。
	ErrInvalidUser = errors.New("user identity, email and password hash are required")
)

// Email は、正規化済みのメールアドレスを表す値オブジェクトです。
//
// 値オブジェクトはIDを持たず、内容が同じなら同じ値として扱います。
// Email同士は == で比較できます。valueを非公開にし、NewEmailを入口にすることで、
// 外側のコードが検証を飛ばして値を書き換えることを防いでいます。
// ゼロ値は有効なメールアドレスではなく、NewUserも受け付けません。
type Email struct{ value string }

// NewEmail は、入力の前後の空白を除去し、小文字に正規化してEmailを生成します。
//
// このサンプルではアドレス全体の大文字・小文字を区別しません。
// 表示名付きの入力や254バイトを超える入力などはErrInvalidEmailを返します。
// 形式の検証だけを行うため、実際にメールが届くことまでは保証しません。
func NewEmail(raw string) (Email, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || len(value) > 254 || !strings.Contains(value, "@") {
		return Email{}, ErrInvalidEmail
	}
	return Email{value}, nil
}

// String は、正規化済みのアドレスを返します。ログやHTTP応答への変換に利用できます。
func (e Email) String() string { return e.value }

// ValidatePassword は、このサンプルのパスワード長のルールを検証します。
//
// 長さは文字数ではなくバイト数です。日本語などは1文字が複数バイトになります。
// 72バイトという上限は、採用するbcryptアダプターの制限に合わせています。
// この関数はハッシュ化や保存を行わず、不正な長さの場合はErrInvalidPasswordを返します。
func ValidatePassword(raw string) error {
	if len(raw) < 12 || len(raw) > 72 {
		return ErrInvalidPassword
	}
	return nil
}

// User は、ユーザー情報をまとめて有効な状態に保つ集約のルートです。
//
// 集約は、関連する値とそのルールを一つの操作単位として扱う境界です。
// UserはIDで識別するエンティティであり、Emailはその中に含まれる値オブジェクトです。
// 登録メールの重複は他のUserとの比較が必要なので、保存層の一意性保証が担当します。
// フィールドは非公開ですが、Goのゼロ値は作成できるため、通常はNewUserで生成します。
type User struct {
	id           string    // 集約を識別するID。メールアドレスとは独立した識別子です。
	email        Email     // 検証・正規化を済ませたメールアドレスです。
	passwordHash string    // 外側でハッシュ化した照合用データ。平文のパスワードは保持しません。
	createdAt    time.Time // 呼び出し側から受け取った登録日時です。
}

// NewUser は、必須のID・Email・パスワードハッシュが揃ったUserを生成します。
//
// 検証済みのEmailと、Passwordsポートで作ったハッシュを渡してください。
// ハッシュの生成・形式検証やメールの一意性確認は、このコンストラクターでは行いません。
// 必須値が欠けていればErrInvalidUserを返します。nowを引数にすることで、
// ドメイン内で現在時刻を取得せず、テストで作成日時を固定できます。
func NewUser(id string, email Email, passwordHash string, now time.Time) (User, error) {
	if id == "" || email.value == "" || passwordHash == "" {
		return User{}, ErrInvalidUser
	}
	return User{id: id, email: email, passwordHash: passwordHash, createdAt: now}, nil
}

// ID は、ユーザーの識別子を返します。Taskから所有者を参照する際にも使います。
func (u User) ID() string { return u.id }

// Email は、ユーザーに登録されたメールアドレスの値を返します。
func (u User) Email() Email { return u.email }

// PasswordHash は、認証ユースケースがパスワード照合に使用するハッシュを返します。
// HTTP応答への出力は避け、必要な項目だけを持つDTOへ変換してください。
func (u User) PasswordHash() string { return u.passwordHash }

// CreatedAt は、ユーザーを生成した日時を返します。
func (u User) CreatedAt() time.Time { return u.createdAt }
