package service

import (
	"context"
	"errors"
	"strings"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
)

type ResumeService interface {
	CreateUploadedResume(ctx context.Context, req dto.UploadResumeRequest) (*dto.ResumeResponse, error)
	List(ctx context.Context, query dto.ResumeQuery) ([]dto.ResumeResponse, int64, error)
}

type resumeService struct {
	resumeRepo    repository.ResumeRepository
	candidateRepo repository.CandidateRepository
}

func NewResumeService(resumeRepo repository.ResumeRepository, candidateRepo repository.CandidateRepository) ResumeService {
	return &resumeService{
		resumeRepo:    resumeRepo,
		candidateRepo: candidateRepo,
	}
}

func (s *resumeService) CreateUploadedResume(ctx context.Context, req dto.UploadResumeRequest) (*dto.ResumeResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}

	req.OriginalFilename = strings.TrimSpace(req.OriginalFilename)
	req.FileURL = strings.TrimSpace(req.FileURL)
	req.FileType = strings.TrimSpace(req.FileType)
	req.RawText = trimOptionalString(req.RawText)
	req.Language = trimOptionalString(req.Language)
	if req.OriginalFilename == "" {
		return nil, errors.New("原始文件名不能为空")
	}
	if req.FileURL == "" {
		return nil, errors.New("简历文件地址不能为空")
	}
	if req.FileSize <= 0 {
		return nil, errors.New("简历文件大小不合法")
	}

	if req.CandidateID != nil {
		if _, err := s.candidateRepo.FindByID(ctx, *req.CandidateID); err != nil {
			return nil, errors.New("候选人不存在")
		}
	}

	resume := &model.Resume{
		CandidateID:      req.CandidateID,
		OriginalFilename: &req.OriginalFilename,
		FileURL:          &req.FileURL,
		FileType:         &req.FileType,
		FileSize:         &req.FileSize,
		RawText:          req.RawText,
		Language:         req.Language,
		UploadBy:         &userID,
	}
	if err := s.resumeRepo.Create(ctx, resume); err != nil {
		return nil, err
	}

	return toResumeResponse(resume), nil
}

func (s *resumeService) List(ctx context.Context, query dto.ResumeQuery) ([]dto.ResumeResponse, int64, error) {
	query = normalizeResumeQuery(query)

	if query.CandidateID != nil {
		if _, err := s.candidateRepo.FindByID(ctx, *query.CandidateID); err != nil {
			return nil, 0, errors.New("候选人不存在")
		}
	}

	items, total, err := s.resumeRepo.List(ctx, strings.TrimSpace(query.Keyword), query.CandidateID, strings.TrimSpace(query.Language), query.Page, query.PageSize)
	if err != nil {
		return nil, 0, err
	}

	result := make([]dto.ResumeResponse, 0, len(items))
	for _, item := range items {
		result = append(result, dto.ResumeResponse{
			ID:               item.ID,
			CandidateID:      item.CandidateID,
			CandidateName:    item.CandidateName,
			OriginalFilename: item.OriginalFilename,
			FileURL:          item.FileURL,
			FileType:         item.FileType,
			FileSize:         item.FileSize,
			RawText:          item.RawText,
			Language:         item.Language,
			UploadBy:         item.UploadBy,
			UploadedAt:       item.UploadedAt,
			CreatedAt:        item.CreatedAt,
			UpdatedAt:        item.UpdatedAt,
		})
	}

	return result, total, nil
}

func toResumeResponse(resume *model.Resume) *dto.ResumeResponse {
	return &dto.ResumeResponse{
		ID:               resume.ID,
		CandidateID:      resume.CandidateID,
		OriginalFilename: resume.OriginalFilename,
		FileURL:          resume.FileURL,
		FileType:         resume.FileType,
		FileSize:         resume.FileSize,
		RawText:          resume.RawText,
		Language:         resume.Language,
		UploadBy:         resume.UploadBy,
		UploadedAt:       resume.UploadedAt,
		CreatedAt:        resume.CreatedAt,
		UpdatedAt:        resume.UpdatedAt,
	}
}

func normalizeResumeQuery(query dto.ResumeQuery) dto.ResumeQuery {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}

	return query
}
