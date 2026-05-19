package repository

import (
	"context"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"gorm.io/gorm"
)

type ResumeRepository interface {
	Create(ctx context.Context, resume *model.Resume) error
	FindByID(ctx context.Context, id int64) (*model.Resume, error)
	CandidateExists(ctx context.Context, id int64) (bool, error)
}

type resumeRepository struct {
	db *gorm.DB
}

func NewResumeRepository(db *gorm.DB) ResumeRepository {
	return &resumeRepository{
		db: db,
	}
}

func (r *resumeRepository) Create(ctx context.Context, resume *model.Resume) error {
	return r.db.WithContext(ctx).Create(resume).Error
}

func (r *resumeRepository) FindByID(ctx context.Context, id int64) (*model.Resume, error) {
	resume := &model.Resume{}
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(resume).Error
	if err != nil {
		return nil, err
	}

	return resume, nil
}

func (r *resumeRepository) CandidateExists(ctx context.Context, id int64) (bool, error) {
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
