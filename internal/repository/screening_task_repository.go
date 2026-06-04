package repository

import (
	"context"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
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

type ScreeningTaskRepository interface {
	List(ctx context.Context, filter ScreeningTaskListFilter) ([]ScreeningTaskListItem, int64, error)
}

type screeningTaskRepository struct {
	db *gorm.DB
}

func NewScreeningTaskRepository(db *gorm.DB) ScreeningTaskRepository {
	return &screeningTaskRepository{db: db}
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

func (r *screeningTaskRepository) buildScreeningTaskListBaseQuery(ctx context.Context, filter ScreeningTaskListFilter) *gorm.DB {
	queryBuilder := r.db.WithContext(ctx).
		Table(model.TableNameScreeningResult).
		Joins("JOIN " + model.TableNameApplication + " ON applications.id = screening_results.application_id").
		Joins("JOIN " + model.TableNameJob + " ON jobs.id = applications.job_id").
		Joins("LEFT JOIN " + model.TableNameCandidate + " ON candidates.id = applications.candidate_id")

	if filter.Keyword != "" {
		like := "%" + filter.Keyword + "%"
		queryBuilder = queryBuilder.Where("(candidates.name LIKE ? OR candidates.email LIKE ? OR candidates.phone LIKE ? OR jobs.title LIKE ?)", like, like, like, like)
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

const screeningTaskListSelectColumns = `
	screening_results.id,
	screening_results.application_id,
	applications.candidate_id,
	candidates.name AS candidate_name,
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
