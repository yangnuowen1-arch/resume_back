package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
	"gorm.io/datatypes"
)

type JobService interface {
	Create(ctx context.Context, req dto.CreateJobRequest) (int64, error)
	Update(ctx context.Context, id int64, req dto.UpdateJobRequest) error
	Get(ctx context.Context, id int64) (dto.JobResponse, error)
	List(ctx context.Context, query dto.JobQuery) ([]dto.JobResponse, int64, error)
	Delete(ctx context.Context, id int64) error
	BindTags(ctx context.Context, jobID int64, req dto.BindJobTagsRequest) error
	ListTags(ctx context.Context, jobID int64) ([]dto.JobTagResponse, error)
	AssignMember(ctx context.Context, jobID int64, req dto.AssignJobMemberRequest) (int64, error)
	ListMembers(ctx context.Context, jobID int64) ([]dto.JobMemberDetailResponse, error)
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

	tagIDs, err := s.normalizeAndValidateJobTagIDs(ctx, req.TagIDs)
	if err != nil {
		return 0, err
	}
	dynamicFields, err := normalizeJobDynamicFields(req.DynamicFields)
	if err != nil {
		return 0, err
	}

	job := &model.Job{
		CategoryID:       req.CategoryID,
		Title:            req.Title,
		Headcount:        req.Headcount,
		SalaryMin:        req.SalaryMin,
		SalaryMax:        req.SalaryMax,
		SalaryMonths:     req.SalaryMonths,
		ExperienceMin:    req.ExperienceMin,
		ExperienceMax:    req.ExperienceMax,
		Description:      req.Description,
		Responsibilities: req.Responsibilities,
		Requirements:     req.Requirements,
		BonusPoints:      req.BonusPoints,
		Status:           req.Status,
		Priority:         req.Priority,
		OwnerUserID:      req.OwnerUserID,
		CreatedBy:        &userID,
		DynamicFields:    dynamicFields,
	}
	if err := s.repo.CreateWithTags(ctx, job, tagIDs); err != nil {
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
		Headcount:        req.Headcount,
		SalaryMin:        req.SalaryMin,
		SalaryMax:        req.SalaryMax,
		SalaryMonths:     req.SalaryMonths,
		ExperienceMin:    req.ExperienceMin,
		ExperienceMax:    req.ExperienceMax,
		Description:      req.Description,
		Responsibilities: req.Responsibilities,
		Requirements:     req.Requirements,
		BonusPoints:      req.BonusPoints,
		Status:           req.Status,
		Priority:         req.Priority,
		OwnerUserID:      req.OwnerUserID,
		TagIDs:           req.TagIDs,
		DynamicFields:    req.DynamicFields,
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

	syncTags := req.TagIDs != nil
	tagIDs := []int64(nil)
	if syncTags {
		tagIDs, err = s.normalizeAndValidateJobTagIDs(ctx, req.TagIDs)
		if err != nil {
			return err
		}
	}

	dynamicFields := existing.DynamicFields
	if req.DynamicFields != nil {
		dynamicFields, err = normalizeJobDynamicFields(req.DynamicFields)
		if err != nil {
			return err
		}
	}

	job := &model.Job{
		ID:               id,
		CategoryID:       createReq.CategoryID,
		Title:            createReq.Title,
		Headcount:        createReq.Headcount,
		SalaryMin:        createReq.SalaryMin,
		SalaryMax:        createReq.SalaryMax,
		SalaryMonths:     createReq.SalaryMonths,
		ExperienceMin:    createReq.ExperienceMin,
		ExperienceMax:    createReq.ExperienceMax,
		Description:      createReq.Description,
		Responsibilities: createReq.Responsibilities,
		Requirements:     createReq.Requirements,
		BonusPoints:      createReq.BonusPoints,
		Status:           createReq.Status,
		Priority:         createReq.Priority,
		OwnerUserID:      createReq.OwnerUserID,
		DynamicFields:    dynamicFields,
	}

	return s.repo.UpdateWithTags(ctx, job, tagIDs, syncTags)
}

func (s *jobService) Get(ctx context.Context, id int64) (dto.JobResponse, error) {
	if id <= 0 {
		return dto.JobResponse{}, errors.New("岗位 ID 不合法")
	}

	job, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return dto.JobResponse{}, errors.New("岗位不存在")
	}

	tags, err := s.repo.ListTags(ctx, id)
	if err != nil {
		return dto.JobResponse{}, err
	}

	tagResponses := make([]dto.JobTagResponse, 0, len(tags))
	for _, item := range tags {
		tagResponses = append(tagResponses, toJobTagResponse(item))
	}

	return toJobResponse(job, tagResponses), nil
}

func (s *jobService) List(ctx context.Context, query dto.JobQuery) ([]dto.JobResponse, int64, error) {
	query = normalizeJobQuery(query)

	items, total, err := s.repo.List(ctx, strings.TrimSpace(query.Keyword), query.CategoryID, query.Status, query.Page, query.PageSize)
	if err != nil {
		return nil, 0, err
	}

	jobIDs := make([]int64, 0, len(items))
	for _, item := range items {
		jobIDs = append(jobIDs, item.ID)
	}

	tags, err := s.repo.ListTagsByJobIDs(ctx, jobIDs)
	if err != nil {
		return nil, 0, err
	}

	tagsByJobID := make(map[int64][]dto.JobTagResponse, len(items))
	for _, item := range tags {
		tagsByJobID[item.JobID] = append(tagsByJobID[item.JobID], toJobTagResponse(item))
	}

	result := make([]dto.JobResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toJobResponse(&item, tagsByJobID[item.ID]))
	}

	return result, total, nil
}

func (s *jobService) Delete(ctx context.Context, id int64) error {
	if _, err := currentUserID(ctx); err != nil {
		return err
	}

	if id <= 0 {
		return errors.New("岗位 ID 不合法")
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return errors.New("岗位不存在")
	}

	count, err := s.repo.CountApplicationsByJobID(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("岗位下存在投递记录，不能删除")
	}

	return s.repo.Delete(ctx, id)
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

func (s *jobService) ListTags(ctx context.Context, jobID int64) ([]dto.JobTagResponse, error) {
	if _, err := currentUserID(ctx); err != nil {
		return nil, err
	}

	if jobID <= 0 {
		return nil, errors.New("岗位 ID 不合法")
	}
	if _, err := s.repo.FindByID(ctx, jobID); err != nil {
		return nil, errors.New("岗位不存在")
	}

	items, err := s.repo.ListTags(ctx, jobID)
	if err != nil {
		return nil, err
	}

	result := make([]dto.JobTagResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toJobTagResponse(item))
	}

	return result, nil
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

func (s *jobService) ListMembers(ctx context.Context, jobID int64) ([]dto.JobMemberDetailResponse, error) {
	if _, err := currentUserID(ctx); err != nil {
		return nil, err
	}

	if jobID <= 0 {
		return nil, errors.New("岗位 ID 不合法")
	}
	if _, err := s.repo.FindByID(ctx, jobID); err != nil {
		return nil, errors.New("岗位不存在")
	}

	items, err := s.repo.ListMembers(ctx, jobID)
	if err != nil {
		return nil, err
	}

	result := make([]dto.JobMemberDetailResponse, 0, len(items))
	for _, item := range items {
		result = append(result, dto.JobMemberDetailResponse{
			ID:         item.ID,
			JobID:      item.JobID,
			UserID:     item.UserID,
			Username:   item.Username,
			RealName:   item.RealName,
			Email:      item.Email,
			UserStatus: item.UserStatus,
			MemberRole: item.MemberRole,
			CreatedBy:  item.CreatedBy,
			CreatedAt:  item.CreatedAt,
		})
	}

	return result, nil
}

// normalizeCreateJobRequest 统一清洗岗位创建/更新参数。
// 岗位字段比较多，把去空格和默认值集中在这里，避免 Create 和 Update 重复写同样逻辑。
func normalizeCreateJobRequest(req *dto.CreateJobRequest) {
	req.Title = strings.TrimSpace(req.Title)
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

func (s *jobService) normalizeAndValidateJobTagIDs(ctx context.Context, ids []int64) ([]int64, error) {
	tagIDs := uniquePositiveIDs(ids)
	if len(tagIDs) != len(ids) {
		return nil, errors.New("标签 ID 不能为空或重复")
	}
	if len(tagIDs) == 0 {
		return tagIDs, nil
	}

	count, err := s.repo.CountTagsByIDs(ctx, tagIDs)
	if err != nil {
		return nil, err
	}
	if count != int64(len(tagIDs)) {
		return nil, errors.New("部分标签不存在")
	}

	return tagIDs, nil
}

// normalizeJobQuery 统一处理岗位列表查询参数。
// 主要负责修正分页参数，并让空状态或 all 表示不按状态过滤。
func normalizeJobQuery(query dto.JobQuery) dto.JobQuery {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 200 {
		query.PageSize = 20
	}
	query.Status = strings.TrimSpace(query.Status)
	if query.Status == "all" {
		query.Status = ""
	}

	return query
}

func normalizeJobDynamicFields(fields map[string]interface{}) (datatypes.JSONMap, error) {
	if fields == nil {
		return datatypes.JSONMap{}, nil
	}

	normalized := make(datatypes.JSONMap, len(fields))
	for key, value := range fields {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, errors.New("动态字段名称不能为空")
		}
		normalized[key] = value
	}

	if _, err := json.Marshal(normalized); err != nil {
		return nil, errors.New("动态字段必须是合法 JSON")
	}

	return normalized, nil
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
func toJobResponse(job *repository.JobDetailItem, tags []dto.JobTagResponse) dto.JobResponse {
	if tags == nil {
		tags = []dto.JobTagResponse{}
	}

	return dto.JobResponse{
		ID:               job.ID,
		CategoryID:       job.CategoryID,
		Title:            job.Title,
		Headcount:        job.Headcount,
		SalaryMin:        job.SalaryMin,
		SalaryMax:        job.SalaryMax,
		SalaryMonths:     job.SalaryMonths,
		ExperienceMin:    job.ExperienceMin,
		ExperienceMax:    job.ExperienceMax,
		Description:      job.Description,
		Responsibilities: job.Responsibilities,
		Requirements:     job.Requirements,
		BonusPoints:      job.BonusPoints,
		Status:           job.Status,
		Priority:         job.Priority,
		OwnerUserID:      job.OwnerUserID,
		OwnerRealName:    job.OwnerRealName,
		CreatedBy:        job.CreatedBy,
		CreatorRealName:  job.CreatorRealName,
		PublishedAt:      job.PublishedAt,
		ClosedAt:         job.ClosedAt,
		CreatedAt:        job.CreatedAt,
		UpdatedAt:        job.UpdatedAt,
		Tags:             tags,
		DynamicFields:    toJobDynamicFields(job.DynamicFields),
	}
}

func toJobDynamicFields(fields datatypes.JSONMap) map[string]interface{} {
	if fields == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}(fields)
}

func toJobTagResponse(item repository.JobTagWithTag) dto.JobTagResponse {
	return dto.JobTagResponse{
		JobID:     item.JobID,
		TagID:     item.TagID,
		GroupID:   item.GroupID,
		Name:      item.Name,
		Color:     item.Color,
		Status:    item.Status,
		CreatedAt: item.CreatedAt,
	}
}
