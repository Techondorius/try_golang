package httpapi

import "github.com/gin-gonic/gin"

// register は、登録DTOを読み、Auth.RegisterUseCaseを呼んで201と公開用User DTOを返します。
// HTTPの読み取り・応答はここで行い、メール検証・ハッシュ化・保存の順序はユースケースに任せます。
func (h handler) register(c *gin.Context) {
	var input credentials
	if !decode(c, &input) {
		return
	}
	user, err := h.auth.RegisterUseCase(c.Request.Context(), input.Email, input.Password)
	if err != nil {
		fail(c, err, h.logger)
		return
	}
	c.JSON(201, userDTO(user))
}

// login は、資格情報をAuth.Loginへ渡し、認証トークンと有効期限をJSONで返します。
// セッションの保存用ハッシュは応答に含めず、利用者が提示する平文トークンを一度返します。
func (h handler) login(c *gin.Context) {
	var input credentials
	if !decode(c, &input) {
		return
	}
	token, err := h.auth.LoginUseCase(c.Request.Context(), input.Email, input.Password)
	if err != nil {
		fail(c, err, h.logger)
		return
	}
	c.JSON(200, gin.H{"access_token": token.AccessToken, "token_type": "Bearer", "expires_at": token.ExpiresAt})
}

// logout は、認証済みリクエストのトークンを失効させ、本文なしの204を返します。
// 先にauthenticateを通るルートなので、不正なトークンを成功扱いにする入口にはなりません。
func (h handler) logout(c *gin.Context) {
	if err := h.auth.LogoutUseCase(c.Request.Context(), bearer(c)); err != nil {
		fail(c, err, h.logger)
		return
	}
	c.Status(204)
}

// create は、入力されたタイトルと認証済みUserのIDでTaskを作成します。
// 所有者を入力DTOから取得しないことで、他人のIDを指定した作成を防ぎます。
// 作成結果は201で返し、Locationヘッダーに取得先のURLを示します。
func (h handler) create(c *gin.Context) {
	var input taskInput
	if !decode(c, &input) {
		return
	}
	item, err := h.tasks.CreateUseCase(c.Request.Context(), currentUser(c).ID(), input.Title)
	if err != nil {
		fail(c, err, h.logger)
		return
	}
	c.Header("Location", "/v1/tasks/"+item.ID())
	c.JSON(201, taskDTO(item))
}

// list は、認証済みUserのTask一覧をDTOへ変換して返します。
// 空のスライスを用意してから詰めるため、対象がなくてもJSONのtasksはnullではなく[]になります。
func (h handler) list(c *gin.Context) {
	items, err := h.tasks.ListUseCase(c.Request.Context(), currentUser(c).ID())
	if err != nil {
		fail(c, err, h.logger)
		return
	}
	out := make([]taskResponse, 0, len(items))
	for _, item := range items {
		out = append(out, taskDTO(item))
	}
	c.JSON(200, gin.H{"tasks": out})
}

// get は、URLのIDと認証済みUserのIDをTasks.Getへ渡し、取得結果をDTOとして返します。
// 所有者の認可はユースケースが担当し、ハンドラーはエラーをHTTPへ変換します。
func (h handler) get(c *gin.Context) {
	item, err := h.tasks.GetUseCase(c.Request.Context(), currentUser(c).ID(), c.Param("id"))
	if err != nil {
		fail(c, err, h.logger)
		return
	}
	c.JSON(200, taskDTO(item))
}

// rename は、タイトルだけを入力DTOで受け取り、Tasks.RenameUseCaseの結果を返します。
// 完了後の変更禁止はハンドラーに重複実装せず、Task集約のルールを利用します。
func (h handler) rename(c *gin.Context) {
	var input taskInput
	if !decode(c, &input) {
		return
	}
	item, err := h.tasks.RenameUseCase(c.Request.Context(), currentUser(c).ID(), c.Param("id"), input.Title)
	if err != nil {
		fail(c, err, h.logger)
		return
	}
	c.JSON(200, taskDTO(item))
}

// complete は、Tasks.CompleteUseCaseへ完了という操作を依頼します。
// 状態文字列を任意に更新するAPIにせず、ドメインが定める遷移を通して結果を返します。
func (h handler) complete(c *gin.Context) {
	item, err := h.tasks.CompleteUseCase(c.Request.Context(), currentUser(c).ID(), c.Param("id"))
	if err != nil {
		fail(c, err, h.logger)
		return
	}
	c.JSON(200, taskDTO(item))
}

// delete は、所有者のIDと対象IDをTasks.Deleteへ渡し、成功時に204を返します。
// 他人のTaskや未存在のTaskへの要求は、ユースケースのErrNotFoundを404へ変換します。
func (h handler) delete(c *gin.Context) {
	if err := h.tasks.DeleteUseCase(c.Request.Context(), currentUser(c).ID(), c.Param("id")); err != nil {
		fail(c, err, h.logger)
		return
	}
	c.Status(204)
}
