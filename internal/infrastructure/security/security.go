// Package security は、暗号処理と乱数生成を具体的なライブラリで実装するアダプターです。
// ユースケースはPasswordsやGeneratorポートを使うため、この実装に直接依存しません。
package security

import (
	"crypto/rand"
	"encoding/base64"

	"golang.org/x/crypto/bcrypt"

	"try_golang/internal/application"
)

// Bcrypt は、application.Passwordsポートをbcryptで満たすアダプターです。
//
// パスワードごとのランダムなソルトと計算コストはbcryptの生成結果に含まれます。
// main.goは本番用のcost 12、テストは高速に実行できる最低costを選びます。
type Bcrypt struct {
	// Cost はハッシュ生成の計算量を指定します。大きい値ほど照合にも時間がかかります。
	// 有効範囲や既定値への補正は、使用するbcryptライブラリが扱います。
	Cost int
}

// BcryptがPasswordsポートを満たすことをコンパイル時に確認します。
var _ application.Passwords = Bcrypt{}

// Hash は、ランダムなソルトを使い、平文パスワードをbcryptハッシュへ変換します。
// 生成のたびにソルトが変わるため、同じパスワードでも文字列として同じハッシュにはなりません。
// 72バイトを超える入力や不正なcostなど、ライブラリのエラーを呼び出し側へ返します。
func (b Bcrypt) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), b.Cost)
	return string(hash), err
}

// Verify は、保存済みハッシュに含まれる設定を使ってパスワードを照合します。
// 新しくHashを呼んで文字列比較するのではなく、bcryptの照合APIを使います。
// 不一致や読み取れないハッシュなどのエラーはfalseにまとめます。
func (b Bcrypt) Verify(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// RandomID は、crypto/randで生成した32バイト（256ビット）を、URLに使えるBase64へ変換します。
//
// ユーザーID・タスクID・認証トークンに共通して使うGeneratorの実装です。
// 連番や時刻から作る値と異なり、トークンとして利用できる推測困難性を持たせています。
// RawURLEncodingは末尾のパディングを付けないため、返す文字列は43文字になります。
// ユースケースは具体的な乱数処理を知らず、Generatorとして受け取ります。
func RandomID() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
