package repository

import (
	"context"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"gorm.io/gorm"
)

type ResumeListItem struct {
	ID               int64
	CandidateID      *int64
	CandidateName    *string
	OriginalFilename *string
	FileURL          *string
	FileType         *string
	FileSize         *int64
	RawText          *string
	Language         *string
	UploadBy         *int64
	UploadedAt       time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ResumeRepository interface {
	Create(ctx context.Context, resume *model.Resume) error
	FindByID(ctx context.Context, id int64) (*model.Resume, error)
	List(ctx context.Context, keyword string, candidateID *int64, language string, page int, pageSize int) ([]ResumeListItem, int64, error)
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

func (r *resumeRepository) List(
	ctx context.Context,
	keyword string,
	candidateID *int64,
	language string,
	page int,
	pageSize int,
) ([]ResumeListItem, int64, error) {
	queryBuilder := r.db.WithContext(ctx).
		Table(model.TableNameResume).
		Joins("LEFT JOIN " + model.TableNameCandidate + " ON candidates.id = resumes.candidate_id")

	if keyword != "" {
		like := "%" + keyword + "%"
		queryBuilder = queryBuilder.Where("(resumes.original_filename LIKE ? OR resumes.raw_text LIKE ? OR candidates.name LIKE ?)", like, like, like)
	}

	if candidateID != nil {
		queryBuilder = queryBuilder.Where("resumes.candidate_id = ?", *candidateID)
	}

	if language != "" {
		queryBuilder = queryBuilder.Where("resumes.language = ?", language)
	}

	var total int64
	if err := queryBuilder.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]ResumeListItem, 0)
	err := queryBuilder.
		Select("resumes.id, resumes.candidate_id, candidates.name AS candidate_name, resumes.original_filename, resumes.file_url, resumes.file_type, resumes.file_size, resumes.raw_text, resumes.language, resumes.upload_by, resumes.uploaded_at, resumes.created_at, resumes.updated_at").
		Order("resumes.id DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
