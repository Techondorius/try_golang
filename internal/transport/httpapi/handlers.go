package httpapi

import "github.com/gin-gonic/gin"

func (h handler) register(c *gin.Context) {
	var input credentials
	if !decode(c, &input) {
		return
	}
	user, err := h.auth.Register(c.Request.Context(), input.Email, input.Password)
	if err != nil {
		fail(c, err, h.logger)
		return
	}
	c.JSON(201, userDTO(user))
}
func (h handler) login(c *gin.Context) {
	var input credentials
	if !decode(c, &input) {
		return
	}
	token, err := h.auth.Login(c.Request.Context(), input.Email, input.Password)
	if err != nil {
		fail(c, err, h.logger)
		return
	}
	c.JSON(200, gin.H{"access_token": token.AccessToken, "token_type": "Bearer", "expires_at": token.ExpiresAt})
}
func (h handler) logout(c *gin.Context) {
	if err := h.auth.Logout(c.Request.Context(), bearer(c)); err != nil {
		fail(c, err, h.logger)
		return
	}
	c.Status(204)
}
func (h handler) create(c *gin.Context) {
	var input taskInput
	if !decode(c, &input) {
		return
	}
	item, err := h.tasks.Create(c.Request.Context(), currentUser(c).ID(), input.Title)
	if err != nil {
		fail(c, err, h.logger)
		return
	}
	c.Header("Location", "/v1/tasks/"+item.ID())
	c.JSON(201, taskDTO(item))
}
func (h handler) list(c *gin.Context) {
	items, err := h.tasks.List(c.Request.Context(), currentUser(c).ID())
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
func (h handler) get(c *gin.Context) {
	item, err := h.tasks.Get(c.Request.Context(), currentUser(c).ID(), c.Param("id"))
	if err != nil {
		fail(c, err, h.logger)
		return
	}
	c.JSON(200, taskDTO(item))
}
func (h handler) rename(c *gin.Context) {
	var input taskInput
	if !decode(c, &input) {
		return
	}
	item, err := h.tasks.Rename(c.Request.Context(), currentUser(c).ID(), c.Param("id"), input.Title)
	if err != nil {
		fail(c, err, h.logger)
		return
	}
	c.JSON(200, taskDTO(item))
}
func (h handler) complete(c *gin.Context) {
	item, err := h.tasks.Complete(c.Request.Context(), currentUser(c).ID(), c.Param("id"))
	if err != nil {
		fail(c, err, h.logger)
		return
	}
	c.JSON(200, taskDTO(item))
}
func (h handler) delete(c *gin.Context) {
	if err := h.tasks.Delete(c.Request.Context(), currentUser(c).ID(), c.Param("id")); err != nil {
		fail(c, err, h.logger)
		return
	}
	c.Status(204)
}
