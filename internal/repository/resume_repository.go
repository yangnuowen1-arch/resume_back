package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"gorm.io/gorm"
)

type ResumeListItem struct {
	ID               int64
	CandidateID      *int64
	CandidateName    *string
	OriginalFilename *string
	FileKey          *string
	FileURL          *string
	FileType         *string
	FileSize         *int64
	RawText          *string
	ParsedData       *string
	ParseStatus      string
	ParseError       *string
	ParsedAt         *time.Time
	Language         *string
	UploadBy         *int64
	UploadedAt       time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ResumeRepository interface {
	Create(ctx context.Context, resume *model.Resume) error
	FindByID(ctx context.Context, id int64) (*model.Resume, error)
	FindByFileHash(ctx context.Context, fileHash string) (*model.Resume, error)
	List(ctx context.Context, keyword string, candidateID *int64, language string, page int, pageSize int) ([]ResumeListItem, int64, error)
	MarkParsing(ctx context.Context, id int64) error
	MarkParsed(ctx context.Context, id int64, rawText string, parsedData *string, language *string, parsedAt time.Time) error
	MarkParseFailed(ctx context.Context, id int64, message string) error
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

// FindByFileHash 按文件 SHA-256 查简历，用于邮箱导入时跳过重复附件。
// 未命中返回 (nil, nil)，便于调用方区分「重复」与「出错」。
func (r *resumeRepository) FindByFileHash(ctx context.Context, fileHash string) (*model.Resume, error) {
	fileHash = strings.TrimSpace(fileHash)
	if fileHash == "" {
		return nil, nil
	}

	resume := &model.Resume{}
	err := r.db.WithContext(ctx).
		Where("file_hash = ?", fileHash).
		First(resume).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
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
		Select("resumes.id, resumes.candidate_id, candidates.name AS candidate_name, resumes.original_filename, resumes.file_key, resumes.file_url, resumes.file_type, resumes.file_size, resumes.raw_text, resumes.parsed_data, resumes.parse_status, resumes.parse_error, resumes.parsed_at, resumes.language, resumes.upload_by, resumes.uploaded_at, resumes.created_at, resumes.updated_at").
		Order("resumes.id DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *resumeRepository) MarkParsing(ctx context.Context, id int64) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.Resume{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"parse_status": "parsing",
			"parse_error":  nil,
			"updated_at":   now,
		}).Error
}

func (r *resumeRepository) MarkParsed(ctx context.Context, id int64, rawText string, parsedData *string, language *string, parsedAt time.Time) error {
	updates := map[string]interface{}{
		"raw_text":     rawText,
		"parsed_data":  parsedData,
		"parse_status": "parsed",
		"parse_error":  nil,
		"parsed_at":    parsedAt,
		"updated_at":   parsedAt,
	}
	if language != nil {
		updates["language"] = language
	}

	return r.db.WithContext(ctx).
		Model(&model.Resume{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *resumeRepository) MarkParseFailed(ctx context.Context, id int64, message string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.Resume{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"parse_status": "failed",
			"parse_error":  message,
			"parsed_at":    nil,
			"updated_at":   now,
		}).Error
}
