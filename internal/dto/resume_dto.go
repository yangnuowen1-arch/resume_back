package dto

import "time"

type UploadResumeRequest struct {
	CandidateID      *int64
	OriginalFilename string
	FileURL          string
	FileType         string
	FileSize         int64
	RawText          *string
	Language         *string
}

type ResumeResponse struct {
	ID               int64     `json:"id"`
	CandidateID      *int64    `json:"candidateId"`
	OriginalFilename *string   `json:"originalFilename"`
	FileURL          *string   `json:"fileUrl"`
	FileType         *string   `json:"fileType"`
	FileSize         *int64    `json:"fileSize"`
	RawText          *string   `json:"rawText"`
	Language         *string   `json:"language"`
	UploadBy         *int64    `json:"uploadBy"`
	UploadedAt       time.Time `json:"uploadedAt"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type UploadResumeResponse struct {
	Code      int            `json:"code"`
	Message   string         `json:"message"`
	Data      ResumeResponse `json:"data"`
	RequestID string         `json:"requestId"`
	Timestamp string         `json:"timestamp"`
}
