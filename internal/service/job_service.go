package service

import (
	"context"
	"errors"
	"strings"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
)

type JobService interface {
	Create(ctx context.Context, req dto.CreateJobRequest) (int64, error)
	Update(ctx context.Context, id int64, req dto.UpdateJobRequest) error
	List(ctx context.Context, query dto.JobQuery) ([]dto.JobResponse, int64, error)
	BindTags(ctx context.Context, jobID int64, req dto.BindJobTagsRequest) error
	AssignMember(ctx context.Context, jobID int64, req dto.AssignJobMemberRequest) (int64, error)
}

type jobService struct {
	repo repository.JobRepository
}

func NewJobService(repo repository.JobRepository) JobService {
	return &jobService{
		repo: repo,
	}
}

func (s *jobService) Create(ctx context.Context, req dto.CreateJobRequest) (int64, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return 0, err
	}

	normalizeCreateJobRequest(&req)
	if req.Title == "" {
		return 0, errors.New("岗位名称不能为空")
	}
	if req.Headcount < 1 {
		req.Headcount = 1
	}
	if req.SalaryMin != nil && req.SalaryMax != nil && *req.SalaryMin > *req.SalaryMax {
		return 0, errors.New("薪资下限不能大于薪资上限")
	}
	if req.ExperienceMin != nil && req.ExperienceMax != nil && *req.ExperienceMin > *req.ExperienceMax {
		return 0, errors.New("经验下限不能大于经验上限")
	}

	if req.CategoryID != nil {
		exists, err := s.repo.CategoryExists(ctx, *req.CategoryID)
		if err != nil {
			return 0, err
		}
		if !exists {
			return 0, errors.New("岗位分类不存在")
		}
	}

	if req.OwnerUserID == nil {
		req.OwnerUserID = &userID
	} else {
		exists, err := s.repo.UserExists(ctx, *req.OwnerUserID)
		if err != nil {
			return 0, err
		}
		if !exists {
			return 0, errors.New("岗位负责人不存在")
		}
	}

	job := &model.Job{
		CategoryID:       req.CategoryID,
		Title:            req.Title,
		Department:       req.Department,
		Headcount:        req.Headcount,
		WorkLocation:     req.WorkLocation,
		WorkType:         req.WorkType,
		EmploymentType:   req.EmploymentType,
		SalaryMin:        req.SalaryMin,
		SalaryMax:        req.SalaryMax,
		SalaryMonths:     req.SalaryMonths,
		ExperienceMin:    req.ExperienceMin,
		ExperienceMax:    req.ExperienceMax,
		EducationLevel:   req.EducationLevel,
		Description:      req.Description,
		Responsibilities: req.Responsibilities,
		Requirements:     req.Requirements,
		BonusPoints:      req.BonusPoints,
		Status:           req.Status,
		Priority:         req.Priority,
		OwnerUserID:      req.OwnerUserID,
		CreatedBy:        &userID,
	}
	if err := s.repo.Create(ctx, job); err != nil {
		return 0, err
	}

	return job.ID, nil
}

func (s *jobService) Update(ctx context.Context, id int64, req dto.UpdateJobRequest) error {
	if _, err := currentUserID(ctx); err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("岗位 ID 不合法")
	}

	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return errors.New("岗位不存在")
	}

	// 更新岗位和创建岗位的大部分字段规则一致，这里转成 CreateJobRequest 复用同一套清洗逻辑。
	createReq := dto.CreateJobRequest{
		CategoryID:       req.CategoryID,
		Title:            req.Title,
		Department:       req.Department,
		Headcount:        req.Headcount,
		WorkLocation:     req.WorkLocation,
		WorkType:         req.WorkType,
		EmploymentType:   req.EmploymentType,
		SalaryMin:        req.SalaryMin,
		SalaryMax:        req.SalaryMax,
		SalaryMonths:     req.SalaryMonths,
		ExperienceMin:    req.ExperienceMin,
		ExperienceMax:    req.ExperienceMax,
		EducationLevel:   req.EducationLevel,
		Description:      req.Description,
		Responsibilities: req.Responsibilities,
		Requirements:     req.Requirements,
		BonusPoints:      req.BonusPoints,
		Status:           req.Status,
		Priority:         req.Priority,
		OwnerUserID:      req.OwnerUserID,
	}
	normalizeCreateJobRequest(&createReq)
	if createReq.Title == "" {
		return errors.New("岗位名称不能为空")
	}
	if createReq.Status == "" {
		return errors.New("岗位状态不能为空")
	}
	if createReq.Priority == "" {
		return errors.New("岗位优先级不能为空")
	}
	if createReq.Headcount < 1 {
		createReq.Headcount = 1
	}
	if createReq.SalaryMin != nil && createReq.SalaryMax != nil && *createReq.SalaryMin > *createReq.SalaryMax {
		return errors.New("薪资下限不能大于薪资上限")
	}
	if createReq.ExperienceMin != nil && createReq.ExperienceMax != nil && *createReq.ExperienceMin > *createReq.ExperienceMax {
		return errors.New("经验下限不能大于经验上限")
	}

	if createReq.CategoryID != nil {
		exists, err := s.repo.CategoryExists(ctx, *createReq.CategoryID)
		if err != nil {
			return err
		}
		if !exists {
			return errors.New("岗位分类不存在")
		}
	}

	if createReq.OwnerUserID == nil {
		createReq.OwnerUserID = existing.OwnerUserID
	} else {
		exists, err := s.repo.UserExists(ctx, *createReq.OwnerUserID)
		if err != nil {
			return err
		}
		if !exists {
			return errors.New("岗位负责人不存在")
		}
	}

	job := &model.Job{
		ID:               id,
		CategoryID:       createReq.CategoryID,
		Title:            createReq.Title,
		Department:       createReq.Department,
		Headcount:        createReq.Headcount,
		WorkLocation:     createReq.WorkLocation,
		WorkType:         createReq.WorkType,
		EmploymentType:   createReq.EmploymentType,
		SalaryMin:        createReq.SalaryMin,
		SalaryMax:        createReq.SalaryMax,
		SalaryMonths:     createReq.SalaryMonths,
		ExperienceMin:    createReq.ExperienceMin,
		ExperienceMax:    createReq.ExperienceMax,
		EducationLevel:   createReq.EducationLevel,
		Description:      createReq.Description,
		Responsibilities: createReq.Responsibilities,
		Requirements:     createReq.Requirements,
		BonusPoints:      createReq.BonusPoints,
		Status:           createReq.Status,
		Priority:         createReq.Priority,
		OwnerUserID:      createReq.OwnerUserID,
	}

	return s.repo.Update(ctx, job)
}

func (s *jobService) List(ctx context.Context, query dto.JobQuery) ([]dto.JobResponse, int64, error) {
	query = normalizeJobQuery(query)

	items, total, err := s.repo.List(ctx, strings.TrimSpace(query.Keyword), query.CategoryID, query.Status, query.Page, query.PageSize)
	if err != nil {
		return nil, 0, err
	}

	result := make([]dto.JobResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toJobResponse(item))
	}

	return result, total, nil
}

func (s *jobService) BindTags(ctx context.Context, jobID int64, req dto.BindJobTagsRequest) error {
	if _, err := currentUserID(ctx); err != nil {
		return err
	}

	if jobID <= 0 {
		return errors.New("岗位 ID 不合法")
	}
	if _, err := s.repo.FindByID(ctx, jobID); err != nil {
		return errors.New("岗位不存在")
	}

	tagIDs := uniquePositiveIDs(req.TagIDs)
	if len(tagIDs) != len(req.TagIDs) {
		return errors.New("标签 ID 不能为空或重复")
	}
	if len(tagIDs) > 0 {
		count, err := s.repo.CountTagsByIDs(ctx, tagIDs)
		if err != nil {
			return err
		}
		if count != int64(len(tagIDs)) {
			return errors.New("部分标签不存在")
		}
	}

	return s.repo.BindTags(ctx, jobID, tagIDs)
}

func (s *jobService) AssignMember(ctx context.Context, jobID int64, req dto.AssignJobMemberRequest) (int64, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return 0, err
	}

	req.MemberRole = strings.TrimSpace(req.MemberRole)
	if jobID <= 0 {
		return 0, errors.New("岗位 ID 不合法")
	}
	if req.UserID <= 0 {
		return 0, errors.New("成员用户 ID 不合法")
	}
	if req.MemberRole == "" {
		return 0, errors.New("成员角色不能为空")
	}

	if _, err := s.repo.FindByID(ctx, jobID); err != nil {
		return 0, errors.New("岗位不存在")
	}

	exists, err := s.repo.UserExists(ctx, req.UserID)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, errors.New("成员用户不存在")
	}

	member := &model.JobMember{
		JobID:      jobID,
		UserID:     req.UserID,
		MemberRole: req.MemberRole,
		CreatedBy:  &userID,
	}
	if err := s.repo.AssignMember(ctx, member); err != nil {
		return 0, err
	}

	return member.ID, nil
}

// normalizeCreateJobRequest 统一清洗岗位创建/更新参数。
// 岗位字段比较多，把去空格和默认值集中在这里，避免 Create 和 Update 重复写同样逻辑。
func normalizeCreateJobRequest(req *dto.CreateJobRequest) {
	req.Title = strings.TrimSpace(req.Title)
	req.Department = trimOptionalString(req.Department)
	req.WorkLocation = trimOptionalString(req.WorkLocation)
	req.WorkType = trimOptionalString(req.WorkType)
	req.EmploymentType = trimOptionalString(req.EmploymentType)
	req.EducationLevel = trimOptionalString(req.EducationLevel)
	req.Description = trimOptionalString(req.Description)
	req.Responsibilities = trimOptionalString(req.Responsibilities)
	req.Requirements = trimOptionalString(req.Requirements)
	req.BonusPoints = trimOptionalString(req.BonusPoints)
	req.Status = strings.TrimSpace(req.Status)
	req.Priority = strings.TrimSpace(req.Priority)

	if req.Status == "" {
		req.Status = "draft"
	}
	if req.Priority == "" {
		req.Priority = "normal"
	}
}

// normalizeJobQuery 统一处理岗位列表查询参数。
// 主要负责修正分页参数，并给状态查询设置默认值，避免 repository 收到不合理的分页条件。
func normalizeJobQuery(query dto.JobQuery) dto.JobQuery {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}
	if query.Status == "" {
		query.Status = "draft"
	}

	return query
}

// uniquePositiveIDs 过滤掉非法 ID，并去掉重复 ID。
// 绑定标签前先整理 tag_ids，方便判断前端是否传了空 ID、负数 ID 或重复 ID。
func uniquePositiveIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		result = append(result, id)
	}

	return result
}

// toJobResponse 把数据库模型转换成返回给前端的 DTO。
// 这样 handler 不直接暴露 model，也能把“数据库字段”和“接口返回字段”的转换集中管理。
func toJobResponse(job *model.Job) dto.JobResponse {
	return dto.JobResponse{
		ID:               job.ID,
		CategoryID:       job.CategoryID,
		Title:            job.Title,
		Department:       job.Department,
		Headcount:        job.Headcount,
		WorkLocation:     job.WorkLocation,
		WorkType:         job.WorkType,
		EmploymentType:   job.EmploymentType,
		SalaryMin:        job.SalaryMin,
		SalaryMax:        job.SalaryMax,
		SalaryMonths:     job.SalaryMonths,
		ExperienceMin:    job.ExperienceMin,
		ExperienceMax:    job.ExperienceMax,
		EducationLevel:   job.EducationLevel,
		Description:      job.Description,
		Responsibilities: job.Responsibilities,
		Requirements:     job.Requirements,
		BonusPoints:      job.BonusPoints,
		Status:           job.Status,
		Priority:         job.Priority,
		OwnerUserID:      job.OwnerUserID,
		CreatedBy:        job.CreatedBy,
		PublishedAt:      job.PublishedAt,
		ClosedAt:         job.ClosedAt,
		CreatedAt:        job.CreatedAt,
		UpdatedAt:        job.UpdatedAt,
	}
}
