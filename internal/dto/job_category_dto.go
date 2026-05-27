package dto

import "time"

type CreateJobCategoryRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
	Status      string  `json:"status"`
}

type UpdateJobCategoryRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
	Status      string  `json:"status" binding:"required"`
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
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
