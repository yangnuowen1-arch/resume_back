package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/repository"
)

func TestDashboardSummaryReadsCurrentRepositoryStateOnEveryRequest(t *testing.T) {
	repo := &testDashboardRepository{
		summary: repository.DashboardSummary{
			TotalResumes:     4,
			PendingScreening: 2,
			Recommended:      1,
			Rejected:         3,
		},
	}
	service := NewDashboardService(repo).(*dashboardService)
	now := time.Date(2026, time.July, 16, 9, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	service.now = func() time.Time { return now }

	first, err := service.Summary(context.Background())
	if err != nil {
		t.Fatalf("first summary: %v", err)
	}
	if first.TotalResumes != 4 || first.PendingScreening != 2 || first.Recommended != 1 || first.Rejected != 3 {
		t.Fatalf("unexpected first summary: %#v", first)
	}
	if !first.GeneratedAt.Equal(now.UTC()) {
		t.Fatalf("generatedAt=%s, want %s", first.GeneratedAt, now.UTC())
	}

	// Simulate writes completed after the first dashboard request. The second
	// read must use the repository again rather than returning a cached result.
	repo.summary = repository.DashboardSummary{
		TotalResumes:     5,
		PendingScreening: 1,
		Recommended:      2,
		Rejected:         4,
	}

	second, err := service.Summary(context.Background())
	if err != nil {
		t.Fatalf("second summary: %v", err)
	}
	if second.TotalResumes != 5 || second.PendingScreening != 1 || second.Recommended != 2 || second.Rejected != 4 {
		t.Fatalf("unexpected second summary: %#v", second)
	}
	if repo.calls != 2 {
		t.Fatalf("repository calls=%d, want 2", repo.calls)
	}
}

func TestDashboardSummaryReturnsRepositoryError(t *testing.T) {
	want := errors.New("database unavailable")
	service := NewDashboardService(&testDashboardRepository{err: want})

	if _, err := service.Summary(context.Background()); !errors.Is(err, want) {
		t.Fatalf("summary error=%v, want %v", err, want)
	}
}

type testDashboardRepository struct {
	summary repository.DashboardSummary
	err     error
	calls   int
}

func (r *testDashboardRepository) Summary(context.Context) (repository.DashboardSummary, error) {
	r.calls++
	if r.err != nil {
		return repository.DashboardSummary{}, r.err
	}
	return r.summary, nil
}

var _ repository.DashboardRepository = (*testDashboardRepository)(nil)
