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
	List(ctx context.Context, query dto.ApplicationQuery) ([]dto.ApplicationResponse, int64, error)
}

type applicationService struct {
	applicationRepo repository.ApplicationRepository
	jobRepo         repository.JobRepository
	resumeRepo      repository.ResumeRepository
	candidateRepo   repository.CandidateRepository
}

func NewApplicationService(
	applicationRepo repository.ApplicationRepository,
	jobRepo repository.JobRepository,
	resumeRepo repository.ResumeRepository,
	candidateRepo repository.CandidateRepository,
) ApplicationService {
	return &applicationService{
		applicationRepo: applicationRepo,
		jobRepo:         jobRepo,
		resumeRepo:      resumeRepo,
		candidateRepo:   candidateRepo,
	}
}

func (s *applicationService) Create(ctx context.Context, req dto.CreateApplicationRequest) (*dto.ApplicationResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}

	req = normalizeCreateApplicationRequest(req)
	if err := s.validateCreateApplication(ctx, &req); err != nil {
		return nil, err
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
	if err := s.applicationRepo.Create(ctx, application); err != nil {
		return nil, err
	}

	return toApplicationResponse(application), nil
}

func (s *applicationService) List(ctx context.Context, query dto.ApplicationQuery) ([]dto.ApplicationResponse, int64, error) {
	query = normalizeApplicationQuery(query)

	items, total, err := s.applicationRepo.List(ctx, repository.ApplicationListFilter{
		Keyword:     strings.TrimSpace(query.Keyword),
		JobID:       query.JobID,
		CandidateID: query.CandidateID,
		ResumeID:    query.ResumeID,
		Status:      strings.TrimSpace(query.Status),
		Source:      strings.TrimSpace(query.Source),
		Page:        query.Page,
		PageSize:    query.PageSize,
	})
	if err != nil {
		return nil, 0, err
	}

	result := make([]dto.ApplicationResponse, 0, len(items))
	for _, item := range items {
		result = append(result, dto.ApplicationResponse{
			ID:             item.ID,
			JobID:          item.JobID,
			JobTitle:       item.JobTitle,
			CandidateID:    item.CandidateID,
			CandidateName:  item.CandidateName,
			ResumeID:       item.ResumeID,
			ResumeFilename: item.ResumeFilename,
			Source:         item.Source,
			Status:         item.Status,
			ReceivedAt:     item.ReceivedAt,
			Remark:         item.Remark,
			CreatedBy:      item.CreatedBy,
			CreatedAt:      item.CreatedAt,
			UpdatedAt:      item.UpdatedAt,
		})
	}

	return result, total, nil
}

func (s *applicationService) validateCreateApplication(ctx context.Context, req *dto.CreateApplicationRequest) error {
	if req.JobID <= 0 {
		return errors.New("岗位 ID 不合法")
	}
	if req.ResumeID <= 0 {
		return errors.New("简历 ID 不合法")
	}

	if _, err := s.jobRepo.FindByID(ctx, req.JobID); err != nil {
		return errors.New("岗位不存在")
	}

	resume, err := s.resumeRepo.FindByID(ctx, req.ResumeID)
	if err != nil {
		return errors.New("简历不存在")
	}

	if req.CandidateID == nil {
		req.CandidateID = resume.CandidateID
	}
	if req.CandidateID == nil {
		return nil
	}

	if *req.CandidateID <= 0 {
		return errors.New("候选人 ID 不合法")
	}
	if resume.CandidateID != nil && *resume.CandidateID != *req.CandidateID {
		return errors.New("候选人和简历不匹配")
	}
	if _, err := s.candidateRepo.FindByID(ctx, *req.CandidateID); err != nil {
		return errors.New("候选人不存在")
	}

	return nil
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

func normalizeCreateApplicationRequest(req dto.CreateApplicationRequest) dto.CreateApplicationRequest {
	req.Source = trimOptionalString(req.Source)
	req.Remark = trimOptionalString(req.Remark)
	req.Status = strings.TrimSpace(req.Status)
	if req.Status == "" {
		req.Status = "received"
	}

	return req
}

func normalizeApplicationQuery(query dto.ApplicationQuery) dto.ApplicationQuery {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}

	return query
}
