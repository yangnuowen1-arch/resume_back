package service

import (
	"context"
	"errors"
	"strings"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
)

type ApplicationService interface {
	Create(ctx context.Context, req dto.CreateApplicationRequest) (*dto.ApplicationResponse, error)
}

type applicationService struct {
	repo repository.ApplicationRepository
}

func NewApplicationService(repo repository.ApplicationRepository) ApplicationService {
	return &applicationService{
		repo: repo,
	}
}

func (s *applicationService) Create(ctx context.Context, req dto.CreateApplicationRequest) (*dto.ApplicationResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}

	req.Source = trimOptionalString(req.Source)
	req.Remark = trimOptionalString(req.Remark)
	req.Status = strings.TrimSpace(req.Status)
	if req.Status == "" {
		req.Status = "received"
	}

	if req.JobID <= 0 {
		return nil, errors.New("岗位 ID 不合法")
	}
	if req.ResumeID <= 0 {
		return nil, errors.New("简历 ID 不合法")
	}

	jobExists, err := s.repo.JobExists(ctx, req.JobID)
	if err != nil {
		return nil, err
	}
	if !jobExists {
		return nil, errors.New("岗位不存在")
	}

	resume, err := s.repo.FindResumeByID(ctx, req.ResumeID)
	if err != nil {
		return nil, errors.New("简历不存在")
	}

	if req.CandidateID == nil {
		req.CandidateID = resume.CandidateID
	}
	if req.CandidateID != nil {
		candidateExists, err := s.repo.CandidateExists(ctx, *req.CandidateID)
		if err != nil {
			return nil, err
		}
		if !candidateExists {
			return nil, errors.New("候选人不存在")
		}
	}

	application := &model.Application{
		JobID:       req.JobID,
		CandidateID: req.CandidateID,
		ResumeID:    req.ResumeID,
		Source:      req.Source,
		Status:      req.Status,
		Remark:      req.Remark,
		CreatedBy:   &userID,
	}
	if err := s.repo.Create(ctx, application); err != nil {
		return nil, err
	}

	return toApplicationResponse(application), nil
}

func toApplicationResponse(application *model.Application) *dto.ApplicationResponse {
	return &dto.ApplicationResponse{
		ID:          application.ID,
		JobID:       application.JobID,
		CandidateID: application.CandidateID,
		ResumeID:    application.ResumeID,
		Source:      application.Source,
		Status:      application.Status,
		ReceivedAt:  application.ReceivedAt,
		Remark:      application.Remark,
		CreatedBy:   application.CreatedBy,
		CreatedAt:   application.CreatedAt,
		UpdatedAt:   application.UpdatedAt,
	}
}
