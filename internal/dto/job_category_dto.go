package dto

import "time"

type CreateJobCategoryRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
	ParentID    *int64  `json:"parentId"`
	SortOrder   int32   `json:"sortOrder"`
}

type JobCategoryQuery struct {
	Page     int
	PageSize int
	Keyword  string
	Status   string
}

type JobCategoryResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	ParentID    *int64    `json:"parentId"`
	SortOrder   int32     `json:"sortOrder"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
