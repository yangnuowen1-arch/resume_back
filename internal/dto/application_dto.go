package dto

import "time"

type CreateApplicationRequest struct {
	JobID       int64   `json:"jobId" binding:"required"`
	CandidateID *int64  `json:"candidateId"`
	ResumeID    int64   `json:"resumeId" binding:"required"`
	Source      *string `json:"source"`
	Status      string  `json:"status"`
	Remark      *string `json:"remark"`
}

type ApplicationResponse struct {
	ID          int64     `json:"id"`
	JobID       int64     `json:"jobId"`
	CandidateID *int64    `json:"candidateId"`
	ResumeID    int64     `json:"resumeId"`
	Source      *string   `json:"source"`
	Status      string    `json:"status"`
	ReceivedAt  time.Time `json:"receivedAt"`
	Remark      *string   `json:"remark"`
	CreatedBy   *int64    `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
