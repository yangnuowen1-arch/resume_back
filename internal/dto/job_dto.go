package dto

import "time"

type CreateJobRequest struct {
	CategoryID       *int64                 `json:"categoryId"`
	Title            string                 `json:"title" binding:"required"`
	Headcount        int32                  `json:"headcount"`
	SalaryMin        *int32                 `json:"salaryMin"`
	SalaryMax        *int32                 `json:"salaryMax"`
	SalaryMonths     *int32                 `json:"salaryMonths"`
	ExperienceMin    *int32                 `json:"experienceMin"`
	ExperienceMax    *int32                 `json:"experienceMax"`
	Description      *string                `json:"description"`
	Responsibilities *string                `json:"responsibilities"`
	Requirements     *string                `json:"requirements"`
	BonusPoints      *string                `json:"bonusPoints"`
	Status           string                 `json:"status"`
	Priority         string                 `json:"priority"`
	OwnerUserID      *int64                 `json:"ownerUserId"`
	TagIDs           []int64                `json:"tagIds"`
	DynamicFields    map[string]interface{} `json:"dynamicFields" swaggertype:"object"`
}

type UpdateJobRequest struct {
	CategoryID       *int64                 `json:"categoryId"`
	Title            string                 `json:"title" binding:"required"`
	Headcount        int32                  `json:"headcount"`
	SalaryMin        *int32                 `json:"salaryMin"`
	SalaryMax        *int32                 `json:"salaryMax"`
	SalaryMonths     *int32                 `json:"salaryMonths"`
	ExperienceMin    *int32                 `json:"experienceMin"`
	ExperienceMax    *int32                 `json:"experienceMax"`
	Description      *string                `json:"description"`
	Responsibilities *string                `json:"responsibilities"`
	Requirements     *string                `json:"requirements"`
	BonusPoints      *string                `json:"bonusPoints"`
	Status           string                 `json:"status" binding:"required"`
	Priority         string                 `json:"priority" binding:"required"`
	OwnerUserID      *int64                 `json:"ownerUserId"`
	TagIDs           []int64                `json:"tagIds"`
	DynamicFields    map[string]interface{} `json:"dynamicFields" swaggertype:"object"`
}

type JobQuery struct {
	Page       int
	PageSize   int
	Keyword    string
	CategoryID *int64
	Status     string
}

type JobResponse struct {
	ID               int64                  `json:"id"`
	CategoryID       *int64                 `json:"categoryId"`
	Title            string                 `json:"title"`
	Headcount        int32                  `json:"headcount"`
	SalaryMin        *int32                 `json:"salaryMin"`
	SalaryMax        *int32                 `json:"salaryMax"`
	SalaryMonths     *int32                 `json:"salaryMonths"`
	ExperienceMin    *int32                 `json:"experienceMin"`
	ExperienceMax    *int32                 `json:"experienceMax"`
	Description      *string                `json:"description"`
	Responsibilities *string                `json:"responsibilities"`
	Requirements     *string                `json:"requirements"`
	BonusPoints      *string                `json:"bonusPoints"`
	Status           string                 `json:"status"`
	Priority         string                 `json:"priority"`
	OwnerUserID      *int64                 `json:"ownerUserId"`
	OwnerRealName    *string                `json:"ownerRealName"`
	CreatedBy        *int64                 `json:"createdBy"`
	CreatorRealName  *string                `json:"creatorRealName"`
	PublishedAt      *time.Time             `json:"publishedAt"`
	ClosedAt         *time.Time             `json:"closedAt"`
	CreatedAt        time.Time              `json:"createdAt"`
	UpdatedAt        time.Time              `json:"updatedAt"`
	Tags             []JobTagResponse       `json:"tags"`
	DynamicFields    map[string]interface{} `json:"dynamicFields"`
}

type JobScreeningContextResponse struct {
	JobID      int64                      `json:"jobId"`
	JobTitle   string                     `json:"jobTitle"`
	JobContext string                     `json:"jobContext"`
	Payload    JobScreeningContextPayload `json:"payload"`
}

type JobScreeningContextPayload struct {
	ContextVersion   string                 `json:"context_version"`
	JobID            int64                  `json:"job_id"`
	JobTitle         string                 `json:"job_title"`
	CategoryID       *int64                 `json:"category_id"`
	Department       *string                `json:"department"`
	Headcount        int32                  `json:"headcount"`
	WorkLocation     *string                `json:"work_location"`
	WorkType         *string                `json:"work_type"`
	EmploymentType   *string                `json:"employment_type"`
	SalaryMin        *int32                 `json:"salary_min"`
	SalaryMax        *int32                 `json:"salary_max"`
	SalaryMonths     *int32                 `json:"salary_months"`
	ExperienceMin    *int32                 `json:"experience_min"`
	ExperienceMax    *int32                 `json:"experience_max"`
	EducationLevel   *string                `json:"education_level"`
	Description      *string                `json:"description"`
	Responsibilities *string                `json:"responsibilities"`
	Requirements     *string                `json:"requirements"`
	BonusPoints      *string                `json:"bonus_points"`
	Priority         string                 `json:"priority"`
	Tags             []JobScreeningTag      `json:"tags"`
	DynamicFields    map[string]interface{} `json:"dynamic_fields"`
}

type JobScreeningTag struct {
	ID      int64   `json:"id"`
	GroupID *int64  `json:"group_id"`
	Name    string  `json:"name"`
	Color   *string `json:"color"`
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
