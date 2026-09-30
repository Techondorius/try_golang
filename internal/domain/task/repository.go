package task

import "context"

// ITasksRepository は、Task集約を単位として保存・取得するRepositoryのポートです。
//
// Repositoryは、集約を取り出して操作し、再び保存するための窓口です。
// Task集約の保存契約をこのドメインに置き、applicationはそれを使い、infrastructureが実装します。
// ドメインはGin・SQL・具体的な保存実装を知る必要がありません。
// 個々のカラムを直接操作するAPIにせず、Taskのルールを通して変更する形にしています。
// 取得した値の変更だけでは保存済み状態は変わらず、UpdateTaskを通した成功時だけ保存します。
type ITasksRepository interface {
	// InsertTask は新規Taskを保存します。同じIDが存在する場合はdomain.ErrConflictです。
	InsertTask(ctx context.Context, item Task) error
	// FindTask はIDでTaskのスナップショットを取得します。未存在ならdomain.ErrNotFoundです。
	// 所有者の認可はこの取得操作では行わず、呼び出し側のユースケースが確認します。
	FindTask(ctx context.Context, id string) (Task, error)
	// ListTasks は指定所有者のTaskだけを、作成日時・IDの順で返します。
	// 対象がなければ空の一覧を返します。
	ListTasks(ctx context.Context, ownerID string) ([]Task, error)
	// UpdateTask は取得・changeの実行・保存を原子的に行います。
	// changeは所有者の確認やドメイン操作を行い、エラーなら変更を保存しません。
	// 同時更新で古い状態を上書きしないことが契約です。SQL実装ではトランザクションや
	// バージョン番号などで、このメモリ実装と同じ性質を保証する必要があります。
	UpdateTask(ctx context.Context, id string, change func(*Task) error) (Task, error)
	// DeleteTask は所有者の確認と削除を原子的に行います。
	// 未存在と所有者不一致はどちらもdomain.ErrNotFoundにし、他人のTaskの存在を漏らしません。
	DeleteTask(ctx context.Context, id, ownerID string) error
}
