package service

import (
	"context"
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
	fileType := "application/pdf"
	repo := &stubScreeningTaskRepository{}
	difyClient := &stubDifyClient{
		resultText: `{"score":91,"match_level":"strong","recommendation":"recommend_interview","summary":"Good fit","strengths":["Go"],"weaknesses":[],"risks":[],"missing_requirements":[]}`,
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
	if difyClient.calls != 1 {
		t.Fatalf("expected one Dify call, got %d", difyClient.calls)
	}
	if difyClient.lastReq.OutputLanguage != "English" {
		t.Fatalf("expected output language English, got %q", difyClient.lastReq.OutputLanguage)
	}
}

type stubScreeningTaskRepository struct {
	repository.ScreeningTaskRepository
	nextID        int64
	created       *model.ScreeningResult
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
