package dto

import "time"

type CreateCandidateRequest struct {
	Name              string   `json:"name" binding:"required"`
	Email             *string  `json:"email"`
	Phone             *string  `json:"phone"`
	Gender            *string  `json:"gender"`
	CurrentCompany    *string  `json:"currentCompany"`
	CurrentPosition   *string  `json:"currentPosition"`
	YearsOfExperience *float64 `json:"yearsOfExperience"`
	HighestEducation  *string  `json:"highestEducation"`
	School            *string  `json:"school"`
	Major             *string  `json:"major"`
	Location          *string  `json:"location"`
	Source            *string  `json:"source"`
}

type UpdateCandidateRequest struct {
	Name              string   `json:"name" binding:"required"`
	Email             *string  `json:"email"`
	Phone             *string  `json:"phone"`
	Gender            *string  `json:"gender"`
	CurrentCompany    *string  `json:"currentCompany"`
	CurrentPosition   *string  `json:"currentPosition"`
	YearsOfExperience *float64 `json:"yearsOfExperience"`
	HighestEducation  *string  `json:"highestEducation"`
	School            *string  `json:"school"`
	Major             *string  `json:"major"`
	Location          *string  `json:"location"`
	Source            *string  `json:"source"`
}

type CandidateQuery struct {
	Page     int
	PageSize int
	Keyword  string
	Source   string
}

type CandidateResponse struct {
	ID                int64     `json:"id"`
	Name              *string   `json:"name"`
	Email             *string   `json:"email"`
	Phone             *string   `json:"phone"`
	Gender            *string   `json:"gender"`
	CurrentCompany    *string   `json:"currentCompany"`
	CurrentPosition   *string   `json:"currentPosition"`
	YearsOfExperience *float64  `json:"yearsOfExperience"`
	HighestEducation  *string   `json:"highestEducation"`
	School            *string   `json:"school"`
	Major             *string   `json:"major"`
	Location          *string   `json:"location"`
	Source            *string   `json:"source"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}
