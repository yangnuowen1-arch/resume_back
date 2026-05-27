package dto

import "time"

type CreateUserRequest struct {
	Username string   `json:"username" binding:"required,min=3,max=50"`
	Password string   `json:"password" binding:"required,min=6,max=72"`
	Email    *string  `json:"email" binding:"omitempty,email,max=100"`
	Phone    *string  `json:"phone" binding:"omitempty,max=30"`
	RealName *string  `json:"realName" binding:"omitempty,max=50"`
	Status   string   `json:"status"`
	Roles    []string `json:"roles"`
}

type UpdateUserRequest struct {
	Username string  `json:"username" binding:"required,min=3,max=50"`
	Email    *string `json:"email" binding:"omitempty,email,max=100"`
	Phone    *string `json:"phone" binding:"omitempty,max=30"`
	RealName *string `json:"realName" binding:"omitempty,max=50"`
	Status   string  `json:"status" binding:"required"`
}

type AssignUserRolesRequest struct {
	Roles []string `json:"roles" binding:"required"`
}

type UserQuery struct {
	Page     int
	PageSize int
	Keyword  string
	Status   string
}

type ManagedUserResponse struct {
	ID          int64      `json:"id"`
	Username    string     `json:"username"`
	Email       *string    `json:"email"`
	Phone       *string    `json:"phone"`
	RealName    *string    `json:"realName"`
	AvatarURL   *string    `json:"avatarUrl"`
	Status      string     `json:"status"`
	Roles       []string   `json:"roles"`
	LastLoginAt *time.Time `json:"lastLoginAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type RoleResponse struct {
	ID          int64     `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}
