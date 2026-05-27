package service

import (
	"context"
	"errors"
	"strings"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
)

type TagService interface {
	Create(ctx context.Context, req dto.CreateTagRequest) (int64, error)
	Update(ctx context.Context, id int64, req dto.UpdateTagRequest) error
	List(ctx context.Context, query dto.TagQuery) ([]dto.TagResponse, int64, error)
	ListGrouped(ctx context.Context, status string) ([]dto.TagGroupWithTagsResponse, error)
}

type tagService struct {
	repo      repository.TagRepository
	groupRepo repository.TagGroupRepository
}

func NewTagService(repo repository.TagRepository, groupRepo repository.TagGroupRepository) TagService {
	return &tagService{
		repo:      repo,
		groupRepo: groupRepo,
	}
}

func (s *tagService) Create(ctx context.Context, req dto.CreateTagRequest) (int64, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return 0, err
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Color = trimOptionalString(req.Color)
	req.Status = normalizeActiveDisabledStatus(req.Status)
	if req.Name == "" {
		return 0, errors.New("标签名称不能为空")
	}
	if err := validateActiveDisabledStatus(req.Status, "标签状态"); err != nil {
		return 0, err
	}

	if req.GroupID != nil {
		if _, err := s.groupRepo.FindByID(ctx, *req.GroupID); err != nil {
			return 0, errors.New("标签分组不存在")
		}
	}

	exists, err := s.repo.ExistsByNameInGroup(ctx, req.Name, req.GroupID)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, errors.New("同一分组下标签名称已存在")
	}

	tag := &model.Tag{
		GroupID:   req.GroupID,
		Name:      req.Name,
		Color:     req.Color,
		Status:    req.Status,
		CreatedBy: &userID,
	}
	if err := s.repo.Create(ctx, tag); err != nil {
		return 0, err
	}

	return tag.ID, nil
}

func (s *tagService) Update(ctx context.Context, id int64, req dto.UpdateTagRequest) error {
	if _, err := currentUserID(ctx); err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("标签 ID 不合法")
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Color = trimOptionalString(req.Color)
	req.Status = normalizeActiveDisabledStatus(req.Status)
	if req.Name == "" {
		return errors.New("标签名称不能为空")
	}
	if err := validateActiveDisabledStatus(req.Status, "标签状态"); err != nil {
		return err
	}

	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return errors.New("标签不存在")
	}

	if req.GroupID != nil {
		if _, err := s.groupRepo.FindByID(ctx, *req.GroupID); err != nil {
			return errors.New("标签分组不存在")
		}
	}

	exists, err := s.repo.ExistsByNameInGroupExceptID(ctx, req.Name, req.GroupID, id)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("同一分组下标签名称已存在")
	}

	tag := &model.Tag{
		ID:      id,
		GroupID: req.GroupID,
		Name:    req.Name,
		Color:   req.Color,
		Status:  req.Status,
	}

	return s.repo.Update(ctx, tag)
}

func (s *tagService) List(ctx context.Context, query dto.TagQuery) ([]dto.TagResponse, int64, error) {
	query = normalizeTagQuery(query)

	items, total, err := s.repo.List(ctx, strings.TrimSpace(query.Keyword), query.GroupID, query.Status, query.Page, query.PageSize)
	if err != nil {
		return nil, 0, err
	}

	result := make([]dto.TagResponse, 0, len(items))
	for _, item := range items {
		result = append(result, dto.TagResponse{
			ID:        item.ID,
			GroupID:   item.GroupID,
			Name:      item.Name,
			Color:     item.Color,
			Status:    item.Status,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		})
	}

	return result, total, nil
}

func (s *tagService) ListGrouped(ctx context.Context, status string) ([]dto.TagGroupWithTagsResponse, error) {
	if _, err := currentUserID(ctx); err != nil {
		return nil, err
	}

	status = normalizeStatusFilter(status)
	if status == "" {
		status = statusActive
	}
	if err := validateActiveDisabledStatus(status, "标签状态"); err != nil {
		return nil, err
	}

	groups, err := s.groupRepo.ListAll(ctx, status)
	if err != nil {
		return nil, err
	}

	groupIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
	}

	tags, err := s.repo.ListByGroupIDs(ctx, groupIDs, status)
	if err != nil {
		return nil, err
	}

	tagsByGroupID := make(map[int64][]dto.TagResponse, len(groups))
	for _, tag := range tags {
		if tag.GroupID == nil {
			continue
		}
		tagsByGroupID[*tag.GroupID] = append(tagsByGroupID[*tag.GroupID], dto.TagResponse{
			ID:        tag.ID,
			GroupID:   tag.GroupID,
			Name:      tag.Name,
			Color:     tag.Color,
			Status:    tag.Status,
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt,
		})
	}

	result := make([]dto.TagGroupWithTagsResponse, 0, len(groups))
	for _, group := range groups {
		groupTags := tagsByGroupID[group.ID]
		if groupTags == nil {
			groupTags = []dto.TagResponse{}
		}

		result = append(result, dto.TagGroupWithTagsResponse{
			ID:          group.ID,
			Name:        group.Name,
			Description: group.Description,
			Status:      group.Status,
			Tags:        groupTags,
		})
	}

	return result, nil
}

func normalizeTagQuery(query dto.TagQuery) dto.TagQuery {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}
	query.Status = normalizeStatusFilter(query.Status)

	return query
}
