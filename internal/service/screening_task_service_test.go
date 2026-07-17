package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/auth"
	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dify"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
	"github.com/yangnuowen1-arch/resume_back/internal/storage"
)

func TestNormalizeScreeningTaskQueryDefaultsInvalidPagination(t *testing.T) {
	query := normalizeScreeningTaskQuery(dto.ScreeningTaskQuery{
		Page:     0,
		PageSize: 201,
	})

	if query.Page != 1 {
		t.Fatalf("expected page to default to 1, got %d", query.Page)
	}
	if query.PageSize != 20 {
		t.Fatalf("expected page size to default to 20, got %d", query.PageSize)
	}
}

func TestNormalizeScreeningTaskQueryAllowsParserMaxPageSize(t *testing.T) {
	query := normalizeScreeningTaskQuery(dto.ScreeningTaskQuery{
		Page:     2,
		PageSize: 200,
	})

	if query.Page != 2 {
		t.Fatalf("expected page to remain 2, got %d", query.Page)
	}
	if query.PageSize != 200 {
		t.Fatalf("expected page size to remain 200, got %d", query.PageSize)
	}
}

func TestToScreeningTaskResponseSetsTableDisplayFields(t *testing.T) {
	now := time.Date(2026, time.June, 4, 10, 30, 0, 0, time.UTC)
	score := 88.5
	candidateName := "Alice"

	resp := toScreeningTaskResponse(repository.ScreeningTaskListItem{
		ID:            1,
		ApplicationID: 2,
		CandidateName: &candidateName,
		JobID:         3,
		JobTitle:      "Backend Engineer",
		Position:      "Backend Engineer",
		AIScore:       &score,
		Status:        "success",
		CreatedAt:     now,
	})

	if resp.Candidate == nil || *resp.Candidate != candidateName {
		t.Fatalf("expected candidate display field %q, got %v", candidateName, resp.Candidate)
	}
	if resp.Position != "Backend Engineer" {
		t.Fatalf("expected position display field, got %q", resp.Position)
	}
	if resp.AIScore == nil || *resp.AIScore != score {
		t.Fatalf("expected AI score %v, got %v", score, resp.AIScore)
	}
	if resp.Status != "success" {
		t.Fatalf("expected status success, got %q", resp.Status)
	}
	if !resp.Date.Equal(now) {
		t.Fatalf("expected date %v, got %v", now, resp.Date)
	}
}

func TestParseScreeningAIOutputAcceptsJSONCodeFence(t *testing.T) {
	result, err := parseScreeningAIOutput("```json\n{\"score\":86,\"match_level\":\"strong\",\"recommendation\":\"recommend_interview\",\"summary\":\"匹配\",\"strengths\":[\"Go\"],\"weaknesses\":[],\"risks\":[],\"missing_requirements\":[],\"markdown_report\":\"## Report\"}\n```")
	if err != nil {
		t.Fatalf("parse screening AI output: %v", err)
	}

	if result.Score != 86 {
		t.Fatalf("expected score 86, got %v", result.Score)
	}
	if result.MarkdownReport == nil || *result.MarkdownReport != "## Report" {
		t.Fatalf("expected markdown report, got %#v", result.MarkdownReport)
	}
}

func TestParseScreeningAIOutputRejectsInvalidScore(t *testing.T) {
	if _, err := parseScreeningAIOutput("{\"score\":120}"); err == nil {
		t.Fatal("expected invalid score to be rejected")
	}
}

func TestNormalizeScreeningWorkerCount(t *testing.T) {
	if got := normalizeScreeningWorkerCount(0); got != defaultScreeningWorkerCount {
		t.Fatalf("expected default worker count %d, got %d", defaultScreeningWorkerCount, got)
	}
	if got := normalizeScreeningWorkerCount(5); got != 5 {
		t.Fatalf("expected worker count 5, got %d", got)
	}
	if got := normalizeScreeningWorkerCount(maxScreeningWorkerCount + 1); got != maxScreeningWorkerCount {
		t.Fatalf("expected max worker count %d, got %d", maxScreeningWorkerCount, got)
	}
}

func TestRunResumeScreeningQueuesTaskWithoutCallingDifySynchronously(t *testing.T) {
	fileKey := "resumes/alice.pdf"
	filename := "alice.pdf"
	fileType := "application/pdf"
	candidateID := int64(12)
	repo := &stubScreeningTaskRepository{nextID: 99}
	difyClient := &stubDifyClient{resultText: `{"score":86}`}
	service := &screeningTaskService{
		repo: repo,
		resumeRepo: &stubResumeRepository{resume: &model.Resume{
			ID:               1,
			CandidateID:      &candidateID,
			OriginalFilename: &filename,
			FileKey:          &fileKey,
			FileType:         &fileType,
		}},
		jobRepo: &stubJobRepository{job: &repository.JobDetailItem{
			Job: model.Job{ID: 2, Title: "Backend Engineer"},
		}},
		applicationRepo: &stubApplicationRepository{application: &model.Application{ID: 3}},
		uploader:        &stubUploader{},
		difyClient:      difyClient,
		queue:           make(chan screeningTaskJob, 1),
	}

	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7})
	resp, err := service.RunResumeScreening(ctx, dto.RunResumeScreeningRequest{
		ResumeID:       1,
		JobID:          2,
		OutputLanguage: "English",
	})
	if err != nil {
		t.Fatalf("queue resume screening: %v", err)
	}

	if resp.Status != ScreeningTaskStatusQueued {
		t.Fatalf("expected queued status, got %q", resp.Status)
	}
	if resp.ScreeningResultID != 99 {
		t.Fatalf("expected screening result ID 99, got %d", resp.ScreeningResultID)
	}
	if repo.created == nil || repo.created.Status != ScreeningTaskStatusQueued {
		t.Fatalf("expected created screening result to be queued, got %#v", repo.created)
	}
	if repo.created.CreatedAt.IsZero() {
		t.Fatal("expected created screening result time to be set")
	}
	if _, offset := repo.created.CreatedAt.Zone(); offset != 8*60*60 {
		t.Fatalf("expected created screening result time to use +08:00, got %s", repo.created.CreatedAt.Format(time.RFC3339))
	}
	if difyClient.calls != 0 {
		t.Fatalf("expected Dify not to be called synchronously, got %d calls", difyClient.calls)
	}

	select {
	case job := <-service.queue:
		if job.ScreeningResultID != 99 || job.ResumeID != 1 || job.JobID != 2 || job.OutputLanguage != "English" {
			t.Fatalf("unexpected queued job: %#v", job)
		}
	default:
		t.Fatal("expected job to be queued")
	}
}

func TestProcessQueuedResumeScreeningMarksRunningAndSuccess(t *testing.T) {
	fileKey := "resumes/alice.pdf"
	filename := "alice.pdf"
	fileType := "pdf"
	repo := &stubScreeningTaskRepository{}
	difyClient := &stubDifyClient{
		resultText: `{"candidate_name":" Alice ","score":91,"match_level":"strong","recommendation":"recommend_interview","summary":"Good fit","strengths":["Go"],"weaknesses":[],"risks":[],"missing_requirements":[]}`,
	}
	service := &screeningTaskService{
		repo: repo,
		resumeRepo: &stubResumeRepository{resume: &model.Resume{
			ID:               1,
			OriginalFilename: &filename,
			FileKey:          &fileKey,
			FileType:         &fileType,
		}},
		jobRepo: &stubJobRepository{job: &repository.JobDetailItem{
			Job: model.Job{ID: 2, Title: "Backend Engineer"},
		}},
		uploader: &stubUploader{object: &storage.Object{
			Body:        io.NopCloser(strings.NewReader("resume text")),
			ContentType: "application/pdf",
		}},
		difyClient: difyClient,
		difyUser:   "resume_back",
	}

	err := service.processQueuedResumeScreening(context.Background(), screeningTaskJob{
		ScreeningResultID: 99,
		ResumeID:          1,
		JobID:             2,
		OutputLanguage:    "English",
	})
	if err != nil {
		t.Fatalf("process queued screening: %v", err)
	}

	if len(repo.runningIDs) != 1 || repo.runningIDs[0] != 99 {
		t.Fatalf("expected task 99 to be marked running, got %#v", repo.runningIDs)
	}
	if repo.successID != 99 {
		t.Fatalf("expected task 99 to be marked success, got %d", repo.successID)
	}
	if repo.successUpdate.Score == nil || *repo.successUpdate.Score != 91 {
		t.Fatalf("expected score 91, got %#v", repo.successUpdate.Score)
	}
	if repo.successUpdate.CandidateName == nil || *repo.successUpdate.CandidateName != "Alice" {
		t.Fatalf("expected trimmed candidate name Alice, got %#v", repo.successUpdate.CandidateName)
	}
	if difyClient.calls != 1 {
		t.Fatalf("expected one Dify call, got %d", difyClient.calls)
	}
	if difyClient.lastReq.OutputLanguage != "English" {
		t.Fatalf("expected output language English, got %q", difyClient.lastReq.OutputLanguage)
	}
	if difyClient.lastReq.ContentType != "application/pdf" {
		t.Fatalf("expected normalized Dify content type application/pdf, got %q", difyClient.lastReq.ContentType)
	}
}

func TestScreeningTaskDetailBuildsSectionedResponse(t *testing.T) {
	candidateName := "杨诺雯-AI全栈或应用开发"
	score := 35.0
	storedMatchLevel := "poor"
	storedRecommendation := "reject"
	storedSummary := "候选人与岗位的核心技术栈不匹配，不建议推荐。"
	storedStrengths := `["已落库优势"]`
	storedWeaknesses := `["已落库劣势"]`
	storedRisks := `["已落库风险"]`
	resumeText := "姓名：杨诺雯\n嘉应学院·软件工程(本科)\n工作经验: 两年"
	requirements := `[
		{"id":"r1","label":"负责 Flutter 开发 / 要会 Flutter","status":"miss","comment":"候选人简历中无 Flutter 或 Dart 经验。","evidence":[]},
		{"id":"r2","label":"学历：本科","status":"pass","comment":"符合本科要求。","evidence":[{"text":"嘉应学院·软件工程(本科)","start":7,"end":20}]},
		{"id":"r3","label":"工作经验：3 年","status":"partial","comment":"工作年限略低于要求。","evidence":[{"text":"工作经验: 两年","start":21,"end":29}]}
	]`
	rawResponse := `{
		"output": {
			"current_title": "前端开发工程师 / AI 全栈开发",
			"years_of_experience": 2,
			"highest_education": "嘉应学院·软件工程（本科）",
			"summary": "原始摘要不应覆盖已落库摘要。",
			"strengths": ["原始优势不应覆盖已落库优势"],
			"weaknesses": ["原始劣势不应覆盖已落库劣势"],
			"risks": ["原始风险不应覆盖已落库风险"],
			"suggested_interview_questions": ["请说明 Flutter 学习计划。"],
			"markdown_report": "# Markdown 报告"
		}
	}`
	candidateCurrentTitle := "候选人档案岗位"
	candidateYears := 9.0
	candidateEducation := "候选人档案学历"

	service := &screeningTaskService{repo: &stubScreeningTaskRepository{detail: &repository.ScreeningTaskDetailItem{
		ID:                         30,
		Status:                     ScreeningTaskStatusSuccess,
		CandidateName:              &candidateName,
		Position:                   "Frontend",
		CandidateCurrentTitle:      &candidateCurrentTitle,
		CandidateYearsOfExperience: &candidateYears,
		CandidateHighestEducation:  &candidateEducation,
		AIScore:                    &score,
		MatchLevel:                 &storedMatchLevel,
		Recommendation:             &storedRecommendation,
		Summary:                    &storedSummary,
		Strengths:                  &storedStrengths,
		Weaknesses:                 &storedWeaknesses,
		Risks:                      &storedRisks,
		RawResponse:                &rawResponse,
		Requirements:               &requirements,
		ResumeText:                 &resumeText,
	}}}

	result, err := service.Detail(context.Background(), 30)
	if err != nil {
		t.Fatalf("get screening detail: %v", err)
	}

	if result.Status != ScreeningTaskStatusSuccess {
		t.Fatalf("expected success status, got %q", result.Status)
	}
	if result.Sections.Summary.Text == nil || *result.Sections.Summary.Text != storedSummary {
		t.Fatalf("expected stored summary, got %#v", result.Sections.Summary.Text)
	}
	if result.Sections.CandidateInfo.Name == nil || *result.Sections.CandidateInfo.Name != candidateName {
		t.Fatalf("expected candidate name, got %#v", result.Sections.CandidateInfo.Name)
	}
	if result.Sections.CandidateInfo.CurrentTitle == nil || *result.Sections.CandidateInfo.CurrentTitle != "前端开发工程师 / AI 全栈开发" {
		t.Fatalf("expected AI current title to take precedence, got %#v", result.Sections.CandidateInfo.CurrentTitle)
	}
	if result.Sections.CandidateInfo.YearsOfExperience == nil || *result.Sections.CandidateInfo.YearsOfExperience != 2 {
		t.Fatalf("expected AI years of experience, got %#v", result.Sections.CandidateInfo.YearsOfExperience)
	}
	if result.Sections.CandidateInfo.HighestEducation == nil || *result.Sections.CandidateInfo.HighestEducation != "嘉应学院·软件工程（本科）" {
		t.Fatalf("expected AI education, got %#v", result.Sections.CandidateInfo.HighestEducation)
	}
	if result.Sections.CandidateInfo.TaskStatus != ScreeningTaskStatusSuccess {
		t.Fatalf("expected task status success, got %q", result.Sections.CandidateInfo.TaskStatus)
	}
	if result.Sections.Assessment.Score == nil || *result.Sections.Assessment.Score != score {
		t.Fatalf("expected assessment score %v, got %#v", score, result.Sections.Assessment.Score)
	}
	if result.Sections.Assessment.MatchLevel == nil || *result.Sections.Assessment.MatchLevel != storedMatchLevel {
		t.Fatalf("expected match level %q, got %#v", storedMatchLevel, result.Sections.Assessment.MatchLevel)
	}
	if len(result.Sections.RequirementsComparison.Items) != 3 {
		t.Fatalf("expected 3 requirements, got %d", len(result.Sections.RequirementsComparison.Items))
	}
	if len(result.Sections.RequirementsComparison.MatchedItems) != 1 {
		t.Fatalf("expected 1 matched item, got %d", len(result.Sections.RequirementsComparison.MatchedItems))
	}
	if len(result.Sections.RequirementsComparison.AttentionItems) != 2 {
		t.Fatalf("expected 2 attention items, got %d", len(result.Sections.RequirementsComparison.AttentionItems))
	}
	if situation := result.Sections.RequirementsComparison.Items[1].CandidateSituation; situation == nil || *situation != "嘉应学院·软件工程(本科)" {
		t.Fatalf("expected candidate situation derived from evidence, got %#v", situation)
	}
	if situation := result.Sections.RequirementsComparison.Items[0].CandidateSituation; situation == nil || *situation != "候选人简历中无 Flutter 或 Dart 经验。" {
		t.Fatalf("expected candidate situation to fall back to comment, got %#v", situation)
	}
	if got := result.Sections.CandidateAnalysis.Strengths; len(got) != 1 || got[0] != "已落库优势" {
		t.Fatalf("expected stored strengths to take precedence, got %#v", got)
	}
	if got := result.Sections.CandidateAnalysis.SuggestedInterviewQuestions; len(got) != 1 || got[0] != "请说明 Flutter 学习计划。" {
		t.Fatalf("expected interview questions from raw output, got %#v", got)
	}
	if !result.Sections.Resume.TextAvailable || !result.Sections.Resume.HighlightAvailable {
		t.Fatalf("expected resume text and highlights to be available, got %#v", result.Sections.Resume)
	}
	if result.Sections.Fallback.ShouldUseMarkdownFallback {
		t.Fatal("structured requirements should not use the Markdown fallback")
	}
	if result.Sections.Fallback.MarkdownReport == nil || *result.Sections.Fallback.MarkdownReport != "# Markdown 报告" {
		t.Fatalf("expected fallback markdown report, got %#v", result.Sections.Fallback.MarkdownReport)
	}
}

func TestScreeningTaskDetailUsesMarkdownFallbackWhenRequirementsAreAbsent(t *testing.T) {
	rawResponse := `{"output":{"markdown_report":"# Markdown 报告"}}`
	errorMessage := "Dify 简历筛选失败: 请求超时"
	service := &screeningTaskService{repo: &stubScreeningTaskRepository{detail: &repository.ScreeningTaskDetailItem{
		ID:           31,
		Status:       ScreeningTaskStatusFailed,
		ErrorMessage: &errorMessage,
		RawResponse:  &rawResponse,
	}}}

	result, err := service.Detail(context.Background(), 31)
	if err != nil {
		t.Fatalf("get screening detail: %v", err)
	}
	if result.Status != ScreeningTaskStatusFailed {
		t.Fatalf("expected failed status, got %q", result.Status)
	}
	if result.ErrorMessage == nil || *result.ErrorMessage != errorMessage {
		t.Fatalf("expected detail error message, got %#v", result.ErrorMessage)
	}
	if result.Sections.CandidateInfo.TaskErrorMessage == nil || *result.Sections.CandidateInfo.TaskErrorMessage != errorMessage {
		t.Fatalf("expected section error message, got %#v", result.Sections.CandidateInfo.TaskErrorMessage)
	}
	if result.Sections.Fallback.ShouldUseMarkdownFallback {
		t.Fatal("failed tasks must show their error rather than a Markdown fallback")
	}
	if result.Sections.Resume.TextAvailable || result.Sections.Resume.HighlightAvailable {
		t.Fatalf("expected unavailable resume state, got %#v", result.Sections.Resume)
	}
	if result.Sections.CandidateAnalysis.Strengths == nil || result.Sections.CandidateAnalysis.Weaknesses == nil || result.Sections.CandidateAnalysis.Risks == nil || result.Sections.CandidateAnalysis.SuggestedInterviewQuestions == nil {
		t.Fatalf("expected empty arrays, got %#v", result.Sections.CandidateAnalysis)
	}
}

func TestScreeningTaskDetailKeepsStructuredSectionsWithoutResumeText(t *testing.T) {
	requirements := `[
		{"id":"r1","label":"学历：本科","status":"pass","comment":"符合本科及以上学历要求。","evidence":[{"text":"嘉应学院·软件工程(本科)","start":null,"end":null}]}
	]`
	rawResponse := `{"output":{"markdown_report":"# Markdown 报告"}}`
	service := &screeningTaskService{repo: &stubScreeningTaskRepository{detail: &repository.ScreeningTaskDetailItem{
		ID:           32,
		Status:       ScreeningTaskStatusSuccess,
		Requirements: &requirements,
		RawResponse:  &rawResponse,
	}}}

	result, err := service.Detail(context.Background(), 32)
	if err != nil {
		t.Fatalf("get screening detail: %v", err)
	}
	if result.Sections.Fallback.ShouldUseMarkdownFallback {
		t.Fatal("available structured requirements must not fall back only because resume text is missing")
	}
	if result.Sections.Resume.TextAvailable || result.Sections.Resume.HighlightAvailable {
		t.Fatalf("expected unavailable resume/highlight state, got %#v", result.Sections.Resume)
	}
	item := result.Sections.RequirementsComparison.Items[0]
	if len(item.Evidence) != 0 {
		t.Fatalf("expected unverified evidence to be removed, got %#v", item.Evidence)
	}
	if item.CandidateSituation == nil || *item.CandidateSituation != "嘉应学院·软件工程(本科)" {
		t.Fatalf("expected candidate situation for table display, got %#v", item.CandidateSituation)
	}
}

func TestBuildRequirementsJSONVerifiesEvidenceAndUsesUTF16Offsets(t *testing.T) {
	comment := "符合本科要求。"
	resumeText := "姓名：杨诺雯\n嘉应学院·软件工程(本科)"
	stored := buildRequirementsJSON([]aiRequirement{{
		ID:      "r1",
		Label:   "学历：本科",
		Status:  "pass",
		Comment: &comment,
		Evidence: []aiEvidence{
			{Text: "嘉应学院·软件工程(本科)"},
			{Text: "不存在的证据"},
		},
	}}, resumeText)
	if stored == nil {
		t.Fatal("expected normalized requirements")
	}

	requirements := parseStoredRequirements(stored)
	if len(requirements) != 1 || len(requirements[0].Evidence) != 1 {
		t.Fatalf("expected only the verified evidence, got %#v", requirements)
	}
	evidence := requirements[0].Evidence[0]
	if evidence.Start == nil || evidence.End == nil || *evidence.Start != 7 || *evidence.End != 20 {
		t.Fatalf("expected UTF-16 offsets [7,20], got %#v", evidence)
	}
	if requirements[0].CandidateSituation == nil || *requirements[0].CandidateSituation != "嘉应学院·软件工程(本科)" {
		t.Fatalf("expected situation from verified evidence, got %#v", requirements[0].CandidateSituation)
	}

	withoutResumeText := buildRequirementsJSON([]aiRequirement{{
		ID:       "r1",
		Label:    "学历：本科",
		Status:   "pass",
		Evidence: []aiEvidence{{Text: "嘉应学院·软件工程(本科)"}},
	}}, "")
	withoutResumeRequirements := parseStoredRequirements(withoutResumeText)
	if len(withoutResumeRequirements) != 1 || len(withoutResumeRequirements[0].Evidence) != 0 {
		t.Fatalf("expected no unverified evidence without resume text, got %#v", withoutResumeRequirements)
	}

	candidateSituation := "候选人简历中未体现 Flutter 或 Dart 经验。"
	withSituation := buildRequirementsJSON([]aiRequirement{{
		ID:                 "r2",
		Label:              "要会 Flutter",
		CandidateSituation: &candidateSituation,
		Status:             "miss",
	}}, "")
	withSituationRequirements := parseStoredRequirements(withSituation)
	if len(withSituationRequirements) != 1 || withSituationRequirements[0].CandidateSituation == nil || *withSituationRequirements[0].CandidateSituation != candidateSituation {
		t.Fatalf("expected candidate_situation from AI output, got %#v", withSituationRequirements)
	}
}

func TestScreeningTaskDetailHandlesMalformedStoredDataSafely(t *testing.T) {
	malformedRequirements := `not-json`
	malformedStringSlice := `{not-json}`
	malformedRawResponse := `{not-json}`
	service := &screeningTaskService{repo: &stubScreeningTaskRepository{detail: &repository.ScreeningTaskDetailItem{
		ID:           33,
		Status:       ScreeningTaskStatusSuccess,
		Requirements: &malformedRequirements,
		Strengths:    &malformedStringSlice,
		Weaknesses:   &malformedStringSlice,
		Risks:        &malformedStringSlice,
		RawResponse:  &malformedRawResponse,
	}}}

	result, err := service.Detail(context.Background(), 33)
	if err != nil {
		t.Fatalf("get screening detail: %v", err)
	}
	if result.Requirements == nil || len(result.Requirements) != 0 {
		t.Fatalf("expected safe empty requirements, got %#v", result.Requirements)
	}
	if result.Sections.RequirementsComparison.Items == nil || result.Sections.RequirementsComparison.MatchedItems == nil || result.Sections.RequirementsComparison.AttentionItems == nil {
		t.Fatalf("expected non-nil requirement arrays, got %#v", result.Sections.RequirementsComparison)
	}
	if result.Sections.CandidateAnalysis.Strengths == nil || result.Sections.CandidateAnalysis.Weaknesses == nil || result.Sections.CandidateAnalysis.Risks == nil || result.Sections.CandidateAnalysis.SuggestedInterviewQuestions == nil {
		t.Fatalf("expected safe empty analysis arrays, got %#v", result.Sections.CandidateAnalysis)
	}
	if result.MarkdownReport != nil || result.Sections.Fallback.MarkdownReport != nil {
		t.Fatalf("expected malformed raw response to omit Markdown, got %#v", result.Sections.Fallback)
	}
}

func TestScreeningTaskDetailTreatsNilRepositoryItemAsNotFound(t *testing.T) {
	service := &screeningTaskService{repo: &stubScreeningTaskRepository{}}
	_, err := service.Detail(context.Background(), 34)
	if !errors.Is(err, ErrScreeningTaskNotFound) {
		t.Fatalf("expected not found for nil detail item, got %v", err)
	}
}

type stubScreeningTaskRepository struct {
	repository.ScreeningTaskRepository
	nextID        int64
	created       *model.ScreeningResult
	detail        *repository.ScreeningTaskDetailItem
	detailErr     error
	runningIDs    []int64
	successID     int64
	successUpdate repository.ScreeningResultSuccessUpdate
	failedID      int64
	failedMessage string
}

func (r *stubScreeningTaskRepository) Create(ctx context.Context, result *model.ScreeningResult) error {
	if r.nextID != 0 {
		result.ID = r.nextID
	}
	copied := *result
	r.created = &copied
	return nil
}

func (r *stubScreeningTaskRepository) MarkRunning(ctx context.Context, id int64) error {
	r.runningIDs = append(r.runningIDs, id)
	return nil
}

func (r *stubScreeningTaskRepository) MarkSuccess(ctx context.Context, id int64, update repository.ScreeningResultSuccessUpdate) error {
	r.successID = id
	r.successUpdate = update
	return nil
}

func (r *stubScreeningTaskRepository) MarkFailed(ctx context.Context, id int64, message string) error {
	r.failedID = id
	r.failedMessage = message
	return nil
}

func (r *stubScreeningTaskRepository) FindDetailByID(ctx context.Context, id int64) (*repository.ScreeningTaskDetailItem, error) {
	return r.detail, r.detailErr
}

type stubResumeRepository struct {
	repository.ResumeRepository
	resume *model.Resume
	err    error
}

func (r *stubResumeRepository) FindByID(ctx context.Context, id int64) (*model.Resume, error) {
	return r.resume, r.err
}

type stubJobRepository struct {
	repository.JobRepository
	job     *repository.JobDetailItem
	tags    []repository.JobTagWithTag
	err     error
	tagsErr error
}

func (r *stubJobRepository) FindByID(ctx context.Context, id int64) (*repository.JobDetailItem, error) {
	return r.job, r.err
}

func (r *stubJobRepository) ListTags(ctx context.Context, jobID int64) ([]repository.JobTagWithTag, error) {
	return r.tags, r.tagsErr
}

type stubApplicationRepository struct {
	repository.ApplicationRepository
	application *model.Application
	err         error
}

func (r *stubApplicationRepository) FindOrCreateForScreening(ctx context.Context, jobID int64, resumeID int64, candidateID *int64, createdBy int64) (*model.Application, error) {
	return r.application, r.err
}

type stubUploader struct {
	storage.Uploader
	object *storage.Object
	err    error
}

func (u *stubUploader) Open(ctx context.Context, key string) (*storage.Object, error) {
	return u.object, u.err
}

type stubDifyClient struct {
	calls      int
	resultText string
	err        error
	lastReq    dify.RunResumeScreeningRequest
}

func (c *stubDifyClient) RunResumeScreening(ctx context.Context, req dify.RunResumeScreeningRequest) (*dify.RunResumeScreeningResponse, error) {
	c.calls++
	c.lastReq = req
	return &dify.RunResumeScreeningResponse{ResultText: c.resultText}, c.err
}
