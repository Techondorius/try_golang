package application

import (
	"context"
	"fmt"

	"try_golang/internal/domain"
	"try_golang/internal/domain/task"
)

// Tasks は、認証済みの利用者によるタスク操作を調整するユースケースです。
//
// ownerには、入口の認証処理で確認したUserのIDを渡してください。
// この型はトークン照合を行わず、所有者かどうかの認可とドメイン操作・保存を担当します。
// ownerが空ならErrUnauthenticated、他人のTaskにはdomain.ErrNotFoundを返します。
type Tasks struct {
	repo     task.ITasksRepository // 集約の取得と原子的な変更を提供するポートです。
	generate Generator             // 新規TaskのIDを生成します。
	now      Clock                 // 集約の作成・更新日時を供給します。
}

// NewTasksUseCase は、Repository・ID生成・時刻取得の実装を注入してTasksを組み立てます。
// テストではFakeを渡し、本番ではmain.goが具体的なアダプターを選びます。
func NewTasksUseCase(repo task.ITasksRepository, generate Generator, now Clock) *Tasks {
	return &Tasks{repo, generate, now}
}

// CreateUseCase は、認証済み所有者の未完了Taskをドメインで生成し、Repositoryに保存します。
// タイトルのルールはtask.Newに任せ、ID生成や保存に失敗した場合はエラーを返します。
// 返した値は保存成功後の集約です。ownerのUserが存在するかはここでは再検索しません。
func (s *Tasks) CreateUseCase(ctx context.Context, owner, title string) (task.Task, error) {
	if owner == "" {
		return task.Task{}, ErrUnauthenticated
	}
	id, err := s.generate()
	if err != nil {
		return task.Task{}, fmt.Errorf("generate task ID: %w", err)
	}
	item, err := task.New(id, owner, title, s.now())
	if err != nil {
		return task.Task{}, err
	}
	if err = s.repo.InsertTask(ctx, item); err != nil {
		return task.Task{}, fmt.Errorf("create task: %w", err)
	}
	return item, nil
}

// GetUseCase は、対象Taskを取得し、要求者が所有者であることを確認して返します。
// 取得と認可をユースケースにまとめることで、HTTP以外の入口にも同じ制約を適用できます。
// 対象の未存在と所有者不一致は、どちらもdomain.ErrNotFoundとして扱います。
func (s *Tasks) GetUseCase(ctx context.Context, owner, id string) (task.Task, error) {
	if owner == "" {
		return task.Task{}, ErrUnauthenticated
	}
	item, err := s.repo.FindTask(ctx, id)
	if err != nil {
		return task.Task{}, err
	}
	if item.OwnerID() != owner {
		return task.Task{}, domain.ErrNotFound
	}
	return item, nil
}

// ListUseCase は、要求者が所有するTaskだけを取得します。
// 所有者で絞り込む契約をRepositoryへ渡し、他ユーザーの集約が一覧に混ざることを防ぎます。
func (s *Tasks) ListUseCase(ctx context.Context, owner string) ([]task.Task, error) {
	if owner == "" {
		return nil, ErrUnauthenticated
	}
	return s.repo.ListTasks(ctx, owner)
}

// RenameUseCase は、所有者を確認してTask.Renameを実行し、改名結果を保存します。
// タイトル不正や完了済みの判定はドメインに任せ、失敗時には保存済み状態を変えません。
func (s *Tasks) RenameUseCase(ctx context.Context, owner, id, title string) (task.Task, error) {
	return s.change(ctx, owner, id, func(item *task.Task) error { return item.Rename(title, s.now()) })
}

// CompleteUseCase は、所有者を確認してTask.Completeを実行し、完了結果を保存します。
// 完了条件はドメイン、同時更新での整合性はRepositoryの原子的な更新が担当します。
func (s *Tasks) CompleteUseCase(ctx context.Context, owner, id string) (task.Task, error) {
	return s.change(ctx, owner, id, func(item *task.Task) error { return item.Complete(s.now()) })
}

// change は、改名と完了に共通する「取得・認可・ドメイン操作・保存」を一か所に集めます。
// 認可もUpdateTaskのコールバック内で行い、確認した状態と保存する状態を同じ更新単位にします。
// operationがエラーを返せば、Repositoryは変更を保存しない契約です。
func (s *Tasks) change(ctx context.Context, owner, id string, operation func(*task.Task) error) (task.Task, error) {
	if owner == "" {
		return task.Task{}, ErrUnauthenticated
	}
	return s.repo.UpdateTask(ctx, id, func(item *task.Task) error {
		if item.OwnerID() != owner {
			return domain.ErrNotFound
		}
		return operation(item)
	})
}

// DeleteUseCase は、所有者であることを確認してTaskを削除します。完了済みのTaskも削除できます。
// 所有者の確認と削除を一体で行うため、RepositoryのDeleteTaskへ依頼します。
// 未存在と所有者不一致は、どちらもdomain.ErrNotFoundです。
func (s *Tasks) DeleteUseCase(ctx context.Context, owner, id string) error {
	if owner == "" {
		return ErrUnauthenticated
	}
	return s.repo.DeleteTask(ctx, id, owner)
}
