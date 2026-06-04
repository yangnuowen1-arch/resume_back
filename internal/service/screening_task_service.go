package service

import (
	"context"
	"strings"

	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
)

type ScreeningTaskService interface {
	List(ctx context.Context, query dto.ScreeningTaskQuery) ([]dto.ScreeningTaskResponse, int64, error)
}

type screeningTaskService struct {
	repo repository.ScreeningTaskRepository
}

func NewScreeningTaskService(repo repository.ScreeningTaskRepository) ScreeningTaskService {
	return &screeningTaskService{repo: repo}
}

func (s *screeningTaskService) List(ctx context.Context, query dto.ScreeningTaskQuery) ([]dto.ScreeningTaskResponse, int64, error) {
	query = normalizeScreeningTaskQuery(query)

	items, total, err := s.repo.List(ctx, repository.ScreeningTaskListFilter{
		Keyword:     strings.TrimSpace(query.Keyword),
		Status:      normalizeStatusFilter(query.Status),
		JobID:       query.JobID,
		CandidateID: query.CandidateID,
		Page:        query.Page,
		PageSize:    query.PageSize,
	})
	if err != nil {
		return nil, 0, err
	}

	result := make([]dto.ScreeningTaskResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toScreeningTaskResponse(item))
	}

	return result, total, nil
}

func normalizeScreeningTaskQuery(query dto.ScreeningTaskQuery) dto.ScreeningTaskQuery {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 200 {
		query.PageSize = 20
	}

	return query
}

func toScreeningTaskResponse(item repository.ScreeningTaskListItem) dto.ScreeningTaskResponse {
	return dto.ScreeningTaskResponse{
		ID:             item.ID,
		ApplicationID:  item.ApplicationID,
		CandidateID:    item.CandidateID,
		Candidate:      item.CandidateName,
		CandidateName:  item.CandidateName,
		JobID:          item.JobID,
		JobTitle:       item.JobTitle,
		Position:       item.Position,
		AIScore:        item.AIScore,
		Status:         item.Status,
		Date:           item.CreatedAt,
		CreatedAt:      item.CreatedAt,
		CreatedBy:      item.CreatedBy,
		MatchLevel:     item.MatchLevel,
		Recommendation: item.Recommendation,
		ErrorMessage:   item.ErrorMessage,
	}
}
