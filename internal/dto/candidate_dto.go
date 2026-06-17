package dto

import "time"

type CreateCandidateRequest struct {
	Name                    string   `json:"name" binding:"required"`
	Email                   *string  `json:"email"`
	Phone                   *string  `json:"phone"`
	Gender                  *string  `json:"gender"`
	CurrentCompany          *string  `json:"currentCompany"`
	PositionCategoryID      *int64   `json:"positionCategoryId"`
	CurrentJobID            *int64   `json:"currentJobId"`
	CurrentPosition         *string  `json:"currentPosition"`
	CurrentPositionCategory *string  `json:"currentPositionCategory"`
	YearsOfExperience       *float64 `json:"yearsOfExperience"`
	HighestEducation        *string  `json:"highestEducation"`
	School                  *string  `json:"school"`
	Major                   *string  `json:"major"`
	Location                *string  `json:"location"`
	Source                  *string  `json:"source"`
	Status                  string   `json:"status"`
}

type UpdateCandidateRequest struct {
	Name                    string   `json:"name" binding:"required"`
	Email                   *string  `json:"email"`
	Phone                   *string  `json:"phone"`
	Gender                  *string  `json:"gender"`
	CurrentCompany          *string  `json:"currentCompany"`
	PositionCategoryID      *int64   `json:"positionCategoryId"`
	CurrentJobID            *int64   `json:"currentJobId"`
	CurrentPosition         *string  `json:"currentPosition"`
	CurrentPositionCategory *string  `json:"currentPositionCategory"`
	YearsOfExperience       *float64 `json:"yearsOfExperience"`
	HighestEducation        *string  `json:"highestEducation"`
	School                  *string  `json:"school"`
	Major                   *string  `json:"major"`
	Location                *string  `json:"location"`
	Source                  *string  `json:"source"`
	Status                  string   `json:"status" binding:"required"`
}

type CandidateQuery struct {
	Page     int
	PageSize int
	Keyword  string
	Source   string
	Status   string
}

type CandidateResponse struct {
	ID                      int64      `json:"id"`
	Name                    *string    `json:"name"`
	Email                   *string    `json:"email"`
	Phone                   *string    `json:"phone"`
	Gender                  *string    `json:"gender"`
	CurrentCompany          *string    `json:"currentCompany"`
	PositionCategoryID      *int64     `json:"positionCategoryId"`
	PositionCategoryName    *string    `json:"positionCategoryName"`
	CurrentJobID            *int64     `json:"currentJobId"`
	CurrentPosition         *string    `json:"currentPosition"`
	CurrentPositionCategory *string    `json:"currentPositionCategory"`
	YearsOfExperience       *float64   `json:"yearsOfExperience"`
	HighestEducation        *string    `json:"highestEducation"`
	School                  *string    `json:"school"`
	Major                   *string    `json:"major"`
	Location                *string    `json:"location"`
	Source                  *string    `json:"source"`
	Status                  string     `json:"status"`
	Position                *string    `json:"position"`
	ResumeID                *int64     `json:"resumeId"`
	ResumeFilename          *string    `json:"resumeFilename"`
	ResumeFileURL           *string    `json:"resumeFileUrl"`
	ResumeParseStatus       *string    `json:"resumeParseStatus"`
	ResumeParseError        *string    `json:"resumeParseError,omitempty"`
	ResumeLanguage          *string    `json:"resumeLanguage"`
	Language                *string    `json:"language"`
	ResumeUploadedAt        *time.Time `json:"resumeUploadedAt"`
	ResumeEvaluated         bool       `json:"resumeEvaluated"`
	ScreeningStatus         *string    `json:"screeningStatus"`
	AIScore                 *float64   `json:"aiScore"`
	ApplicationID           *int64     `json:"applicationId"`
	JobID                   *int64     `json:"jobId"`
	JobTitle                *string    `json:"jobTitle"`
	CreatedAt               time.Time  `json:"createdAt"`
	UpdatedAt               time.Time  `json:"updatedAt"`
}

type CandidateStatusOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type BatchAnalyzeCandidatesRequest struct {
	CandidateIDs []int64 `json:"candidateIds" binding:"required"`
	JobID        *int64  `json:"jobId"`
}

type BatchAnalyzeCandidateResult struct {
	CandidateID       int64   `json:"candidateId"`
	ResumeID          *int64  `json:"resumeId"`
	ApplicationID     *int64  `json:"applicationId"`
	ScreeningResultID *int64  `json:"screeningResultId,omitempty"`
	JobID             *int64  `json:"jobId,omitempty"`
	ParseStatus       *string `json:"parseStatus,omitempty"`
	Status            string  `json:"status"`
	Message           *string `json:"message,omitempty"`
}

type BatchAnalyzeCandidatesResponse struct {
	Total  int                           `json:"total"`
	Queued int                           `json:"queued"`
	Failed int                           `json:"failed"`
	Items  []BatchAnalyzeCandidateResult `json:"items"`
}
