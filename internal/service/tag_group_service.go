package service

import (
	"context"
	"errors"
	"strings"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
)

type TagGroupService interface {
	Create(ctx context.Context, req dto.CreateTagGroupRequest) (int64, error)
	List(ctx context.Context, query dto.TagGroupQuery) ([]dto.TagGroupResponse, int64, error)
}

type tagGroupService struct {
	repo repository.TagGroupRepository
}

func NewTagGroupService(repo repository.TagGroupRepository) TagGroupService {
	return &tagGroupService{
		repo: repo,
	}
}

func (s *tagGroupService) Create(ctx context.Context, req dto.CreateTagGroupRequest) (int64, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return 0, err
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = trimOptionalString(req.Description)
	if req.Name == "" {
		return 0, errors.New("标签分组名称不能为空")
	}

	exists, err := s.repo.ExistsByName(ctx, req.Name)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, errors.New("标签分组名称已存在")
	}

	group := &model.TagGroup{
		Name:        req.Name,
		Description: req.Description,
		SortOrder:   req.SortOrder,
		Status:      "active",
		CreatedBy:   &userID,
	}
	if err := s.repo.Create(ctx, group); err != nil {
		return 0, err
	}

	return group.ID, nil
}

func (s *tagGroupService) List(ctx context.Context, query dto.TagGroupQuery) ([]dto.TagGroupResponse, int64, error) {
	query = normalizeTagGroupQuery(query)

	items, total, err := s.repo.List(ctx, strings.TrimSpace(query.Keyword), query.Status, query.Page, query.PageSize)
	if err != nil {
		return nil, 0, err
	}

	result := make([]dto.TagGroupResponse, 0, len(items))
	for _, item := range items {
		result = append(result, dto.TagGroupResponse{
			ID:          item.ID,
			Name:        item.Name,
			Description: item.Description,
			SortOrder:   item.SortOrder,
			Status:      item.Status,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	return result, total, nil
}

func normalizeTagGroupQuery(query dto.TagGroupQuery) dto.TagGroupQuery {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}
	if query.Status == "" {
		query.Status = "active"
	}

	return query
}
