// Package task は、タスクの集約・タイトルの値オブジェクト・状態遷移のルールを定義します。
//
// 「完了後のタスクは改名できない」といったルールをここに集めることで、
// HTTP以外から操作する場合にも同じ制約を適用できます。
// 誰が操作できるかという所有者の確認はapplication層が担当します。
package task

import (
	"errors"
	"time"
)

var (
	// ErrInvalidTitle は、タイトルが正規化後に1〜200 Unicodeコードポイントに収まらない場合などのエラーです。
	ErrInvalidTitle = errors.New("title must contain 1 to 200 characters")
	// ErrInvalidIdentity は、タスクIDまたは所有者IDが空であることを表します。
	ErrInvalidIdentity = errors.New("task and owner IDs are required")
	// ErrCompleted は、完了したタスクを改名または再完了しようとしたことを表します。
	ErrCompleted = errors.New("completed task cannot be changed")
)

// Status は、タスクの進行状態を表す型です。
// Taskの状態変更はCompleteを通して行い、外側から状態を直接書き換えるAPIは提供しません。
type Status string

const (
	// Pending は、作成直後の未完了状態です。この状態では改名と完了ができます。
	Pending Status = "pending"
	// Completed は、完了状態です。改名・再完了はできませんが、ユースケースから削除できます。
	Completed Status = "completed"
)

// Task は、タイトル・所有者・状態・日時をまとめて管理する集約のルートです。
//
// IDによって同一のタスクかどうかを識別します。Titleはその内部の値オブジェクトです。
// User集約を直接保持せず所有者IDで参照することで、集約の境界を分けています。
// フィールドを非公開にし、RenameとCompleteに状態変更を集めることで、
// 業務ルールを通らない変更を防ぎます。通常はゼロ値ではなくNewで生成します。
type Task struct {
	id        string    // タスクを識別するIDです。
	ownerID   string    // 所有者のUser ID。作成後に変更する操作はありません。
	title     Title     // 正規化と検証を済ませたタイトルです。
	status    Status    // 集約の操作を通して更新する進行状態です。
	createdAt time.Time // 作成日時です。改名や完了では変更しません。
	updatedAt time.Time // 最後に改名または完了した日時。作成時はcreatedAtと同じです。
}

// New は、所有者を持つ未完了のTaskを生成します。
//
// 空のid・ownerIDにはErrInvalidIdentity、不正なタイトルにはErrInvalidTitleを返します。
// 所有者が存在するかの検索や保存は行いません。application層が認証済みの所有者IDを渡します。
// 作成日時と更新日時には同じnowを設定し、実時間への依存を呼び出し側に任せます。
func New(id, ownerID, rawTitle string, now time.Time) (Task, error) {
	if id == "" || ownerID == "" {
		return Task{}, ErrInvalidIdentity
	}
	title, err := NewTitle(rawTitle)
	if err != nil {
		return Task{}, err
	}
	return Task{id: id, ownerID: ownerID, title: title, status: Pending, createdAt: now, updatedAt: now}, nil
}

// Rename は、未完了のタスクのタイトルと更新日時を変更します。
//
// 完了済みならErrCompleted、不正なタイトルならErrInvalidTitleを返し、状態を変更しません。
// タイトルの検証に成功してからフィールドへ代入する順序が、失敗時の状態を守ります。
// ポインターレシーバーはTask自身を変更するために使います。保存は呼び出し側の責務です。
func (t *Task) Rename(raw string, now time.Time) error {
	if t.status == Completed {
		return ErrCompleted
	}
	title, err := NewTitle(raw)
	if err != nil {
		return err
	}
	t.title, t.updatedAt = title, now
	return nil
}

// Complete は、未完了のタスクをCompletedへ遷移させ、更新日時を設定します。
// 既に完了していればErrCompletedを返します。このサンプルでは再完了は競合として扱い、
// 未完了に戻す操作も提供していません。所有者の確認と変更後の保存はapplication層が行います。
func (t *Task) Complete(now time.Time) error {
	if t.status == Completed {
		return ErrCompleted
	}
	t.status, t.updatedAt = Completed, now
	return nil
}

// ID は、タイトルが変わっても同じタスクを識別できるIDを返します。
func (t Task) ID() string { return t.id }

// OwnerID は、application層が操作権限を判定するための所有者IDを返します。
func (t Task) OwnerID() string { return t.ownerID }

// Title は、HTTP応答などで使える正規化済みのタイトル文字列を返します。
func (t Task) Title() string { return t.title.String() }

// Status は、現在の進行状態を返します。返した値を変更してもTaskの状態は変わりません。
func (t Task) Status() Status { return t.status }

// CreatedAt は、タスクを生成した日時を返します。
func (t Task) CreatedAt() time.Time { return t.createdAt }

// UpdatedAt は、最後に集約を変更した日時を返します。
func (t Task) UpdatedAt() time.Time { return t.updatedAt }
