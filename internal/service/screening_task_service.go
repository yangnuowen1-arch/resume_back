package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dify"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
	"github.com/yangnuowen1-arch/resume_back/internal/storage"
)

type ScreeningTaskService interface {
	EnqueueResumeScreening(ctx context.Context, job ScreeningTaskQueueJob) error
	RunResumeScreening(ctx context.Context, req dto.RunResumeScreeningRequest) (*dto.RunResumeScreeningResponse, error)
	List(ctx context.Context, query dto.ScreeningTaskQuery) ([]dto.ScreeningTaskResponse, int64, error)
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
			if err := s.processQueuedResumeScreening(context.Background(), job); err != nil {
				log.Printf("screening task %d failed: %v", job.ScreeningResultID, err)
			}
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
		_ = s.repo.MarkFailed(ctx, job.ScreeningResultID, err.Error())
	}

	return err
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

	resume, err := s.resumeRepo.FindByID(ctx, screeningJob.ResumeID)
	if err != nil {
		return s.markRunFailed(ctx, screeningJob.ScreeningResultID, "简历不存在")
	}
	objectKey := resumeObjectKey(resume)
	if objectKey == "" {
		return s.markRunFailed(ctx, screeningJob.ScreeningResultID, "简历文件 key 不能为空")
	}

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

	object, err := s.uploader.Open(ctx, objectKey)
	if err != nil {
		return s.markRunFailed(ctx, screeningJob.ScreeningResultID, "打开简历文件失败: "+err.Error())
	}
	if object == nil || object.Body == nil {
		return s.markRunFailed(ctx, screeningJob.ScreeningResultID, "简历文件内容为空")
	}
	defer object.Body.Close()

	difyResult, err := s.difyClient.RunResumeScreening(ctx, dify.RunResumeScreeningRequest{
		File:           object.Body,
		Filename:       stringValue(resume.OriginalFilename, "resume"),
		ContentType:    firstNonEmpty(stringValue(resume.FileType, ""), object.ContentType, "application/octet-stream"),
		JobContext:     jobContext.JobContext,
		OutputLanguage: firstNonEmpty(strings.TrimSpace(screeningJob.OutputLanguage), "Chinese"),
		User:           firstNonEmpty(s.difyUser, "resume_back"),
	})
	if err != nil {
		return s.markRunFailed(ctx, screeningJob.ScreeningResultID, "Dify 简历筛选失败: "+err.Error())
	}

	aiOutput, err := parseScreeningAIOutput(difyResult.ResultText)
	if err != nil {
		return s.markRunFailed(ctx, screeningJob.ScreeningResultID, "解析 Dify 返回结果失败: "+err.Error())
	}

	rawResponse, err := buildScreeningRawResponse(difyResult, aiOutput)
	if err != nil {
		return s.markRunFailed(ctx, screeningJob.ScreeningResultID, "保存 Dify 返回结果失败: "+err.Error())
	}

	score := aiOutput.Score
	if err := s.repo.MarkSuccess(ctx, screeningJob.ScreeningResultID, repository.ScreeningResultSuccessUpdate{
		Score:               &score,
		MatchLevel:          trimOptionalString(aiOutput.MatchLevel),
		Recommendation:      trimOptionalString(aiOutput.Recommendation),
		Summary:             trimOptionalString(aiOutput.Summary),
		Strengths:           jsonStringPtr(aiOutput.Strengths),
		Weaknesses:          jsonStringPtr(aiOutput.Weaknesses),
		Risks:               jsonStringPtr(aiOutput.Risks),
		MissingRequirements: jsonStringPtr(aiOutput.MissingRequirements),
		AIProvider:          stringPtrValue("dify"),
		PromptVersion:       stringPtrValue("dify_resume_screening_v1"),
		RawResponse:         &rawResponse,
	}); err != nil {
		return err
	}

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
	_ = s.repo.MarkFailed(ctx, id, message)
	return errors.New(message)
}

type screeningAIOutput struct {
	CandidateName               *string       `json:"candidate_name"`
	CurrentTitle                *string       `json:"current_title"`
	YearsOfExperience           *float64      `json:"years_of_experience"`
	HighestEducation            *string       `json:"highest_education"`
	Score                       float64       `json:"score"`
	MatchLevel                  *string       `json:"match_level"`
	Recommendation              *string       `json:"recommendation"`
	Summary                     *string       `json:"summary"`
	MatchedRequirements         []interface{} `json:"matched_requirements"`
	MissingRequirements         []interface{} `json:"missing_requirements"`
	Strengths                   []string      `json:"strengths"`
	Weaknesses                  []string      `json:"weaknesses"`
	Risks                       []string      `json:"risks"`
	SuggestedInterviewQuestions []string      `json:"suggested_interview_questions"`
	MarkdownReport              *string       `json:"markdown_report"`
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
