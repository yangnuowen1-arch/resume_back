package repository

import (
	"context"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/timeutil"
	"gorm.io/gorm"
)

type ScreeningTaskListItem struct {
	ID             int64
	ApplicationID  int64
	CandidateID    *int64
	CandidateName  *string
	JobID          int64
	JobTitle       string
	Position       string
	AIScore        *float64
	Status         string
	CreatedAt      time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
	RetryCount     int
	LastErrorAt    *time.Time
	CreatedBy      *int64
	MatchLevel     *string
	Recommendation *string
	ErrorMessage   *string
}

type ScreeningTaskListFilter struct {
	Keyword     string
	Status      string
	JobID       *int64
	CandidateID *int64
	Page        int
	PageSize    int
}

type ScreeningTaskDetailItem struct {
	ID                         int64
	Status                     string
	ErrorMessage               *string
	CandidateName              *string
	Position                   string
	CandidateCurrentTitle      *string
	CandidateYearsOfExperience *float64
	CandidateHighestEducation  *string
	AIScore                    *float64
	MatchLevel                 *string
	Recommendation             *string
	Summary                    *string
	Strengths                  *string
	Weaknesses                 *string
	Risks                      *string
	RawResponse                *string
	Requirements               *string
	ResumeText                 *string
}

type ScreeningResultSuccessUpdate struct {
	CandidateName       *string
	Score               *float64
	MatchLevel          *string
	Recommendation      *string
	Summary             *string
	Strengths           *string
	Weaknesses          *string
	Risks               *string
	MissingRequirements *string
	Requirements        *string
	AIProvider          *string
	AIModel             *string
	PromptVersion       *string
	RawResponse         *string
	RetryCount          int
}

type ScreeningTaskRepository interface {
	Create(ctx context.Context, result *model.ScreeningResult) error
	MarkRunning(ctx context.Context, id int64) error
	MarkSuccess(ctx context.Context, id int64, update ScreeningResultSuccessUpdate) error
	MarkFailed(ctx context.Context, id int64, message string) error
	List(ctx context.Context, filter ScreeningTaskListFilter) ([]ScreeningTaskListItem, int64, error)
	FindDetailByID(ctx context.Context, id int64) (*ScreeningTaskDetailItem, error)
}

type screeningTaskRepository struct {
	db *gorm.DB
}

func NewScreeningTaskRepository(db *gorm.DB) ScreeningTaskRepository {
	return &screeningTaskRepository{db: db}
}

func (r *screeningTaskRepository) Create(ctx context.Context, result *model.ScreeningResult) error {
	if result.CreatedAt.IsZero() {
		result.CreatedAt = timeutil.Now()
	}

	return r.db.WithContext(ctx).Create(result).Error
}

func (r *screeningTaskRepository) MarkRunning(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.ScreeningResult{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        "running",
			"error_message": nil,
		}).Error
}

func (r *screeningTaskRepository) MarkSuccess(ctx context.Context, id int64, update ScreeningResultSuccessUpdate) error {
	return r.db.WithContext(ctx).
		Model(&model.ScreeningResult{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"candidate_name":       update.CandidateName,
			"score":                update.Score,
			"match_level":          update.MatchLevel,
			"recommendation":       update.Recommendation,
			"summary":              update.Summary,
			"strengths":            update.Strengths,
			"weaknesses":           update.Weaknesses,
			"risks":                update.Risks,
			"missing_requirements": update.MissingRequirements,
			"requirements":         update.Requirements,
			"ai_provider":          update.AIProvider,
			"ai_model":             update.AIModel,
			"prompt_version":       update.PromptVersion,
			"raw_response":         update.RawResponse,
			"status":               "success",
			"error_message":        nil,
		}).Error
}

func (r *screeningTaskRepository) MarkFailed(ctx context.Context, id int64, message string) error {
	return r.db.WithContext(ctx).
		Model(&model.ScreeningResult{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        "failed",
			"error_message": message,
		}).Error
}

func (r *screeningTaskRepository) List(ctx context.Context, filter ScreeningTaskListFilter) ([]ScreeningTaskListItem, int64, error) {
	queryBuilder := r.buildScreeningTaskListBaseQuery(ctx, filter)

	var total int64
	if err := queryBuilder.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]ScreeningTaskListItem, 0)
	err := queryBuilder.
		Select(screeningTaskListSelectColumns).
		Order("screening_results.created_at DESC, screening_results.id DESC").
		Limit(filter.PageSize).
		Offset((filter.Page - 1) * filter.PageSize).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *screeningTaskRepository) FindDetailByID(ctx context.Context, id int64) (*ScreeningTaskDetailItem, error) {
	item := &ScreeningTaskDetailItem{}
	err := r.db.WithContext(ctx).
		Table(model.TableNameScreeningResult).
		Joins("JOIN "+model.TableNameApplication+" ON applications.id = screening_results.application_id").
		Joins("JOIN "+model.TableNameJob+" ON jobs.id = applications.job_id").
		Joins("LEFT JOIN "+model.TableNameCandidate+" ON candidates.id = applications.candidate_id").
		Joins("LEFT JOIN "+model.TableNameResume+" ON resumes.id = applications.resume_id").
		Select(screeningTaskDetailSelectColumns).
		Where("screening_results.id = ?", id).
		Take(item).Error
	if err != nil {
		return nil, err
	}

	return item, nil
}

const screeningTaskDetailSelectColumns = `
	screening_results.id,
	screening_results.status,
	screening_results.error_message,
` + screeningTaskCandidateNameExpression + ` AS candidate_name,
	jobs.title AS position,
	candidates.current_position AS candidate_current_title,
	candidates.years_of_experience AS candidate_years_of_experience,
	candidates.highest_education AS candidate_highest_education,
	screening_results.score AS ai_score,
	screening_results.match_level,
	screening_results.recommendation,
	screening_results.summary,
	screening_results.strengths,
	screening_results.weaknesses,
	screening_results.risks,
	screening_results.raw_response,
	screening_results.requirements,
	resumes.raw_text AS resume_text
`

func (r *screeningTaskRepository) buildScreeningTaskListBaseQuery(ctx context.Context, filter ScreeningTaskListFilter) *gorm.DB {
	queryBuilder := r.db.WithContext(ctx).
		Table(model.TableNameScreeningResult).
		Joins("JOIN " + model.TableNameApplication + " ON applications.id = screening_results.application_id").
		Joins("JOIN " + model.TableNameJob + " ON jobs.id = applications.job_id").
		Joins("LEFT JOIN " + model.TableNameCandidate + " ON candidates.id = applications.candidate_id")

	if filter.Keyword != "" {
		like := "%" + filter.Keyword + "%"
		queryBuilder = queryBuilder.Where("("+screeningTaskCandidateNameExpression+" LIKE ? OR candidates.email LIKE ? OR candidates.phone LIKE ? OR jobs.title LIKE ?)", like, like, like, like)
	}

	if filter.Status != "" {
		queryBuilder = queryBuilder.Where("screening_results.status = ?", filter.Status)
	}

	if filter.JobID != nil {
		queryBuilder = queryBuilder.Where("applications.job_id = ?", *filter.JobID)
	}

	if filter.CandidateID != nil {
		queryBuilder = queryBuilder.Where("applications.candidate_id = ?", *filter.CandidateID)
	}

	return queryBuilder
}

// screeningTaskCandidateNameExpression prefers the canonical candidate record,
// then the persistent AI-extracted name. The JSONB fallback supports rows that
// predate the candidate_name migration.
const screeningTaskCandidateNameExpression = `
	COALESCE(
		NULLIF(BTRIM(candidates.name), ''),
		NULLIF(BTRIM(screening_results.candidate_name), ''),
		CASE
			WHEN jsonb_typeof(screening_results.raw_response #> '{output,candidate_name}') = 'string'
			THEN NULLIF(BTRIM(screening_results.raw_response #>> '{output,candidate_name}'), '')
		END,
		CASE
			WHEN jsonb_typeof(screening_results.raw_response -> 'candidate_name') = 'string'
			THEN NULLIF(BTRIM(screening_results.raw_response ->> 'candidate_name'), '')
		END
	)
`

const screeningTaskListSelectColumns = `
	screening_results.id,
	screening_results.application_id,
	applications.candidate_id,
` + screeningTaskCandidateNameExpression + ` AS candidate_name,
	applications.job_id,
	jobs.title AS job_title,
	jobs.title AS position,
	screening_results.score AS ai_score,
	screening_results.status,
	screening_results.created_at,
	screening_results.created_by,
	screening_results.match_level,
	screening_results.recommendation,
	screening_results.error_message
`
