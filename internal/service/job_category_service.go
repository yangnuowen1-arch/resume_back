package service

import (
	"context"
	"errors"
	"strings"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
)

type JobCategoryService interface {
	Create(ctx context.Context, req dto.CreateJobCategoryRequest) (int64, error)
	Update(ctx context.Context, id int64, req dto.UpdateJobCategoryRequest) error
	List(ctx context.Context, query dto.JobCategoryQuery) ([]dto.JobCategoryResponse, int64, error)
}

// jobCategoryService 是 JobCategoryService 接口的具体实现，内部通过 repo 操作岗位分类数据。
type jobCategoryService struct {
	repo repository.JobCategoryRepository
}

// NewJobCategoryService 把外部创建好的 repository 注入 service，handler 只需要依赖返回的接口。
func NewJobCategoryService(repo repository.JobCategoryRepository) JobCategoryService {
	return &jobCategoryService{
		repo: repo,
	}
}

func (s *jobCategoryService) Create(
	ctx context.Context,
	req dto.CreateJobCategoryRequest,
) (int64, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return 0, err
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = trimOptionalString(req.Description)
	req.Status = normalizeActiveDisabledStatus(req.Status)
	if req.Name == "" {
		return 0, errors.New("岗位分类名称不能为空")
	}
	if err := validateActiveDisabledStatus(req.Status, "岗位分类状态"); err != nil {
		return 0, err
	}

	exists, err := s.repo.ExistsByName(ctx, req.Name)
	if err != nil {
		return 0, err
	}

	if exists {
		return 0, errors.New("岗位分类名称已存在")
	}

	// 取局部变量地址赋给 CreatedBy，避免直接取结构体字段地址带来的可读性问题。
	category := &model.JobCategory{
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
		CreatedBy:   &userID,
	}

	if err := s.repo.Create(ctx, category); err != nil {
		return 0, err
	}

	return category.ID, nil
}

func (s *jobCategoryService) Update(ctx context.Context, id int64, req dto.UpdateJobCategoryRequest) error {
	if _, err := currentUserID(ctx); err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("岗位分类 ID 不合法")
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = trimOptionalString(req.Description)
	req.Status = normalizeActiveDisabledStatus(req.Status)
	if req.Name == "" {
		return errors.New("岗位分类名称不能为空")
	}
	if err := validateActiveDisabledStatus(req.Status, "岗位分类状态"); err != nil {
		return err
	}

	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return errors.New("岗位分类不存在")
	}

	exists, err := s.repo.ExistsByNameExceptID(ctx, req.Name, id)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("岗位分类名称已存在")
	}

	// 将清洗和校验后的请求数据组装成数据库模型，再交给 repository 执行更新。
	category := &model.JobCategory{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
	}

	return s.repo.Update(ctx, category)
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
	query.Status = normalizeStatusFilter(query.Status)

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
			Status:      item.Status,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	return result, total, nil
}
