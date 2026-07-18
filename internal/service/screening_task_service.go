package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dify"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/filemime"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
	"github.com/yangnuowen1-arch/resume_back/internal/storage"
	"github.com/yangnuowen1-arch/resume_back/internal/timeutil"
	"gorm.io/gorm"
)

type ScreeningTaskService interface {
	EnqueueResumeScreening(ctx context.Context, job ScreeningTaskQueueJob) error
	RunResumeScreening(ctx context.Context, req dto.RunResumeScreeningRequest) (*dto.RunResumeScreeningResponse, error)
	List(ctx context.Context, query dto.ScreeningTaskQuery) ([]dto.ScreeningTaskResponse, int64, error)
	Detail(ctx context.Context, id int64) (*dto.ScreeningTaskDetailResponse, error)
}

type DifyResumeScreeningClient interface {
	RunResumeScreening(ctx context.Context, req dify.RunResumeScreeningRequest) (*dify.RunResumeScreeningResponse, error)
}

const (
	ScreeningTaskStatusQueued  = "queued"
	ScreeningTaskStatusRunning = "running"
	ScreeningTaskStatusSuccess = "success"
	ScreeningTaskStatusFailed  = "failed"

	defaultScreeningTaskQueueSize = 100
	defaultScreeningWorkerCount   = 3
	maxScreeningWorkerCount       = 10
)

type ScreeningTaskDependencies struct {
	JobRepo         repository.JobRepository
	ResumeRepo      repository.ResumeRepository
	ApplicationRepo repository.ApplicationRepository
	Uploader        storage.Uploader
	DifyClient      DifyResumeScreeningClient
	DifyUser        string
	QueueSize       int
	WorkerCount     int
}

type ScreeningTaskQueueJob struct {
	ScreeningResultID int64
	ResumeID          int64
	JobID             int64
	OutputLanguage    string
}

type screeningTaskJob struct {
	ScreeningResultID int64
	ResumeID          int64
	JobID             int64
	OutputLanguage    string
}

type screeningTaskService struct {
	repo            repository.ScreeningTaskRepository
	jobRepo         repository.JobRepository
	resumeRepo      repository.ResumeRepository
	applicationRepo repository.ApplicationRepository
	uploader        storage.Uploader
	difyClient      DifyResumeScreeningClient
	difyUser        string
	queue           chan screeningTaskJob
}

func NewScreeningTaskService(repo repository.ScreeningTaskRepository, deps ...ScreeningTaskDependencies) ScreeningTaskService {
	service := &screeningTaskService{repo: repo}
	if len(deps) > 0 {
		service.jobRepo = deps[0].JobRepo
		service.resumeRepo = deps[0].ResumeRepo
		service.applicationRepo = deps[0].ApplicationRepo
		service.uploader = deps[0].Uploader
		service.difyClient = deps[0].DifyClient
		service.difyUser = deps[0].DifyUser

		if service.screeningDependenciesConfigured() {
			queueSize := deps[0].QueueSize
			if queueSize <= 0 {
				queueSize = defaultScreeningTaskQueueSize
			}
			service.queue = make(chan screeningTaskJob, queueSize)
			service.startScreeningWorkers(normalizeScreeningWorkerCount(deps[0].WorkerCount))
		}
	}

	return service
}

func normalizeScreeningWorkerCount(workerCount int) int {
	if workerCount <= 0 {
		return defaultScreeningWorkerCount
	}
	if workerCount > maxScreeningWorkerCount {
		return maxScreeningWorkerCount
	}

	return workerCount
}

func (s *screeningTaskService) screeningDependenciesConfigured() bool {
	return s.repo != nil &&
		s.jobRepo != nil &&
		s.resumeRepo != nil &&
		s.applicationRepo != nil &&
		s.uploader != nil &&
		s.difyClient != nil
}

func (s *screeningTaskService) startScreeningWorkers(workerCount int) {
	for i := 0; i < workerCount; i++ {
		go s.runScreeningWorker(context.Background())
	}
}

func (s *screeningTaskService) runScreeningWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-s.queue:
			start := time.Now()
			log.Printf("screening task started screeningResultId=%d resumeId=%d jobId=%d outputLanguage=%s",
				job.ScreeningResultID, job.ResumeID, job.JobID, job.OutputLanguage)

			if err := s.processQueuedResumeScreening(context.Background(), job); err != nil {
				log.Printf("screening task failed screeningResultId=%d resumeId=%d jobId=%d duration=%s error=%v",
					job.ScreeningResultID, job.ResumeID, job.JobID, time.Since(start), err)
				continue
			}

			log.Printf("screening task succeeded screeningResultId=%d resumeId=%d jobId=%d duration=%s",
				job.ScreeningResultID, job.ResumeID, job.JobID, time.Since(start))
		}
	}
}

func (s *screeningTaskService) enqueueScreeningJob(ctx context.Context, job screeningTaskJob) error {
	if s.queue == nil {
		return errors.New("筛选任务队列未启动")
	}

	select {
	case s.queue <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errors.New("筛选任务队列已满")
	}
}

func (s *screeningTaskService) EnqueueResumeScreening(ctx context.Context, job ScreeningTaskQueueJob) error {
	if !s.screeningDependenciesConfigured() {
		return s.markRunFailed(ctx, job.ScreeningResultID, "Dify 简历筛选未配置")
	}
	if job.ScreeningResultID <= 0 || job.ResumeID <= 0 || job.JobID <= 0 {
		return s.markRunFailed(ctx, job.ScreeningResultID, "筛选任务信息不完整")
	}

	err := s.enqueueScreeningJob(ctx, screeningTaskJob{
		ScreeningResultID: job.ScreeningResultID,
		ResumeID:          job.ResumeID,
		JobID:             job.JobID,
		OutputLanguage:    job.OutputLanguage,
	})
	if err != nil {
		log.Printf("screening task enqueue failed screeningResultId=%d resumeId=%d jobId=%d error=%v",
			job.ScreeningResultID, job.ResumeID, job.JobID, err)
		_ = s.repo.MarkFailed(ctx, job.ScreeningResultID, err.Error())
		return err
	}

	log.Printf("screening task enqueued screeningResultId=%d resumeId=%d jobId=%d outputLanguage=%s",
		job.ScreeningResultID, job.ResumeID, job.JobID, job.OutputLanguage)
	return nil
}

func (s *screeningTaskService) RunResumeScreening(ctx context.Context, req dto.RunResumeScreeningRequest) (*dto.RunResumeScreeningResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}
	if req.ResumeID <= 0 {
		return nil, errors.New("简历 ID 不合法")
	}
	if req.JobID <= 0 {
		return nil, errors.New("岗位 ID 不合法")
	}
	if !s.screeningDependenciesConfigured() {
		return nil, errors.New("Dify 简历筛选未配置")
	}

	resume, err := s.resumeRepo.FindByID(ctx, req.ResumeID)
	if err != nil {
		return nil, errors.New("简历不存在")
	}
	objectKey := resumeObjectKey(resume)
	if objectKey == "" {
		return nil, errors.New("简历文件 key 不能为空")
	}

	if _, err := s.jobRepo.FindByID(ctx, req.JobID); err != nil {
		return nil, errors.New("岗位不存在")
	}

	application, err := s.applicationRepo.FindOrCreateForScreening(ctx, req.JobID, req.ResumeID, resume.CandidateID, userID)
	if err != nil {
		return nil, err
	}

	screeningResult := &model.ScreeningResult{
		ApplicationID: application.ID,
		Status:        ScreeningTaskStatusQueued,
		CreatedBy:     &userID,
		CreatedAt:     timeutil.Now(),
	}
	if err := s.repo.Create(ctx, screeningResult); err != nil {
		return nil, err
	}

	if err := s.EnqueueResumeScreening(ctx, ScreeningTaskQueueJob{
		ScreeningResultID: screeningResult.ID,
		ResumeID:          req.ResumeID,
		JobID:             req.JobID,
		OutputLanguage:    req.OutputLanguage,
	}); err != nil {
		return nil, err
	}

	return &dto.RunResumeScreeningResponse{
		ScreeningResultID: screeningResult.ID,
		ApplicationID:     application.ID,
		ResumeID:          req.ResumeID,
		JobID:             req.JobID,
		Status:            ScreeningTaskStatusQueued,
	}, nil
}

func (s *screeningTaskService) processQueuedResumeScreening(ctx context.Context, screeningJob screeningTaskJob) error {
	if err := s.repo.MarkRunning(ctx, screeningJob.ScreeningResultID); err != nil {
		return err
	}
	log.Printf("screening task marked running screeningResultId=%d resumeId=%d jobId=%d",
		screeningJob.ScreeningResultID, screeningJob.ResumeID, screeningJob.JobID)

	resume, err := s.resumeRepo.FindByID(ctx, screeningJob.ResumeID)
	if err != nil {
		return s.markRunFailed(ctx, screeningJob.ScreeningResultID, "简历不存在")
	}
	objectKey := resumeObjectKey(resume)
	if objectKey == "" {
		return s.markRunFailed(ctx, screeningJob.ScreeningResultID, "简历文件 key 不能为空")
	}
	log.Printf("screening task resume loaded screeningResultId=%d resumeId=%d objectKey=%s",
		screeningJob.ScreeningResultID, screeningJob.ResumeID, objectKey)

	job, err := s.jobRepo.FindByID(ctx, screeningJob.JobID)
	if err != nil {
		return s.markRunFailed(ctx, screeningJob.ScreeningResultID, "岗位不存在")
	}
	tags, err := s.jobRepo.ListTags(ctx, screeningJob.JobID)
	if err != nil {
		return s.markRunFailed(ctx, screeningJob.ScreeningResultID, "读取岗位标签失败: "+err.Error())
	}
	jobContext, err := buildJobScreeningContextResponse(job, tags)
	if err != nil {
		return s.markRunFailed(ctx, screeningJob.ScreeningResultID, "生成岗位筛选上下文失败: "+err.Error())
	}
	log.Printf("screening task job context built screeningResultId=%d jobId=%d tagCount=%d",
		screeningJob.ScreeningResultID, screeningJob.JobID, len(tags))

	object, err := s.uploader.Open(ctx, objectKey)
	if err != nil {
		return s.markRunFailed(ctx, screeningJob.ScreeningResultID, "打开简历文件失败: "+err.Error())
	}
	if object == nil || object.Body == nil {
		return s.markRunFailed(ctx, screeningJob.ScreeningResultID, "简历文件内容为空")
	}
	defer object.Body.Close()
	log.Printf("screening task resume file opened screeningResultId=%d resumeId=%d contentType=%s",
		screeningJob.ScreeningResultID, screeningJob.ResumeID, object.ContentType)

	difyStart := time.Now()
	log.Printf("screening task dify screening started screeningResultId=%d resumeId=%d jobId=%d",
		screeningJob.ScreeningResultID, screeningJob.ResumeID, screeningJob.JobID)
	filename := stringValue(resume.OriginalFilename, "resume")
	difyResult, err := s.difyClient.RunResumeScreening(ctx, dify.RunResumeScreeningRequest{
		File:           object.Body,
		Filename:       filename,
		ContentType:    filemime.NormalizeAny(filename, stringValue(resume.FileType, ""), object.ContentType),
		JobContext:     jobContext.JobContext,
		OutputLanguage: firstNonEmpty(strings.TrimSpace(screeningJob.OutputLanguage), "Chinese"),
		User:           firstNonEmpty(s.difyUser, "resume_back"),
	})
	if err != nil {
		log.Printf("screening task dify screening failed screeningResultId=%d duration=%s error=%v",
			screeningJob.ScreeningResultID, time.Since(difyStart), err)
		return s.markRunFailed(ctx, screeningJob.ScreeningResultID, "Dify 简历筛选失败: "+err.Error())
	}
	log.Printf("screening task dify screening succeeded screeningResultId=%d workflowRunId=%s taskId=%s duration=%s",
		screeningJob.ScreeningResultID, difyResult.WorkflowRunID, difyResult.TaskID, time.Since(difyStart))

	aiOutput, err := parseScreeningAIOutput(difyResult.ResultText)
	if err != nil {
		return s.markRunFailed(ctx, screeningJob.ScreeningResultID, "解析 Dify 返回结果失败: "+err.Error())
	}
	log.Printf("screening task dify output parsed screeningResultId=%d score=%.2f",
		screeningJob.ScreeningResultID, aiOutput.Score)

	rawResponse, err := buildScreeningRawResponse(difyResult, aiOutput)
	if err != nil {
		return s.markRunFailed(ctx, screeningJob.ScreeningResultID, "保存 Dify 返回结果失败: "+err.Error())
	}

	score := aiOutput.Score
	if err := s.repo.MarkSuccess(ctx, screeningJob.ScreeningResultID, repository.ScreeningResultSuccessUpdate{
		CandidateName:       trimOptionalString(aiOutput.CandidateName),
		Score:               &score,
		MatchLevel:          trimOptionalString(aiOutput.MatchLevel),
		Recommendation:      trimOptionalString(aiOutput.Recommendation),
		Summary:             trimOptionalString(aiOutput.Summary),
		Strengths:           jsonStringPtr(aiOutput.Strengths),
		Weaknesses:          jsonStringPtr(aiOutput.Weaknesses),
		Risks:               jsonStringPtr(aiOutput.Risks),
		MissingRequirements: jsonStringPtr(aiOutput.MissingRequirements),
		Requirements:        buildRequirementsJSON(aiOutput.Requirements),
		AIProvider:          stringPtrValue("dify"),
		PromptVersion:       stringPtrValue("dify_resume_screening_v1"),
		RawResponse:         &rawResponse,
	}); err != nil {
		return err
	}
	log.Printf("screening task result saved screeningResultId=%d score=%.2f",
		screeningJob.ScreeningResultID, score)

	return nil
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

func (s *screeningTaskService) Detail(ctx context.Context, id int64) (*dto.ScreeningTaskDetailResponse, error) {
	if id <= 0 {
		return nil, ErrScreeningTaskNotFound
	}

	item, err := s.repo.FindDetailByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrScreeningTaskNotFound
		}
		return nil, err
	}
	if item == nil {
		return nil, ErrScreeningTaskNotFound
	}

	rawOutput := extractStoredScreeningOutput(item.RawResponse)
	requirements := parseStoredRequirements(item.Requirements)
	markdownReport := trimOptionalString(rawOutput.MarkdownReport)
	summary := firstOptionalString(item.Summary, rawOutput.Summary)
	matchLevel := firstOptionalString(item.MatchLevel, rawOutput.MatchLevel)
	recommendation := firstOptionalString(item.Recommendation, rawOutput.Recommendation)
	previewURL := resumePreviewURL(item.ResumeFileURL)

	detail := &dto.ScreeningTaskDetailResponse{
		ID:               item.ID,
		Status:           item.Status,
		ErrorMessage:     trimOptionalString(item.ErrorMessage),
		CandidateName:    item.CandidateName,
		Position:         item.Position,
		AIScore:          item.AIScore,
		MatchLevel:       matchLevel,
		Recommendation:   recommendation,
		Summary:          summary,
		MarkdownReport:   markdownReport,
		ResumeID:         item.ResumeID,
		ResumeFilename:   item.ResumeFilename,
		ResumeFileURL:    item.ResumeFileURL,
		ResumePreviewURL: previewURL,
		ResumeFileType:   item.ResumeFileType,
		ResumeText:       item.ResumeText,
		Requirements:     requirements,
		Sections:         buildScreeningTaskDetailSections(item, rawOutput, requirements, markdownReport),
	}

	return detail, nil
}

// extractMarkdownReport 从 raw_response JSON 的 output.markdown_report 取出报告文本。
func extractMarkdownReport(rawResponse *string) *string {
	return trimOptionalString(extractStoredScreeningOutput(rawResponse).MarkdownReport)
}

// extractStoredScreeningOutput 从当前 raw_response envelope（或兼容的旧版直出 JSON）取出 AI 输出。
func extractStoredScreeningOutput(rawResponse *string) screeningAIOutput {
	if rawResponse == nil || strings.TrimSpace(*rawResponse) == "" {
		return screeningAIOutput{}
	}

	var payload struct {
		Output json.RawMessage `json:"output"`
	}
	if err := json.Unmarshal([]byte(*rawResponse), &payload); err == nil && len(payload.Output) > 0 && string(payload.Output) != "null" {
		var output screeningAIOutput
		if err := json.Unmarshal(payload.Output, &output); err == nil {
			return output
		}
	}

	var output screeningAIOutput
	if err := json.Unmarshal([]byte(*rawResponse), &output); err != nil {
		return screeningAIOutput{}
	}

	return output
}

// parseStoredRequirements 反序列化已落库的 requirements JSON，缺失时返回空数组（前端走降级态）。
func parseStoredRequirements(stored *string) []dto.ScreeningRequirement {
	if stored == nil || strings.TrimSpace(*stored) == "" {
		return []dto.ScreeningRequirement{}
	}

	var requirements []dto.ScreeningRequirement
	if err := json.Unmarshal([]byte(*stored), &requirements); err != nil {
		return []dto.ScreeningRequirement{}
	}
	if requirements == nil {
		return []dto.ScreeningRequirement{}
	}

	for index := range requirements {
		requirements[index] = normalizeStoredRequirement(requirements[index], index)
	}

	return requirements
}

// buildScreeningTaskDetailSections 将数据库记录和 AI 原始输出组装为页面可直接消费的模块。
// 平铺字段仍在 Detail 响应中保留，sections 是新页面的主数据源。
func buildScreeningTaskDetailSections(
	item *repository.ScreeningTaskDetailItem,
	rawOutput screeningAIOutput,
	requirements []dto.ScreeningRequirement,
	markdownReport *string,
) dto.ScreeningTaskDetailSections {
	matchedItems := make([]dto.ScreeningRequirement, 0)
	attentionItems := make([]dto.ScreeningRequirement, 0)
	for _, requirement := range requirements {
		switch requirement.Status {
		case "pass":
			matchedItems = append(matchedItems, requirement)
		default:
			attentionItems = append(attentionItems, requirement)
		}
	}

	summary := firstOptionalString(item.Summary, rawOutput.Summary)
	matchLevel := firstOptionalString(item.MatchLevel, rawOutput.MatchLevel)
	recommendation := firstOptionalString(item.Recommendation, rawOutput.Recommendation)
	strengths := firstNonEmptyStringSlice(parseStoredStringSlice(item.Strengths), rawOutput.Strengths)
	weaknesses := firstNonEmptyStringSlice(parseStoredStringSlice(item.Weaknesses), rawOutput.Weaknesses)
	risks := firstNonEmptyStringSlice(parseStoredStringSlice(item.Risks), rawOutput.Risks)
	interviewQuestions := normalizedStringSlice(rawOutput.SuggestedInterviewQuestions)

	textAvailable := item.ResumeText != nil && strings.TrimSpace(*item.ResumeText) != ""
	highlightAvailable := hasPDFPreview(item.ResumeFileURL, item.ResumeFileType, item.ResumeFilename) &&
		hasHighlightableEvidence(requirements)

	return dto.ScreeningTaskDetailSections{
		Summary: dto.ScreeningSummarySection{
			Text: summary,
		},
		CandidateInfo: dto.ScreeningCandidateInfoSection{
			Name:              item.CandidateName,
			AppliedPosition:   item.Position,
			CurrentTitle:      firstOptionalString(rawOutput.CurrentTitle, item.CandidateCurrentTitle),
			YearsOfExperience: firstOptionalFloat(rawOutput.YearsOfExperience, item.CandidateYearsOfExperience),
			HighestEducation:  firstOptionalString(rawOutput.HighestEducation, item.CandidateHighestEducation),
			TaskStatus:        item.Status,
			TaskErrorMessage:  trimOptionalString(item.ErrorMessage),
		},
		Assessment: dto.ScreeningAssessmentSection{
			Score:          item.AIScore,
			MatchLevel:     matchLevel,
			Recommendation: recommendation,
		},
		RequirementsComparison: dto.ScreeningRequirementsComparisonSection{
			Items:          requirements,
			MatchedItems:   matchedItems,
			AttentionItems: attentionItems,
		},
		CandidateAnalysis: dto.ScreeningCandidateAnalysisSection{
			Strengths:                   strengths,
			Weaknesses:                  weaknesses,
			Risks:                       risks,
			SuggestedInterviewQuestions: interviewQuestions,
		},
		FinalRecommendation: dto.ScreeningFinalRecommendationSection{
			Recommendation: recommendation,
			Text:           summary,
		},
		Resume: dto.ScreeningResumeSection{
			ID:                 item.ResumeID,
			Filename:           item.ResumeFilename,
			FileURL:            item.ResumeFileURL,
			PreviewURL:         resumePreviewURL(item.ResumeFileURL),
			FileType:           item.ResumeFileType,
			Text:               item.ResumeText,
			TextAvailable:      textAvailable,
			HighlightAvailable: highlightAvailable,
		},
		Fallback: dto.ScreeningFallbackSection{
			MarkdownReport:            markdownReport,
			ShouldUseMarkdownFallback: item.Status == ScreeningTaskStatusSuccess && len(requirements) == 0 && markdownReport != nil,
		},
	}
}

func normalizeStoredRequirement(requirement dto.ScreeningRequirement, index int) dto.ScreeningRequirement {
	requirement.ID = requirementID(requirement.ID, index)
	requirement.Label = strings.TrimSpace(requirement.Label)
	requirement.Status = normalizeRequirementStatus(requirement.Status)
	requirement.Comment = trimOptionalString(requirement.Comment)
	requirement.Evidence = normalizeRequirementEvidence(requirement.Evidence)
	requirement.CandidateSituation = firstOptionalString(
		trimOptionalString(requirement.CandidateSituation),
		candidateSituationFromEvidence(requirement.Evidence, requirement.Comment),
	)
	return requirement
}

func normalizeRequirementEvidence(evidence []dto.RequirementEvidence) []dto.RequirementEvidence {
	result := make([]dto.RequirementEvidence, 0, len(evidence))
	for _, item := range evidence {
		if strings.TrimSpace(item.Text) == "" {
			continue
		}
		result = append(result, dto.RequirementEvidence{
			Text:   item.Text,
			Reason: trimOptionalString(item.Reason),
			Type:   trimOptionalString(item.Type),
		})
	}
	if result == nil {
		return []dto.RequirementEvidence{}
	}
	return result
}

func candidateSituationFromEvidence(evidence []dto.RequirementEvidence, fallback *string) *string {
	parts := make([]string, 0, len(evidence))
	for _, item := range evidence {
		text := strings.TrimSpace(item.Text)
		if text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) > 0 {
		value := strings.Join(parts, "；")
		return &value
	}

	return trimOptionalString(fallback)
}

func parseStoredStringSlice(stored *string) []string {
	if stored == nil || strings.TrimSpace(*stored) == "" {
		return []string{}
	}

	var values []string
	if err := json.Unmarshal([]byte(*stored), &values); err != nil || values == nil {
		return []string{}
	}

	return normalizedStringSlice(values)
}

func normalizedStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func firstNonEmptyStringSlice(values ...[]string) []string {
	for _, value := range values {
		value = normalizedStringSlice(value)
		if len(value) > 0 {
			return value
		}
	}

	return []string{}
}

func firstOptionalString(values ...*string) *string {
	for _, value := range values {
		if value = trimOptionalString(value); value != nil {
			return value
		}
	}

	return nil
}

func firstOptionalFloat(values ...*float64) *float64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}

	return nil
}

func hasHighlightableEvidence(requirements []dto.ScreeningRequirement) bool {
	for _, requirement := range requirements {
		for _, evidence := range requirement.Evidence {
			if strings.TrimSpace(evidence.Text) != "" {
				return true
			}
		}
	}

	return false
}

func hasPDFPreview(fileURL *string, fileType *string, filename *string) bool {
	if fileURL == nil || strings.TrimSpace(*fileURL) == "" {
		return false
	}

	normalizedFileType := strings.ToLower(strings.TrimSpace(stringValue(fileType, "")))
	if strings.Contains(normalizedFileType, "pdf") {
		return true
	}

	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(stringValue(filename, ""))), ".pdf")
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

func (s *screeningTaskService) markRunFailed(ctx context.Context, id int64, message string) error {
	if err := s.repo.MarkFailed(ctx, id, message); err != nil {
		log.Printf("screening task mark failed error screeningResultId=%d error=%v originalError=%s", id, err, message)
	}
	return errors.New(message)
}

type screeningAIOutput struct {
	CandidateName               *string         `json:"candidate_name"`
	CurrentTitle                *string         `json:"current_title"`
	YearsOfExperience           *float64        `json:"years_of_experience"`
	HighestEducation            *string         `json:"highest_education"`
	Score                       float64         `json:"score"`
	MatchLevel                  *string         `json:"match_level"`
	Recommendation              *string         `json:"recommendation"`
	Summary                     *string         `json:"summary"`
	MatchedRequirements         []interface{}   `json:"matched_requirements"`
	MissingRequirements         []interface{}   `json:"missing_requirements"`
	Requirements                []aiRequirement `json:"requirements"`
	Strengths                   []string        `json:"strengths"`
	Weaknesses                  []string        `json:"weaknesses"`
	Risks                       []string        `json:"risks"`
	SuggestedInterviewQuestions []string        `json:"suggested_interview_questions"`
	MarkdownReport              *string         `json:"markdown_report"`
}

type aiRequirement struct {
	ID                 interface{}  `json:"id"`
	Label              string       `json:"label"`
	CandidateSituation *string      `json:"candidate_situation"`
	Status             string       `json:"status"`
	Comment            *string      `json:"comment"`
	Evidence           []aiEvidence `json:"evidence"`
}

type aiEvidence struct {
	Text   string  `json:"text"`
	Reason *string `json:"reason"`
	Type   *string `json:"type"`
}

func parseScreeningAIOutput(text string) (screeningAIOutput, error) {
	text = strings.TrimSpace(stripJSONCodeFence(text))
	if text == "" {
		return screeningAIOutput{}, errors.New("返回内容为空")
	}

	var output screeningAIOutput
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return screeningAIOutput{}, err
	}
	if output.Score < 0 || output.Score > 100 {
		return screeningAIOutput{}, errors.New("score 必须在 0 到 100 之间")
	}

	return output, nil
}

func stripJSONCodeFence(text string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "```") {
		return text
	}

	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```JSON")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(strings.TrimSpace(text), "```")
	return strings.TrimSpace(text)
}

func buildScreeningRawResponse(result *dify.RunResumeScreeningResponse, output screeningAIOutput) (string, error) {
	payload := map[string]interface{}{
		"dify": map[string]interface{}{
			"workflowRunId": result.WorkflowRunID,
			"taskId":        result.TaskID,
			"rawResponse":   result.RawResponse,
		},
		"output": output,
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func jsonStringPtr(value interface{}) *string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	text := string(encoded)
	return &text
}

func normalizeRequirementStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pass":
		return "pass"
	case "partial":
		return "partial"
	default:
		return "miss"
	}
}

// buildRequirementsJSON 将 Dify 输出的 requirements 归一化为前端 DTO 结构。
// evidence.text 保留 Dify 从 PDF 原文复制的文本，前端使用 pdf.js textLayer 定位高亮。
func buildRequirementsJSON(items []aiRequirement) *string {
	if len(items) == 0 {
		return nil
	}

	requirements := make([]dto.ScreeningRequirement, 0, len(items))
	for i, item := range items {
		label := strings.TrimSpace(item.Label)
		if label == "" {
			continue
		}

		evidence := make([]dto.RequirementEvidence, 0, len(item.Evidence))
		for _, ev := range item.Evidence {
			text := ev.Text
			if strings.TrimSpace(text) == "" {
				continue
			}
			evidence = append(evidence, dto.RequirementEvidence{
				Text:   text,
				Reason: trimOptionalString(ev.Reason),
				Type:   trimOptionalString(ev.Type),
			})
		}
		comment := trimOptionalString(item.Comment)
		candidateSituation := firstOptionalString(
			item.CandidateSituation,
			candidateSituationFromEvidence(evidence, comment),
		)

		requirements = append(requirements, dto.ScreeningRequirement{
			ID:                 requirementID(item.ID, i),
			Label:              label,
			CandidateSituation: candidateSituation,
			Status:             normalizeRequirementStatus(item.Status),
			Comment:            comment,
			Evidence:           evidence,
		})
	}

	if len(requirements) == 0 {
		return nil
	}

	return jsonStringPtr(requirements)
}

func requirementID(raw interface{}, index int) string {
	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			return v
		}
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	}
	return "r" + strconv.Itoa(index+1)
}

func stringPtrValue(value string) *string {
	return &value
}

func stringValue(value *string, fallback string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fallback
	}
	return strings.TrimSpace(*value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}

	return ""
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
