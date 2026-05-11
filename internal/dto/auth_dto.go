package dto

import "time"

// 接收前端的
type RegisterRequest struct {
	Username string  `json:"username" binding:"required,min=3,max=50"`
	Password string  `json:"password" binding:"required,min=6,max=72"`
	Email    *string `json:"email" binding:"omitempty,email,max=100"`
	Phone    *string `json:"phone" binding:"omitempty,max=30"`
	RealName *string `json:"realName" binding:"omitempty,max=50"`
}

type LoginRequest struct {
	Account  string `json:"account" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// 后端返回出去的
type UserResponse struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     *string   `json:"email"`
	Phone     *string   `json:"phone"`
	RealName  *string   `json:"realName"`
	AvatarURL *string   `json:"avatarUrl"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

type AuthResponse struct {
	Token     string       `json:"token"`
	TokenType string       `json:"tokenType"`
	User      UserResponse `json:"user"`
	Roles     []string     `json:"roles"`
}
