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
	CreateWithResume(ctx context.Context, req dto.CreateCandidateRequest, resumeReq dto.UploadResumeRequest) (int64, int64, error)
	UploadResume(ctx context.Context, id int64, resumeReq dto.UploadResumeRequest) (int64, error)
	BatchAnalyze(ctx context.Context, req dto.BatchAnalyzeCandidatesRequest) (dto.BatchAnalyzeCandidatesResponse, error)
	Update(ctx context.Context, id int64, req dto.UpdateCandidateRequest) error
	UpdateWithResume(ctx context.Context, id int64, req dto.UpdateCandidateRequest, resumeReq dto.UploadResumeRequest) (int64, error)
	List(ctx context.Context, query dto.CandidateQuery) ([]dto.CandidateResponse, int64, error)
	StatusOptions() []dto.CandidateStatusOption
}

type candidateService struct {
	repo repository.CandidateRepository
}

const (
	CandidateGenderMale   = "男"
	CandidateGenderFemale = "女"

	CandidateSourceBoss  = "boss"
	CandidateSourceEmail = "email"

	CandidateEducationCollege  = "专科"
	CandidateEducationBachelor = "本科"
	CandidateEducationMaster   = "硕士"
	CandidateEducationDoctor   = "博士"
)

var (
	candidateGenderValues = map[string]struct{}{
		CandidateGenderMale:   {},
		CandidateGenderFemale: {},
	}
	candidateSourceValues = map[string]struct{}{
		CandidateSourceBoss:  {},
		CandidateSourceEmail: {},
	}
	candidateEducationValues = map[string]struct{}{
		CandidateEducationCollege:  {},
		CandidateEducationBachelor: {},
		CandidateEducationMaster:   {},
		CandidateEducationDoctor:   {},
	}
)

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
	if err := validateCandidateStatus(req.Status); err != nil {
		return 0, err
	}
	if err := validateCandidateEnums(req); err != nil {
		return 0, err
	}
	if err := s.normalizeAndValidateCandidatePosition(ctx, &req); err != nil {
		return 0, err
	}

	name := req.Name
	candidate := &model.Candidate{
		Name:                    &name,
		Email:                   req.Email,
		Phone:                   req.Phone,
		Gender:                  req.Gender,
		CurrentCompany:          req.CurrentCompany,
		PositionCategoryID:      req.PositionCategoryID,
		CurrentJobID:            req.CurrentJobID,
		CurrentPosition:         req.CurrentPosition,
		CurrentPositionCategory: req.CurrentPositionCategory,
		YearsOfExperience:       req.YearsOfExperience,
		HighestEducation:        req.HighestEducation,
		School:                  req.School,
		Major:                   req.Major,
		Location:                req.Location,
		Source:                  req.Source,
		Status:                  req.Status,
	}
	if err := s.repo.Create(ctx, candidate); err != nil {
		return 0, err
	}

	return candidate.ID, nil
}

func (s *candidateService) CreateWithResume(ctx context.Context, req dto.CreateCandidateRequest, resumeReq dto.UploadResumeRequest) (int64, int64, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return 0, 0, err
	}

	normalizeCreateCandidateRequest(&req)
	if req.Status == CandidateStatusNew {
		req.Status = CandidateStatusPendingReview
	}
	if err := validateCandidate(req.Name, req.YearsOfExperience); err != nil {
		return 0, 0, err
	}
	if err := validateCandidateStatus(req.Status); err != nil {
		return 0, 0, err
	}
	if err := validateCandidateEnums(req); err != nil {
		return 0, 0, err
	}
	if err := s.normalizeAndValidateCandidatePosition(ctx, &req); err != nil {
		return 0, 0, err
	}

	if err := normalizeAndValidateUploadResumeRequest(&resumeReq); err != nil {
		return 0, 0, err
	}

	name := req.Name
	candidate := &model.Candidate{
		Name:                    &name,
		Email:                   req.Email,
		Phone:                   req.Phone,
		Gender:                  req.Gender,
		CurrentCompany:          req.CurrentCompany,
		PositionCategoryID:      req.PositionCategoryID,
		CurrentJobID:            req.CurrentJobID,
		CurrentPosition:         req.CurrentPosition,
		CurrentPositionCategory: req.CurrentPositionCategory,
		YearsOfExperience:       req.YearsOfExperience,
		HighestEducation:        req.HighestEducation,
		School:                  req.School,
		Major:                   req.Major,
		Location:                req.Location,
		Source:                  req.Source,
		Status:                  req.Status,
	}
	resume := &model.Resume{
		OriginalFilename: &resumeReq.OriginalFilename,
		FileKey:          &resumeReq.FileKey,
		FileURL:          &resumeReq.FileURL,
		FileType:         &resumeReq.FileType,
		FileSize:         &resumeReq.FileSize,
		RawText:          resumeReq.RawText,
		ParseStatus:      initialResumeParseStatus(resumeReq.RawText),
		Language:         resumeReq.Language,
		UploadBy:         &userID,
	}
	if err := s.repo.CreateWithResume(ctx, candidate, resume); err != nil {
		return 0, 0, err
	}

	return candidate.ID, resume.ID, nil
}

func (s *candidateService) UploadResume(ctx context.Context, id int64, resumeReq dto.UploadResumeRequest) (int64, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return 0, err
	}

	if id <= 0 {
		return 0, errors.New("候选人 ID 不合法")
	}

	if err := normalizeAndValidateUploadResumeRequest(&resumeReq); err != nil {
		return 0, err
	}

	resume := &model.Resume{
		OriginalFilename: &resumeReq.OriginalFilename,
		FileKey:          &resumeReq.FileKey,
		FileURL:          &resumeReq.FileURL,
		FileType:         &resumeReq.FileType,
		FileSize:         &resumeReq.FileSize,
		RawText:          resumeReq.RawText,
		ParseStatus:      initialResumeParseStatus(resumeReq.RawText),
		Language:         resumeReq.Language,
		UploadBy:         &userID,
	}
	if err := s.repo.CreateResumeForCandidate(ctx, id, resume, CandidateStatusPendingReview); err != nil {
		return 0, errors.New("候选人不存在或上传简历失败")
	}

	return resume.ID, nil
}

func (s *candidateService) BatchAnalyze(ctx context.Context, req dto.BatchAnalyzeCandidatesRequest) (dto.BatchAnalyzeCandidatesResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return dto.BatchAnalyzeCandidatesResponse{}, err
	}

	candidateIDs := uniquePositiveIDs(req.CandidateIDs)
	if len(candidateIDs) == 0 || len(candidateIDs) != len(req.CandidateIDs) {
		return dto.BatchAnalyzeCandidatesResponse{}, errors.New("候选人 ID 不能为空或重复")
	}
	if len(candidateIDs) > 100 {
		return dto.BatchAnalyzeCandidatesResponse{}, errors.New("单次最多分析 100 个候选人")
	}
	if req.JobID != nil && *req.JobID <= 0 {
		return dto.BatchAnalyzeCandidatesResponse{}, errors.New("岗位 ID 不合法")
	}

	items := make([]dto.BatchAnalyzeCandidateResult, 0, len(candidateIDs))
	queued := 0
	for _, candidateID := range candidateIDs {
		result := s.repo.EnqueueScreening(ctx, candidateID, req.JobID, userID, CandidateStatusEvaluating)
		if result.Status == "queued" {
			queued++
		}
		items = append(items, dto.BatchAnalyzeCandidateResult{
			CandidateID:   result.CandidateID,
			ResumeID:      result.ResumeID,
			ApplicationID: result.ApplicationID,
			ParseStatus:   result.ParseStatus,
			Status:        result.Status,
			Message:       result.Message,
		})
	}

	return dto.BatchAnalyzeCandidatesResponse{
		Total:  len(candidateIDs),
		Queued: queued,
		Failed: len(candidateIDs) - queued,
		Items:  items,
	}, nil
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
	if err := validateCandidateStatus(createReq.Status); err != nil {
		return err
	}
	if err := validateCandidateEnums(createReq); err != nil {
		return err
	}
	if err := s.normalizeAndValidateCandidatePosition(ctx, &createReq); err != nil {
		return err
	}

	name := createReq.Name
	candidate := &model.Candidate{
		ID:                      id,
		Name:                    &name,
		Email:                   createReq.Email,
		Phone:                   createReq.Phone,
		Gender:                  createReq.Gender,
		CurrentCompany:          createReq.CurrentCompany,
		PositionCategoryID:      createReq.PositionCategoryID,
		CurrentJobID:            createReq.CurrentJobID,
		CurrentPosition:         createReq.CurrentPosition,
		CurrentPositionCategory: createReq.CurrentPositionCategory,
		YearsOfExperience:       createReq.YearsOfExperience,
		HighestEducation:        createReq.HighestEducation,
		School:                  createReq.School,
		Major:                   createReq.Major,
		Location:                createReq.Location,
		Source:                  createReq.Source,
		Status:                  createReq.Status,
	}

	return s.repo.Update(ctx, candidate)
}

func (s *candidateService) UpdateWithResume(ctx context.Context, id int64, req dto.UpdateCandidateRequest, resumeReq dto.UploadResumeRequest) (int64, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return 0, err
	}

	if id <= 0 {
		return 0, errors.New("候选人 ID 不合法")
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return 0, errors.New("候选人不存在")
	}

	createReq := dto.CreateCandidateRequest(req)
	normalizeCreateCandidateRequest(&createReq)
	if err := validateCandidate(createReq.Name, createReq.YearsOfExperience); err != nil {
		return 0, err
	}
	if err := validateCandidateStatus(createReq.Status); err != nil {
		return 0, err
	}
	if err := validateCandidateEnums(createReq); err != nil {
		return 0, err
	}
	if err := s.normalizeAndValidateCandidatePosition(ctx, &createReq); err != nil {
		return 0, err
	}
	if err := normalizeAndValidateUploadResumeRequest(&resumeReq); err != nil {
		return 0, err
	}

	name := createReq.Name
	candidate := &model.Candidate{
		ID:                      id,
		Name:                    &name,
		Email:                   createReq.Email,
		Phone:                   createReq.Phone,
		Gender:                  createReq.Gender,
		CurrentCompany:          createReq.CurrentCompany,
		PositionCategoryID:      createReq.PositionCategoryID,
		CurrentJobID:            createReq.CurrentJobID,
		CurrentPosition:         createReq.CurrentPosition,
		CurrentPositionCategory: createReq.CurrentPositionCategory,
		YearsOfExperience:       createReq.YearsOfExperience,
		HighestEducation:        createReq.HighestEducation,
		School:                  createReq.School,
		Major:                   createReq.Major,
		Location:                createReq.Location,
		Source:                  createReq.Source,
		Status:                  CandidateStatusPendingReview,
	}
	resume := &model.Resume{
		OriginalFilename: &resumeReq.OriginalFilename,
		FileKey:          &resumeReq.FileKey,
		FileURL:          &resumeReq.FileURL,
		FileType:         &resumeReq.FileType,
		FileSize:         &resumeReq.FileSize,
		RawText:          resumeReq.RawText,
		ParseStatus:      initialResumeParseStatus(resumeReq.RawText),
		Language:         resumeReq.Language,
		UploadBy:         &userID,
	}
	if err := s.repo.UpdateWithResume(ctx, candidate, resume); err != nil {
		return 0, err
	}

	return resume.ID, nil
}

func (s *candidateService) List(ctx context.Context, query dto.CandidateQuery) ([]dto.CandidateResponse, int64, error) {
	query = normalizeCandidateQuery(query)

	source := normalizeCandidateSourceFilter(query.Source)
	if source != "" {
		if err := validateCandidateSourceValue(source); err != nil {
			return nil, 0, err
		}
	}

	status := normalizeStatusFilter(query.Status)
	if status != "" {
		if err := validateCandidateStatus(status); err != nil {
			return nil, 0, err
		}
	}

	items, total, err := s.repo.List(ctx, repository.CandidateListFilter{
		Keyword:  strings.TrimSpace(query.Keyword),
		Source:   source,
		Status:   status,
		Page:     query.Page,
		PageSize: query.PageSize,
	})
	if err != nil {
		return nil, 0, err
	}

	result := make([]dto.CandidateResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toCandidateListResponse(item))
	}

	return result, total, nil
}

func (s *candidateService) StatusOptions() []dto.CandidateStatusOption {
	return []dto.CandidateStatusOption{
		{Value: CandidateStatusNew, Label: candidateStatusLabels[CandidateStatusNew]},
		{Value: CandidateStatusPendingReview, Label: candidateStatusLabels[CandidateStatusPendingReview]},
		{Value: CandidateStatusEvaluating, Label: candidateStatusLabels[CandidateStatusEvaluating]},
		{Value: CandidateStatusEvaluated, Label: candidateStatusLabels[CandidateStatusEvaluated]},
		{Value: CandidateStatusInterview, Label: candidateStatusLabels[CandidateStatusInterview]},
		{Value: CandidateStatusOffered, Label: candidateStatusLabels[CandidateStatusOffered]},
		{Value: CandidateStatusHired, Label: candidateStatusLabels[CandidateStatusHired]},
		{Value: CandidateStatusRejected, Label: candidateStatusLabels[CandidateStatusRejected]},
	}
}

func normalizeCreateCandidateRequest(req *dto.CreateCandidateRequest) {
	req.Name = strings.TrimSpace(req.Name)
	req.Email = trimOptionalString(req.Email)
	req.Phone = trimOptionalString(req.Phone)
	req.Gender = trimOptionalString(req.Gender)
	req.CurrentCompany = trimOptionalString(req.CurrentCompany)
	req.CurrentPosition = trimOptionalString(req.CurrentPosition)
	req.CurrentPositionCategory = trimOptionalString(req.CurrentPositionCategory)
	req.HighestEducation = trimOptionalString(req.HighestEducation)
	req.School = trimOptionalString(req.School)
	req.Major = trimOptionalString(req.Major)
	req.Location = trimOptionalString(req.Location)
	req.Source = normalizeOptionalCandidateSource(req.Source)
	req.Status = normalizeCandidateStatus(req.Status, CandidateStatusNew)
}

func normalizeAndValidateUploadResumeRequest(req *dto.UploadResumeRequest) error {
	req.OriginalFilename = strings.TrimSpace(req.OriginalFilename)
	req.FileKey = strings.TrimSpace(req.FileKey)
	req.FileURL = strings.TrimSpace(req.FileURL)
	req.FileType = strings.TrimSpace(req.FileType)
	req.RawText = trimOptionalString(req.RawText)
	req.Language = trimOptionalString(req.Language)
	if req.OriginalFilename == "" {
		return errors.New("原始文件名不能为空")
	}
	if req.FileKey == "" {
		return errors.New("简历文件 key 不能为空")
	}
	if req.FileURL == "" {
		return errors.New("简历文件地址不能为空")
	}
	if req.FileSize <= 0 {
		return errors.New("简历文件大小不合法")
	}

	return nil
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

func (s *candidateService) normalizeAndValidateCandidatePosition(ctx context.Context, req *dto.CreateCandidateRequest) error {
	if req.PositionCategoryID != nil {
		if *req.PositionCategoryID <= 0 {
			return errors.New("岗位分类 ID 不合法")
		}
		exists, err := s.repo.ActivePositionCategoryExists(ctx, *req.PositionCategoryID)
		if err != nil {
			return err
		}
		if !exists {
			return errors.New("岗位分类不存在或已禁用")
		}
	}

	if req.CurrentJobID == nil {
		return nil
	}
	if *req.CurrentJobID <= 0 {
		return errors.New("当前岗位 ID 不合法")
	}

	job, err := s.repo.FindJobSelectionByID(ctx, *req.CurrentJobID)
	if err != nil {
		return errors.New("当前岗位不存在")
	}
	if job.Status != "published" {
		return errors.New("当前岗位必须是已发布岗位")
	}

	if job.CategoryID != nil {
		if req.PositionCategoryID != nil && *req.PositionCategoryID != *job.CategoryID {
			return errors.New("岗位分类与当前岗位不匹配")
		}
		if req.PositionCategoryID == nil {
			req.PositionCategoryID = job.CategoryID
		}
	}

	if req.CurrentPosition == nil {
		title := job.Title
		req.CurrentPosition = &title
	}

	return nil
}

func validateCandidateEnums(req dto.CreateCandidateRequest) error {
	if err := validateCandidateGenderValue(optionalStringValue(req.Gender)); err != nil {
		return err
	}
	if err := validateCandidateSourceValue(optionalStringValue(req.Source)); err != nil {
		return err
	}
	if err := validateCandidateEducationValue(optionalStringValue(req.HighestEducation)); err != nil {
		return err
	}

	return nil
}

func validateCandidateGenderValue(value string) error {
	if value == "" {
		return nil
	}
	if _, ok := candidateGenderValues[value]; ok {
		return nil
	}

	return newInvalidParameterError("gender 只能是 男 或 女")
}

func validateCandidateSourceValue(value string) error {
	if value == "" {
		return nil
	}
	if _, ok := candidateSourceValues[value]; ok {
		return nil
	}

	return newInvalidParameterError("source 只能是 boss 或 email")
}

func validateCandidateEducationValue(value string) error {
	if value == "" {
		return nil
	}
	if _, ok := candidateEducationValues[value]; ok {
		return nil
	}

	return newInvalidParameterError("highestEducation 只能是 专科、本科、硕士 或 博士")
}

func normalizeOptionalCandidateSource(value *string) *string {
	value = trimOptionalString(value)
	if value == nil {
		return nil
	}

	normalized := strings.ToLower(*value)
	return &normalized
}

func normalizeCandidateSourceFilter(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func normalizeCandidateGenderForResponse(value *string) *string {
	return normalizeAllowedCandidateValueForResponse(value, candidateGenderValues)
}

func normalizeCandidateEducationForResponse(value *string) *string {
	return normalizeAllowedCandidateValueForResponse(value, candidateEducationValues)
}

func normalizeCandidateSourceForResponse(value *string) *string {
	value = trimOptionalString(value)
	if value == nil {
		return nil
	}
	if *value == "邮箱" {
		normalized := CandidateSourceEmail
		return &normalized
	}

	normalized := strings.ToLower(*value)
	if _, ok := candidateSourceValues[normalized]; ok {
		return &normalized
	}

	return value
}

func normalizeAllowedCandidateValueForResponse(value *string, allowed map[string]struct{}) *string {
	value = trimOptionalString(value)
	if value == nil {
		return nil
	}
	if _, ok := allowed[*value]; ok {
		return value
	}

	return value
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
		ID:                      candidate.ID,
		Name:                    candidate.Name,
		Email:                   candidate.Email,
		Phone:                   candidate.Phone,
		Gender:                  normalizeCandidateGenderForResponse(candidate.Gender),
		CurrentCompany:          candidate.CurrentCompany,
		PositionCategoryID:      candidate.PositionCategoryID,
		PositionCategoryName:    candidate.CurrentPositionCategory,
		CurrentJobID:            candidate.CurrentJobID,
		CurrentPosition:         candidate.CurrentPosition,
		CurrentPositionCategory: candidate.CurrentPositionCategory,
		YearsOfExperience:       candidate.YearsOfExperience,
		HighestEducation:        normalizeCandidateEducationForResponse(candidate.HighestEducation),
		School:                  candidate.School,
		Major:                   candidate.Major,
		Location:                candidate.Location,
		Source:                  normalizeCandidateSourceForResponse(candidate.Source),
		Status:                  candidate.Status,
		Position:                candidate.CurrentPosition,
		CreatedAt:               candidate.CreatedAt,
		UpdatedAt:               candidate.UpdatedAt,
	}
}

func toCandidateListResponse(candidate repository.CandidateListItem) dto.CandidateResponse {
	return dto.CandidateResponse{
		ID:                      candidate.ID,
		Name:                    candidate.Name,
		Email:                   candidate.Email,
		Phone:                   candidate.Phone,
		Gender:                  normalizeCandidateGenderForResponse(candidate.Gender),
		CurrentCompany:          candidate.CurrentCompany,
		PositionCategoryID:      candidate.PositionCategoryID,
		PositionCategoryName:    candidate.PositionCategoryName,
		CurrentJobID:            candidate.CurrentJobID,
		CurrentPosition:         candidate.CurrentPosition,
		CurrentPositionCategory: candidate.CurrentPositionCategory,
		YearsOfExperience:       candidate.YearsOfExperience,
		HighestEducation:        normalizeCandidateEducationForResponse(candidate.HighestEducation),
		School:                  candidate.School,
		Major:                   candidate.Major,
		Location:                candidate.Location,
		Source:                  normalizeCandidateSourceForResponse(candidate.Source),
		Status:                  candidate.Status,
		Position:                candidate.Position,
		ResumeID:                candidate.ResumeID,
		ResumeFilename:          candidate.ResumeFilename,
		ResumeFileURL:           candidate.ResumeFileURL,
		ResumeParseStatus:       candidate.ResumeParseStatus,
		ResumeParseError:        candidate.ResumeParseError,
		ResumeLanguage:          candidate.ResumeLanguage,
		Language:                candidate.ResumeLanguage,
		ResumeUploadedAt:        candidate.ResumeUploadedAt,
		ResumeEvaluated:         candidate.ResumeEvaluated,
		ScreeningStatus:         candidate.ScreeningStatus,
		AIScore:                 candidate.AIScore,
		ApplicationID:           candidate.ApplicationID,
		JobID:                   candidate.JobID,
		JobTitle:                candidate.JobTitle,
		CreatedAt:               candidate.CreatedAt,
		UpdatedAt:               candidate.UpdatedAt,
	}
}
