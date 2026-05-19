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
}

type resumeService struct {
	repo repository.ResumeRepository
}

func NewResumeService(repo repository.ResumeRepository) ResumeService {
	return &resumeService{
		repo: repo,
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
		exists, err := s.repo.CandidateExists(ctx, *req.CandidateID)
		if err != nil {
			return nil, err
		}
		if !exists {
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
	if err := s.repo.Create(ctx, resume); err != nil {
		return nil, err
	}

	return toResumeResponse(resume), nil
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
