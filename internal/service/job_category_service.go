package service

import (
	"context"
	"errors"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
)

type JobCategoryService interface {
	Create(ctx context.Context, req dto.CreateJobCategoryRequest, operatorID *int64) (int64, error)
	List(ctx context.Context, query dto.JobCategoryQuery) ([]dto.JobCategoryResponse, int64, error)
}

type jobCategoryService struct {
	repo repository.JobCategoryRepository
}

func NewJobCategoryService(repo repository.JobCategoryRepository) JobCategoryService {
	return &jobCategoryService{
		repo: repo,
	}
}

func (s *jobCategoryService) Create(
	ctx context.Context,
	req dto.CreateJobCategoryRequest,
	operatorID *int64,
) (int64, error) {
	exists, err := s.repo.ExistsByName(ctx, req.Name)
	if err != nil {
		return 0, err
	}

	if exists {
		return 0, errors.New("岗位分类名称已存在")
	}

	if req.ParentID != nil {
		_, err := s.repo.FindByID(ctx, *req.ParentID)
		if err != nil {
			return 0, errors.New("父级分类不存在")
		}
	}

	category := &model.JobCategory{
		Name:        req.Name,
		Description: req.Description,
		ParentID:    req.ParentID,
		SortOrder:   req.SortOrder,
		Status:      "active",
		CreatedBy:   operatorID,
	}

	if err := s.repo.Create(ctx, category); err != nil {
		return 0, err
	}

	return category.ID, nil
}

func (s *jobCategoryService) List(
	ctx context.Context,
	query dto.JobCategoryQuery,
) ([]dto.JobCategoryResponse, int64, error) {
	if query.Page < 1 {
		query.Page = 1
	}

	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}

	items, total, err := s.repo.List(
		ctx,
		query.Keyword,
		query.Status,
		query.Page,
		query.PageSize,
	)

	if err != nil {
		return nil, 0, err
	}

	//创建一个空的 DTO 列表，准备装转换后的数据
	//[]dto.JobCategoryResponse — 列表类型
	//0 — 当前长度为 0，还没有数据
	//len(items) — 预先分配 items 同等大小的空间，避免后面 append 时频繁扩容
	result := make([]dto.JobCategoryResponse, 0, len(items))

	for _, item := range items {
		result = append(result, dto.JobCategoryResponse{
			ID:          item.ID,
			Name:        item.Name,
			Description: item.Description,
			ParentID:    item.ParentID,
			SortOrder:   item.SortOrder,
			Status:      item.Status,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	return result, total, nil
}
