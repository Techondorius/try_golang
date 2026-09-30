package task

import (
	"strings"
	"unicode/utf8"
)

// Title は、正規化・検証済みのタスク名を表す値オブジェクトです。
//
// Title自身にはIDがなく、同じ文字列なら == で同じ値と判定できます。
// valueは非公開で、変更用のメソッドも持ちません。改名では既存のTitleを書き換えず、
// NewTitleで新しい値を作ってTask内のTitleを置き換えます。
// ゼロ値は空のタイトルなので、通常はNewTitleを通して生成します。
type Title struct{ value string }

// NewTitle は、前後の空白を取り除き、1〜200 UnicodeコードポイントのTitleを生成します。
//
// UTF-8として不正な文字列や範囲外の長さにはErrInvalidTitleを返します。
// 長さはバイト数ではなくrune数で数えます。結合文字などを含む場合は、
// 画面上で一文字に見える単位とrune数が一致するとは限りません。
func NewTitle(raw string) (Title, error) {
	title := strings.TrimSpace(raw)
	if !utf8.ValidString(title) || utf8.RuneCountInString(title) < 1 || utf8.RuneCountInString(title) > 200 {
		return Title{}, ErrInvalidTitle
	}
	return Title{value: title}, nil
}

// String は、正規化済みのタイトルを返します。
func (t Title) String() string { return t.value }
