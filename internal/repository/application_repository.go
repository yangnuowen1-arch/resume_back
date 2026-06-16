package repository

import (
	"context"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"gorm.io/gorm"
)

type ApplicationListItem struct {
	ID             int64
	JobID          int64
	JobTitle       string
	CandidateID    *int64
	CandidateName  *string
	ResumeID       int64
	ResumeFilename *string
	Source         *string
	Status         string
	ReceivedAt     time.Time
	Remark         *string
	CreatedBy      *int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ApplicationListFilter struct {
	Keyword     string
	JobID       *int64
	CandidateID *int64
	ResumeID    *int64
	Status      string
	Source      string
	Page        int
	PageSize    int
}

type ApplicationRepository interface {
	Create(ctx context.Context, application *model.Application) error
	FindOrCreateForScreening(ctx context.Context, jobID int64, resumeID int64, candidateID *int64, createdBy int64) (*model.Application, error)
	List(ctx context.Context, filter ApplicationListFilter) ([]ApplicationListItem, int64, error)
}

type applicationRepository struct {
	db *gorm.DB
}

func NewApplicationRepository(db *gorm.DB) ApplicationRepository {
	return &applicationRepository{
		db: db,
	}
}

func (r *applicationRepository) Create(ctx context.Context, application *model.Application) error {
	return r.db.WithContext(ctx).Create(application).Error
}

func (r *applicationRepository) FindOrCreateForScreening(ctx context.Context, jobID int64, resumeID int64, candidateID *int64, createdBy int64) (*model.Application, error) {
	application := &model.Application{}
	err := r.db.WithContext(ctx).
		Where("job_id = ? AND resume_id = ?", jobID, resumeID).
		Order("id DESC").
		First(application).Error
	if err == nil {
		return application, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	source := "ai_screening"
	application = &model.Application{
		JobID:       jobID,
		CandidateID: candidateID,
		ResumeID:    resumeID,
		Source:      &source,
		Status:      "screening",
		CreatedBy:   &createdBy,
	}
	if err := r.db.WithContext(ctx).Create(application).Error; err != nil {
		return nil, err
	}

	return application, nil
}

func (r *applicationRepository) List(ctx context.Context, filter ApplicationListFilter) ([]ApplicationListItem, int64, error) {
	queryBuilder := r.db.WithContext(ctx).
		Table(model.TableNameApplication).
		Joins("JOIN " + model.TableNameJob + " ON jobs.id = applications.job_id").
		Joins("LEFT JOIN " + model.TableNameCandidate + " ON candidates.id = applications.candidate_id").
		Joins("JOIN " + model.TableNameResume + " ON resumes.id = applications.resume_id")

	if filter.Keyword != "" {
		like := "%" + filter.Keyword + "%"
		queryBuilder = queryBuilder.Where("(jobs.title LIKE ? OR candidates.name LIKE ? OR resumes.original_filename LIKE ?)", like, like, like)
	}

	if filter.JobID != nil {
		queryBuilder = queryBuilder.Where("applications.job_id = ?", *filter.JobID)
	}

	if filter.CandidateID != nil {
		queryBuilder = queryBuilder.Where("applications.candidate_id = ?", *filter.CandidateID)
	}

	if filter.ResumeID != nil {
		queryBuilder = queryBuilder.Where("applications.resume_id = ?", *filter.ResumeID)
	}

	if filter.Status != "" {
		queryBuilder = queryBuilder.Where("applications.status = ?", filter.Status)
	}

	if filter.Source != "" {
		queryBuilder = queryBuilder.Where("applications.source = ?", filter.Source)
	}

	var total int64
	if err := queryBuilder.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]ApplicationListItem, 0)
	err := queryBuilder.
		Select("applications.id, applications.job_id, jobs.title AS job_title, applications.candidate_id, candidates.name AS candidate_name, applications.resume_id, resumes.original_filename AS resume_filename, applications.source, applications.status, applications.received_at, applications.remark, applications.created_by, applications.created_at, applications.updated_at").
		Order("applications.id DESC").
		Limit(filter.PageSize).
		Offset((filter.Page - 1) * filter.PageSize).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
