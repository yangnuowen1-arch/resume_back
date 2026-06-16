package dto

import "time"

type UploadResumeRequest struct {
	CandidateID      *int64
	OriginalFilename string
	FileKey          string
	FileURL          string
	FileType         string
	FileSize         int64
	RawText          *string
	Language         *string
}

type ResumeResponse struct {
	ID               int64      `json:"id"`
	CandidateID      *int64     `json:"candidateId"`
	CandidateName    *string    `json:"candidateName,omitempty"`
	OriginalFilename *string    `json:"originalFilename"`
	FileKey          *string    `json:"fileKey,omitempty"`
	FileURL          *string    `json:"fileUrl"`
	FileType         *string    `json:"fileType"`
	FileSize         *int64     `json:"fileSize"`
	RawText          *string    `json:"rawText"`
	ParsedData       *string    `json:"parsedData,omitempty"`
	ParseStatus      string     `json:"parseStatus"`
	ParseError       *string    `json:"parseError,omitempty"`
	ParsedAt         *time.Time `json:"parsedAt,omitempty"`
	Language         *string    `json:"language"`
	UploadBy         *int64     `json:"uploadBy"`
	UploadedAt       time.Time  `json:"uploadedAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type ResumeQuery struct {
	Page        int
	PageSize    int
	Keyword     string
	CandidateID *int64
	Language    string
}

type UploadResumeResponse struct {
	Code      int            `json:"code"`
	Message   string         `json:"message"`
	Data      ResumeResponse `json:"data"`
	RequestID string         `json:"requestId"`
	Timestamp string         `json:"timestamp"`
}
