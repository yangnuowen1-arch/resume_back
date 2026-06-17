package service

import (
	"context"
	"errors"
	"testing"

	"github.com/yangnuowen1-arch/resume_back/internal/auth"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
)

func TestValidateCandidateEnumsAcceptsConfiguredValues(t *testing.T) {
	req := dto.CreateCandidateRequest{
		Gender:           stringPtr(CandidateGenderMale),
		Source:           stringPtr(CandidateSourceEmail),
		HighestEducation: stringPtr(CandidateEducationBachelor),
	}

	if err := validateCandidateEnums(req); err != nil {
		t.Fatalf("validate candidate enums: %v", err)
	}
}

func TestValidateCandidateEnumsRejectsDisplaySourceLabel(t *testing.T) {
	req := dto.CreateCandidateRequest{
		Source: stringPtr("邮箱"),
	}
	normalizeCreateCandidateRequest(&req)

	err := validateCandidateEnums(req)
	if !errors.Is(err, ErrInvalidParameter) {
		t.Fatalf("expected invalid parameter error, got %v", err)
	}
}

func TestNormalizeCandidateSourceFilter(t *testing.T) {
	source := normalizeCandidateSourceFilter(" Email ")
	if source != CandidateSourceEmail {
		t.Fatalf("expected normalized source %q, got %q", CandidateSourceEmail, source)
	}
	if err := validateCandidateSourceValue(source); err != nil {
		t.Fatalf("expected normalized source to be valid: %v", err)
	}
}

func TestBatchAnalyzeEnqueuesScreeningTasks(t *testing.T) {
	resumeID := int64(11)
	applicationID := int64(12)
	screeningResultID := int64(13)
	jobID := int64(14)
	parseStatus := ResumeParseStatusParsed
	repo := &stubCandidateRepository{
		result: repository.CandidateAnalysisResult{
			CandidateID:       1,
			ResumeID:          &resumeID,
			ApplicationID:     &applicationID,
			ScreeningResultID: &screeningResultID,
			JobID:             &jobID,
			ParseStatus:       &parseStatus,
			Status:            ScreeningTaskStatusQueued,
		},
	}
	enqueuer := &stubScreeningTaskEnqueuer{}
	service := NewCandidateService(repo, CandidateServiceDependencies{
		ScreeningTaskEnqueuer: enqueuer,
	})

	ctx := auth.WithClaims(context.Background(), &auth.Claims{UserID: 7})
	resp, err := service.BatchAnalyze(ctx, dto.BatchAnalyzeCandidatesRequest{
		CandidateIDs: []int64{1},
		JobID:        &jobID,
	})
	if err != nil {
		t.Fatalf("batch analyze: %v", err)
	}

	if resp.Queued != 1 || resp.Failed != 0 {
		t.Fatalf("expected one queued and zero failed, got queued=%d failed=%d", resp.Queued, resp.Failed)
	}
	if len(enqueuer.jobs) != 1 {
		t.Fatalf("expected one enqueued job, got %d", len(enqueuer.jobs))
	}
	if enqueuer.jobs[0].ScreeningResultID != screeningResultID ||
		enqueuer.jobs[0].ResumeID != resumeID ||
		enqueuer.jobs[0].JobID != jobID {
		t.Fatalf("unexpected enqueued job: %#v", enqueuer.jobs[0])
	}
	if len(resp.Items) != 1 || resp.Items[0].ScreeningResultID == nil || *resp.Items[0].ScreeningResultID != screeningResultID {
		t.Fatalf("expected response item to include screening result ID, got %#v", resp.Items)
	}
}

func stringPtr(value string) *string {
	return &value
}

type stubCandidateRepository struct {
	repository.CandidateRepository
	result repository.CandidateAnalysisResult
}

func (r *stubCandidateRepository) EnqueueScreening(ctx context.Context, candidateID int64, jobID *int64, createdBy int64, candidateStatus string) repository.CandidateAnalysisResult {
	return r.result
}

type stubScreeningTaskEnqueuer struct {
	jobs []ScreeningTaskQueueJob
	err  error
}

func (e *stubScreeningTaskEnqueuer) EnqueueResumeScreening(ctx context.Context, job ScreeningTaskQueueJob) error {
	if e.err != nil {
		return e.err
	}
	e.jobs = append(e.jobs, job)
	return nil
}
