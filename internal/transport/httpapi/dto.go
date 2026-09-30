package httpapi

import (
	"time"

	"try_golang/internal/domain/account"
	"try_golang/internal/domain/task"
)

// credentials は、登録・ログインのJSON入力に対応するDTO（Data Transfer Object）です。
//
// DTOは通信するデータの形であり、集約のように業務ルールを管理する型ではありません。
// jsonタグはHTTP上の項目名を指定します。読み取り後の形式・長さの検証はユースケースへ渡します。
// 平文のPasswordを含むため、応答やログへこのDTOをそのまま出力しません。
type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// taskInput は、タスク作成・改名でクライアントに指定を許可する項目だけを持つDTOです。
// owner_idやstatusを持たせず、所有者は認証結果、状態はドメインの操作から決定します。
type taskInput struct {
	Title string `json:"title"`
}

// userResponse は、UserからHTTP応答に公開する項目だけを取り出したDTOです。
// 集約をそのままJSON化せず、PasswordHashが応答へ混ざらない構造にしています。
type userResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// userDTO は、User集約を外部公開用のuserResponseへ変換します。
// DTOの構築を明示することで、公開する情報の選択をHTTP層に集めています。
func userDTO(user account.User) userResponse {
	return userResponse{user.ID(), user.Email().String(), user.CreatedAt()}
}

// taskResponse は、Taskの状態をクライアントへ伝えるためのDTOです。
// ドメインの非公開フィールドを変更する入口は作らず、読み取り用メソッドから値を受け取ります。
type taskResponse struct {
	ID        string      `json:"id"`
	Title     string      `json:"title"`
	Status    task.Status `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// taskDTO は、Task集約をHTTP上の項目名と形式を持つ応答DTOへ変換します。
// JSONの項目を変更しても、Taskの業務ルールや保存ポートへ影響を広げずに済みます。
func taskDTO(item task.Task) taskResponse {
	return taskResponse{item.ID(), item.Title(), item.Status(), item.CreatedAt(), item.UpdatedAt()}
}
