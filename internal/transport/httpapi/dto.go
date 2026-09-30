package httpapi

import (
	"time"

	"try_golang/internal/domain/account"
	"try_golang/internal/domain/task"
)

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type taskInput struct {
	Title string `json:"title"`
}
type userResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func userDTO(user account.User) userResponse {
	return userResponse{user.ID(), user.Email().String(), user.CreatedAt()}
}

type taskResponse struct {
	ID        string      `json:"id"`
	Title     string      `json:"title"`
	Status    task.Status `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

func taskDTO(item task.Task) taskResponse {
	return taskResponse{item.ID(), item.Title(), item.Status(), item.CreatedAt(), item.UpdatedAt()}
}
