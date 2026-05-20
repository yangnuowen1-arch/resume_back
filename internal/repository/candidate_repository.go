package repository

import (
	"context"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"gorm.io/gorm"
)

type CandidateRepository interface {
	Create(ctx context.Context, candidate *model.Candidate) error
	Update(ctx context.Context, candidate *model.Candidate) error
	FindByID(ctx context.Context, id int64) (*model.Candidate, error)
	List(ctx context.Context, keyword string, source string, page int, pageSize int) ([]*model.Candidate, int64, error)
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

func (r *candidateRepository) Update(ctx context.Context, candidate *model.Candidate) error {
	return r.db.WithContext(ctx).
		Model(&model.Candidate{}).
		Where("id = ?", candidate.ID).
		Updates(map[string]interface{}{
			"name":                candidate.Name,
			"email":               candidate.Email,
			"phone":               candidate.Phone,
			"gender":              candidate.Gender,
			"current_company":     candidate.CurrentCompany,
			"current_position":    candidate.CurrentPosition,
			"years_of_experience": candidate.YearsOfExperience,
			"highest_education":   candidate.HighestEducation,
			"school":              candidate.School,
			"major":               candidate.Major,
			"location":            candidate.Location,
			"source":              candidate.Source,
		}).Error
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

func (r *candidateRepository) List(
	ctx context.Context,
	keyword string,
	source string,
	page int,
	pageSize int,
) ([]*model.Candidate, int64, error) {
	queryBuilder := r.db.WithContext(ctx).Model(&model.Candidate{})

	if keyword != "" {
		like := "%" + keyword + "%"
		queryBuilder = queryBuilder.Where("(name LIKE ? OR email LIKE ? OR phone LIKE ?)", like, like, like)
	}

	if source != "" {
		queryBuilder = queryBuilder.Where("source = ?", source)
	}

	var total int64
	if err := queryBuilder.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]*model.Candidate, 0)
	err := queryBuilder.
		Order("id DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
