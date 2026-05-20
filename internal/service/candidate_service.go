package service

import (
	"context"
	"errors"
	"strings"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
)

type CandidateService interface {
	Create(ctx context.Context, req dto.CreateCandidateRequest) (int64, error)
	Update(ctx context.Context, id int64, req dto.UpdateCandidateRequest) error
	List(ctx context.Context, query dto.CandidateQuery) ([]dto.CandidateResponse, int64, error)
}

type candidateService struct {
	repo repository.CandidateRepository
}

func NewCandidateService(repo repository.CandidateRepository) CandidateService {
	return &candidateService{
		repo: repo,
	}
}

func (s *candidateService) Create(ctx context.Context, req dto.CreateCandidateRequest) (int64, error) {
	if _, err := currentUserID(ctx); err != nil {
		return 0, err
	}

	normalizeCreateCandidateRequest(&req)
	if err := validateCandidate(req.Name, req.YearsOfExperience); err != nil {
		return 0, err
	}

	name := req.Name
	candidate := &model.Candidate{
		Name:              &name,
		Email:             req.Email,
		Phone:             req.Phone,
		Gender:            req.Gender,
		CurrentCompany:    req.CurrentCompany,
		CurrentPosition:   req.CurrentPosition,
		YearsOfExperience: req.YearsOfExperience,
		HighestEducation:  req.HighestEducation,
		School:            req.School,
		Major:             req.Major,
		Location:          req.Location,
		Source:            req.Source,
	}
	if err := s.repo.Create(ctx, candidate); err != nil {
		return 0, err
	}

	return candidate.ID, nil
}

func (s *candidateService) Update(ctx context.Context, id int64, req dto.UpdateCandidateRequest) error {
	if _, err := currentUserID(ctx); err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("候选人 ID 不合法")
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return errors.New("候选人不存在")
	}

	createReq := dto.CreateCandidateRequest(req)
	normalizeCreateCandidateRequest(&createReq)
	if err := validateCandidate(createReq.Name, createReq.YearsOfExperience); err != nil {
		return err
	}

	name := createReq.Name
	candidate := &model.Candidate{
		ID:                id,
		Name:              &name,
		Email:             createReq.Email,
		Phone:             createReq.Phone,
		Gender:            createReq.Gender,
		CurrentCompany:    createReq.CurrentCompany,
		CurrentPosition:   createReq.CurrentPosition,
		YearsOfExperience: createReq.YearsOfExperience,
		HighestEducation:  createReq.HighestEducation,
		School:            createReq.School,
		Major:             createReq.Major,
		Location:          createReq.Location,
		Source:            createReq.Source,
	}

	return s.repo.Update(ctx, candidate)
}

func (s *candidateService) List(ctx context.Context, query dto.CandidateQuery) ([]dto.CandidateResponse, int64, error) {
	query = normalizeCandidateQuery(query)

	items, total, err := s.repo.List(ctx, strings.TrimSpace(query.Keyword), strings.TrimSpace(query.Source), query.Page, query.PageSize)
	if err != nil {
		return nil, 0, err
	}

	result := make([]dto.CandidateResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toCandidateResponse(item))
	}

	return result, total, nil
}

func normalizeCreateCandidateRequest(req *dto.CreateCandidateRequest) {
	req.Name = strings.TrimSpace(req.Name)
	req.Email = trimOptionalString(req.Email)
	req.Phone = trimOptionalString(req.Phone)
	req.Gender = trimOptionalString(req.Gender)
	req.CurrentCompany = trimOptionalString(req.CurrentCompany)
	req.CurrentPosition = trimOptionalString(req.CurrentPosition)
	req.HighestEducation = trimOptionalString(req.HighestEducation)
	req.School = trimOptionalString(req.School)
	req.Major = trimOptionalString(req.Major)
	req.Location = trimOptionalString(req.Location)
	req.Source = trimOptionalString(req.Source)
}

func validateCandidate(name string, yearsOfExperience *float64) error {
	if name == "" {
		return errors.New("候选人姓名不能为空")
	}
	if yearsOfExperience != nil && (*yearsOfExperience < 0 || *yearsOfExperience > 80) {
		return errors.New("工作年限不合法")
	}

	return nil
}

func normalizeCandidateQuery(query dto.CandidateQuery) dto.CandidateQuery {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}

	return query
}

func toCandidateResponse(candidate *model.Candidate) dto.CandidateResponse {
	return dto.CandidateResponse{
		ID:                candidate.ID,
		Name:              candidate.Name,
		Email:             candidate.Email,
		Phone:             candidate.Phone,
		Gender:            candidate.Gender,
		CurrentCompany:    candidate.CurrentCompany,
		CurrentPosition:   candidate.CurrentPosition,
		YearsOfExperience: candidate.YearsOfExperience,
		HighestEducation:  candidate.HighestEducation,
		School:            candidate.School,
		Major:             candidate.Major,
		Location:          candidate.Location,
		Source:            candidate.Source,
		CreatedAt:         candidate.CreatedAt,
		UpdatedAt:         candidate.UpdatedAt,
	}
}
