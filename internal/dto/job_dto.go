package dto

import "time"

type CreateJobRequest struct {
	CategoryID       *int64  `json:"categoryId"`
	Title            string  `json:"title" binding:"required"`
	Department       *string `json:"department"`
	Headcount        int32   `json:"headcount"`
	WorkLocation     *string `json:"workLocation"`
	WorkType         *string `json:"workType"`
	EmploymentType   *string `json:"employmentType"`
	SalaryMin        *int32  `json:"salaryMin"`
	SalaryMax        *int32  `json:"salaryMax"`
	SalaryMonths     *int32  `json:"salaryMonths"`
	ExperienceMin    *int32  `json:"experienceMin"`
	ExperienceMax    *int32  `json:"experienceMax"`
	EducationLevel   *string `json:"educationLevel"`
	Description      *string `json:"description"`
	Responsibilities *string `json:"responsibilities"`
	Requirements     *string `json:"requirements"`
	BonusPoints      *string `json:"bonusPoints"`
	Status           string  `json:"status"`
	Priority         string  `json:"priority"`
	OwnerUserID      *int64  `json:"ownerUserId"`
}

type UpdateJobRequest struct {
	CategoryID       *int64  `json:"categoryId"`
	Title            string  `json:"title" binding:"required"`
	Department       *string `json:"department"`
	Headcount        int32   `json:"headcount"`
	WorkLocation     *string `json:"workLocation"`
	WorkType         *string `json:"workType"`
	EmploymentType   *string `json:"employmentType"`
	SalaryMin        *int32  `json:"salaryMin"`
	SalaryMax        *int32  `json:"salaryMax"`
	SalaryMonths     *int32  `json:"salaryMonths"`
	ExperienceMin    *int32  `json:"experienceMin"`
	ExperienceMax    *int32  `json:"experienceMax"`
	EducationLevel   *string `json:"educationLevel"`
	Description      *string `json:"description"`
	Responsibilities *string `json:"responsibilities"`
	Requirements     *string `json:"requirements"`
	BonusPoints      *string `json:"bonusPoints"`
	Status           string  `json:"status" binding:"required"`
	Priority         string  `json:"priority" binding:"required"`
	OwnerUserID      *int64  `json:"ownerUserId"`
}

type JobQuery struct {
	Page       int
	PageSize   int
	Keyword    string
	CategoryID *int64
	Status     string
}

type JobResponse struct {
	ID               int64      `json:"id"`
	CategoryID       *int64     `json:"categoryId"`
	Title            string     `json:"title"`
	Department       *string    `json:"department"`
	Headcount        int32      `json:"headcount"`
	WorkLocation     *string    `json:"workLocation"`
	WorkType         *string    `json:"workType"`
	EmploymentType   *string    `json:"employmentType"`
	SalaryMin        *int32     `json:"salaryMin"`
	SalaryMax        *int32     `json:"salaryMax"`
	SalaryMonths     *int32     `json:"salaryMonths"`
	ExperienceMin    *int32     `json:"experienceMin"`
	ExperienceMax    *int32     `json:"experienceMax"`
	EducationLevel   *string    `json:"educationLevel"`
	Description      *string    `json:"description"`
	Responsibilities *string    `json:"responsibilities"`
	Requirements     *string    `json:"requirements"`
	BonusPoints      *string    `json:"bonusPoints"`
	Status           string     `json:"status"`
	Priority         string     `json:"priority"`
	OwnerUserID      *int64     `json:"ownerUserId"`
	CreatedBy        *int64     `json:"createdBy"`
	PublishedAt      *time.Time `json:"publishedAt"`
	ClosedAt         *time.Time `json:"closedAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type BindJobTagsRequest struct {
	TagIDs []int64 `json:"tagIds" binding:"required"`
}

type AssignJobMemberRequest struct {
	UserID     int64  `json:"userId" binding:"required"`
	MemberRole string `json:"memberRole" binding:"required"`
}

type JobMemberResponse struct {
	ID         int64     `json:"id"`
	JobID      int64     `json:"jobId"`
	UserID     int64     `json:"userId"`
	MemberRole string    `json:"memberRole"`
	CreatedBy  *int64    `json:"createdBy"`
	CreatedAt  time.Time `json:"createdAt"`
}

type JobTagResponse struct {
	JobID     int64     `json:"jobId"`
	TagID     int64     `json:"tagId"`
	GroupID   *int64    `json:"groupId"`
	Name      string    `json:"name"`
	Color     *string   `json:"color"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

type JobMemberDetailResponse struct {
	ID         int64     `json:"id"`
	JobID      int64     `json:"jobId"`
	UserID     int64     `json:"userId"`
	Username   string    `json:"username"`
	RealName   *string   `json:"realName"`
	Email      *string   `json:"email"`
	UserStatus string    `json:"userStatus"`
	MemberRole string    `json:"memberRole"`
	CreatedBy  *int64    `json:"createdBy"`
	CreatedAt  time.Time `json:"createdAt"`
}
