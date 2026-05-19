package repository

import (
	"context"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"gorm.io/gorm"
)

type ApplicationRepository interface {
	Create(ctx context.Context, application *model.Application) error
	JobExists(ctx context.Context, id int64) (bool, error)
	FindResumeByID(ctx context.Context, id int64) (*model.Resume, error)
	CandidateExists(ctx context.Context, id int64) (bool, error)
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

func (r *applicationRepository) JobExists(ctx context.Context, id int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Job{}).
		Where("id = ?", id).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *applicationRepository) FindResumeByID(ctx context.Context, id int64) (*model.Resume, error) {
	resume := &model.Resume{}
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(resume).Error
	if err != nil {
		return nil, err
	}

	return resume, nil
}

func (r *applicationRepository) CandidateExists(ctx context.Context, id int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Candidate{}).
		Where("id = ?", id).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
