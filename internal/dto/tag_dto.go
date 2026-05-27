package dto

import "time"

type CreateTagGroupRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
	Status      string  `json:"status"`
}

type UpdateTagGroupRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
	Status      string  `json:"status" binding:"required"`
}

type TagGroupQuery struct {
	Page     int
	PageSize int
	Keyword  string
	Status   string
}

type TagGroupResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type CreateTagRequest struct {
	GroupID *int64  `json:"groupId"`
	Name    string  `json:"name" binding:"required"`
	Color   *string `json:"color"`
	Status  string  `json:"status"`
}

type UpdateTagRequest struct {
	GroupID *int64  `json:"groupId"`
	Name    string  `json:"name" binding:"required"`
	Color   *string `json:"color"`
	Status  string  `json:"status" binding:"required"`
}

type TagQuery struct {
	Page     int
	PageSize int
	Keyword  string
	GroupID  *int64
	Status   string
}

type TagResponse struct {
	ID        int64     `json:"id"`
	GroupID   *int64    `json:"groupId"`
	Name      string    `json:"name"`
	Color     *string   `json:"color"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type TagGroupWithTagsResponse struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Description *string       `json:"description"`
	Status      string        `json:"status"`
	Tags        []TagResponse `json:"tags"`
}
