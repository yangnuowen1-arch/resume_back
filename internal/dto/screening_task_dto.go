package dto

import "time"

type ScreeningTaskQuery struct {
	Page        int
	PageSize    int
	Keyword     string
	Status      string
	JobID       *int64
	CandidateID *int64
}

type RunResumeScreeningRequest struct {
	ResumeID       int64  `json:"resumeId" binding:"required"`
	JobID          int64  `json:"jobId" binding:"required"`
	OutputLanguage string `json:"outputLanguage"`
}

type RunResumeScreeningResponse struct {
	ScreeningResultID int64    `json:"screeningResultId"`
	ApplicationID     int64    `json:"applicationId"`
	ResumeID          int64    `json:"resumeId"`
	JobID             int64    `json:"jobId"`
	Score             *float64 `json:"score"`
	MatchLevel        *string  `json:"matchLevel,omitempty"`
	Recommendation    *string  `json:"recommendation,omitempty"`
	Summary           *string  `json:"summary,omitempty"`
	MarkdownReport    *string  `json:"markdownReport,omitempty"`
	Status            string   `json:"status"`
}

type ScreeningTaskResponse struct {
	ID             int64     `json:"id"`
	ApplicationID  int64     `json:"applicationId"`
	CandidateID    *int64    `json:"candidateId"`
	Candidate      *string   `json:"candidate"`
	CandidateName  *string   `json:"candidateName"`
	JobID          int64     `json:"jobId"`
	JobTitle       string    `json:"jobTitle"`
	Position       string    `json:"position"`
	AIScore        *float64  `json:"aiScore"`
	Status         string    `json:"status"`
	Date           time.Time `json:"date"`
	CreatedAt      time.Time `json:"createdAt"`
	CreatedBy      *int64    `json:"createdBy"`
	MatchLevel     *string   `json:"matchLevel,omitempty"`
	Recommendation *string   `json:"recommendation,omitempty"`
	ErrorMessage   *string   `json:"errorMessage,omitempty"`
}
