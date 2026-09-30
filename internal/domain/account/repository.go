package account

import "context"

// IUsersRepository は、User集約を保存・取得するためのポートです。
//
// Userという業務上の集約の保存契約なので、集約と同じドメインに定義します。
// applicationはこのinterfaceを使い、infrastructureが実装します。
// ドメインからユースケースや具体的なDBへの依存は発生しません。
// ユースケースに必要な操作を列挙し、SQLやmapといった実装の詳細を隠しています。
// 異なるUser間のメールの一意性は、単一集約だけでは判定できないため、このポートの契約にします。
type IUsersRepository interface {
	// InsertUser はUserを新規保存します。IDまたはメールの重複はdomain.ErrConflictを返します。
	// 重複の確認と保存は一体として行い、同時登録でも一件だけ成功させます。
	InsertUser(ctx context.Context, user User) error
	// UserByEmail は、正規化済みのEmailでUserを取得します。未登録ならdomain.ErrNotFoundです。
	UserByEmail(ctx context.Context, email Email) (User, error)
	// UserByID は、集約のIDでUserを取得します。未登録ならdomain.ErrNotFoundです。
	UserByID(ctx context.Context, id string) (User, error)
}
