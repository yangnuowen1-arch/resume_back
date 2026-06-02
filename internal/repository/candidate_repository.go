package repository

import (
	"context"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"gorm.io/gorm"
)

type CandidateListItem struct {
	ID                      int64
	Name                    *string
	Email                   *string
	Phone                   *string
	Gender                  *string
	CurrentCompany          *string
	PositionCategoryID      *int64
	PositionCategoryName    *string
	CurrentJobID            *int64
	CurrentPosition         *string
	CurrentPositionCategory *string
	YearsOfExperience       *float64
	HighestEducation        *string
	School                  *string
	Major                   *string
	Location                *string
	Source                  *string
	Status                  string
	Position                *string
	ResumeID                *int64
	ResumeFilename          *string
	ResumeFileURL           *string
	ResumeLanguage          *string
	ResumeUploadedAt        *time.Time
	ResumeEvaluated         bool
	ScreeningStatus         *string
	AIScore                 *float64
	ApplicationID           *int64
	JobID                   *int64
	JobTitle                *string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type CandidateListFilter struct {
	Keyword  string
	Source   string
	Status   string
	Page     int
	PageSize int
}

type CandidateJobSelection struct {
	ID         int64
	Title      string
	CategoryID *int64
	Status     string
}

type CandidateAnalysisResult struct {
	CandidateID   int64
	ResumeID      *int64
	ApplicationID *int64
	Status        string
	Message       *string
}

type CandidateRepository interface {
	Create(ctx context.Context, candidate *model.Candidate) error
	CreateWithResume(ctx context.Context, candidate *model.Candidate, resume *model.Resume) error
	CreateResumeForCandidate(ctx context.Context, candidateID int64, resume *model.Resume, candidateStatus string) error
	EnqueueScreening(ctx context.Context, candidateID int64, jobID *int64, createdBy int64, candidateStatus string) CandidateAnalysisResult
	Update(ctx context.Context, candidate *model.Candidate) error
	UpdateWithResume(ctx context.Context, candidate *model.Candidate, resume *model.Resume) error
	FindByID(ctx context.Context, id int64) (*model.Candidate, error)
	List(ctx context.Context, filter CandidateListFilter) ([]CandidateListItem, int64, error)
	ActivePositionCategoryExists(ctx context.Context, id int64) (bool, error)
	FindJobSelectionByID(ctx context.Context, id int64) (CandidateJobSelection, error)
}

type candidateRepository struct {
	db *gorm.DB
}

func NewCandidateRepository(db *gorm.DB) CandidateRepository {
	return &candidateRepository{
		db: db,
	}
}

func (r *candidateRepository) Create(ctx context.Context, candidate *model.Candidate) error {
	return r.db.WithContext(ctx).Create(candidate).Error
}

func (r *candidateRepository) CreateWithResume(ctx context.Context, candidate *model.Candidate, resume *model.Resume) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(candidate).Error; err != nil {
			return err
		}

		resume.CandidateID = &candidate.ID
		return tx.Create(resume).Error
	})
}

func (r *candidateRepository) CreateResumeForCandidate(ctx context.Context, candidateID int64, resume *model.Resume, candidateStatus string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidate model.Candidate
		if err := tx.Where("id = ?", candidateID).First(&candidate).Error; err != nil {
			return err
		}

		resume.CandidateID = &candidateID
		if err := tx.Create(resume).Error; err != nil {
			return err
		}

		return tx.Model(&model.Candidate{}).
			Where("id = ?", candidateID).
			Update("status", candidateStatus).Error
	})
}

func (r *candidateRepository) EnqueueScreening(ctx context.Context, candidateID int64, jobID *int64, createdBy int64, candidateStatus string) CandidateAnalysisResult {
	result := CandidateAnalysisResult{
		CandidateID: candidateID,
		Status:      "failed",
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidate model.Candidate
		if err := tx.Where("id = ?", candidateID).First(&candidate).Error; err != nil {
			return err
		}

		resume := &model.Resume{}
		if err := tx.
			Where("candidate_id = ?", candidateID).
			Order("uploaded_at DESC, id DESC").
			First(resume).Error; err != nil {
			return err
		}
		result.ResumeID = &resume.ID

		application := &model.Application{}
		if jobID != nil {
			var count int64
			if err := tx.Model(&model.Job{}).Where("id = ?", *jobID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return gorm.ErrRecordNotFound
			}

			err := tx.
				Where("candidate_id = ? AND resume_id = ? AND job_id = ?", candidateID, resume.ID, *jobID).
				Order("id DESC").
				First(application).Error
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}
			if err == gorm.ErrRecordNotFound {
				application = &model.Application{
					JobID:       *jobID,
					CandidateID: &candidateID,
					ResumeID:    resume.ID,
					Status:      "received",
					CreatedBy:   &createdBy,
				}
				if err := tx.Create(application).Error; err != nil {
					return err
				}
			}
		} else {
			if err := tx.
				Where("candidate_id = ? AND resume_id = ?", candidateID, resume.ID).
				Order("received_at DESC, id DESC").
				First(application).Error; err != nil {
				return err
			}
		}
		result.ApplicationID = &application.ID

		screeningStatus := "pending"
		screeningResult := &model.ScreeningResult{
			ApplicationID: application.ID,
			Status:        screeningStatus,
			CreatedBy:     &createdBy,
		}
		if err := tx.Create(screeningResult).Error; err != nil {
			return err
		}

		if err := tx.Model(&model.Candidate{}).
			Where("id = ?", candidateID).
			Update("status", candidateStatus).Error; err != nil {
			return err
		}

		result.Status = "queued"
		return nil
	})
	if err != nil {
		message := "候选人分析入队失败"
		if result.ResumeID == nil {
			message = "候选人没有可分析的简历"
		} else if jobID == nil {
			message = "候选人最新简历没有投递岗位，请传 jobId"
		} else {
			message = "候选人、岗位或简历不存在"
		}
		result.Message = &message
	}

	return result
}

func (r *candidateRepository) Update(ctx context.Context, candidate *model.Candidate) error {
	return r.db.WithContext(ctx).
		Model(&model.Candidate{}).
		Where("id = ?", candidate.ID).
		Updates(candidateUpdateMap(candidate)).Error
}

func (r *candidateRepository) UpdateWithResume(ctx context.Context, candidate *model.Candidate, resume *model.Resume) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Candidate{}).
			Where("id = ?", candidate.ID).
			Updates(candidateUpdateMap(candidate)).Error; err != nil {
			return err
		}

		resume.CandidateID = &candidate.ID
		return tx.Create(resume).Error
	})
}

func candidateUpdateMap(candidate *model.Candidate) map[string]interface{} {
	return map[string]interface{}{
		"name":                      candidate.Name,
		"email":                     candidate.Email,
		"phone":                     candidate.Phone,
		"gender":                    candidate.Gender,
		"current_company":           candidate.CurrentCompany,
		"position_category_id":      candidate.PositionCategoryID,
		"current_job_id":            candidate.CurrentJobID,
		"current_position":          candidate.CurrentPosition,
		"current_position_category": candidate.CurrentPositionCategory,
		"years_of_experience":       candidate.YearsOfExperience,
		"highest_education":         candidate.HighestEducation,
		"school":                    candidate.School,
		"major":                     candidate.Major,
		"location":                  candidate.Location,
		"source":                    candidate.Source,
		"status":                    candidate.Status,
	}
}

func (r *candidateRepository) FindByID(ctx context.Context, id int64) (*model.Candidate, error) {
	candidate := &model.Candidate{}
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(candidate).Error
	if err != nil {
		return nil, err
	}

	return candidate, nil
}

func (r *candidateRepository) ActivePositionCategoryExists(ctx context.Context, id int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.JobCategory{}).
		Where("id = ? AND status = ?", id, "active").
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *candidateRepository) FindJobSelectionByID(ctx context.Context, id int64) (CandidateJobSelection, error) {
	var item CandidateJobSelection
	err := r.db.WithContext(ctx).
		Model(&model.Job{}).
		Select("id, title, category_id, status").
		Where("id = ?", id).
		First(&item).Error
	if err != nil {
		return CandidateJobSelection{}, err
	}

	return item, nil
}

func (r *candidateRepository) List(ctx context.Context, filter CandidateListFilter) ([]CandidateListItem, int64, error) {
	var total int64
	if err := r.buildCandidateListBaseQuery(ctx, filter).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]CandidateListItem, 0)
	err := r.buildCandidateListDataQuery(ctx, filter).
		Order("candidates.id DESC").
		Limit(filter.PageSize).
		Offset((filter.Page - 1) * filter.PageSize).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *candidateRepository) buildCandidateListBaseQuery(ctx context.Context, filter CandidateListFilter) *gorm.DB {
	queryBuilder := r.db.WithContext(ctx).Model(&model.Candidate{})

	if filter.Keyword != "" {
		like := "%" + filter.Keyword + "%"
		queryBuilder = queryBuilder.Where("(candidates.name LIKE ? OR candidates.email LIKE ? OR candidates.phone LIKE ?)", like, like, like)
	}

	if filter.Source != "" {
		queryBuilder = queryBuilder.Where("candidates.source = ?", filter.Source)
	}

	if filter.Status != "" {
		queryBuilder = queryBuilder.Where("candidates.status = ?", filter.Status)
	}

	return queryBuilder
}

func (r *candidateRepository) buildCandidateListDataQuery(ctx context.Context, filter CandidateListFilter) *gorm.DB {
	return r.buildCandidateListBaseQuery(ctx, filter).
		Select(candidateListSelectColumns).
		Joins(candidateListPositionCategoryJoin).
		Joins(candidateListLatestResumeJoin).
		Joins(candidateListLatestApplicationJoin).
		Joins("LEFT JOIN " + model.TableNameJob + " ON jobs.id = latest_application.job_id").
		Joins(candidateListLatestScreeningJoin)
}

const candidateListSelectColumns = `
	candidates.id,
	candidates.name,
	candidates.email,
	candidates.phone,
	candidates.gender,
	candidates.current_company,
	candidates.position_category_id,
	COALESCE(position_category.name, candidates.current_position_category) AS position_category_name,
	candidates.current_job_id,
	candidates.current_position,
	candidates.current_position_category,
	candidates.years_of_experience,
	candidates.highest_education,
	candidates.school,
	candidates.major,
	candidates.location,
	candidates.source,
	candidates.status,
	COALESCE(jobs.title, candidates.current_position) AS position,
	latest_resume.id AS resume_id,
	latest_resume.original_filename AS resume_filename,
	latest_resume.file_url AS resume_file_url,
	latest_resume.language AS resume_language,
	latest_resume.uploaded_at AS resume_uploaded_at,
	COALESCE(latest_screening.status = 'success', false) AS resume_evaluated,
	latest_screening.status AS screening_status,
	latest_screening.score AS ai_score,
	latest_application.id AS application_id,
	latest_application.job_id,
	jobs.title AS job_title,
	candidates.created_at,
	candidates.updated_at
`

const candidateListPositionCategoryJoin = `
	LEFT JOIN job_categories AS position_category
		ON position_category.id = candidates.position_category_id
`

const candidateListLatestResumeJoin = `
	LEFT JOIN LATERAL (
		SELECT id, original_filename, file_url, language, uploaded_at
		FROM resumes
		WHERE resumes.candidate_id = candidates.id
		ORDER BY uploaded_at DESC, id DESC
		LIMIT 1
	) latest_resume ON true
`

const candidateListLatestApplicationJoin = `
	LEFT JOIN LATERAL (
		SELECT id, job_id, status, received_at
		FROM applications
		WHERE applications.candidate_id = candidates.id
			AND latest_resume.id IS NOT NULL
			AND applications.resume_id = latest_resume.id
		ORDER BY received_at DESC, id DESC
		LIMIT 1
	) latest_application ON true
`

const candidateListLatestScreeningJoin = `
	LEFT JOIN LATERAL (
		SELECT id, score, status, created_at
		FROM screening_results
		WHERE screening_results.application_id = latest_application.id
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	) latest_screening ON true
`
