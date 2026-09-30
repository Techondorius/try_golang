// Package domain は、集約Repositoryに共通する保存結果のエラーを定義します。
// accountとtaskの保存契約を内側の層で共有し、applicationや具体的なDBに依存させません。
// セッション保存も同じエラーを使います。認証失敗そのものはapplication層が判断します。
package domain

import "errors"

var (
	// ErrNotFound は、指定した対象が存在しないことを表します。
	// Taskでは所有者不一致も同じエラーにし、他人の集約の存在を漏らしません。
	// 呼び出し側は、ラップされたエラーもerrors.Isで判定できます。
	ErrNotFound = errors.New("resource not found")
	// ErrConflict は、メール・ID・検索キーの重複など、保存時の一意性違反を表します。
	// Repositoryの実装は既存データを上書きせず、このエラーを返します。
	ErrConflict = errors.New("resource already exists")
)
