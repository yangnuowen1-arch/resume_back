package service

import (
	"context"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
)

type DashboardService interface {
	Summary(ctx context.Context) (*dto.DashboardSummaryResponse, error)
}

type dashboardService struct {
	repo repository.DashboardRepository
	now  func() time.Time
}

func NewDashboardService(repo repository.DashboardRepository) DashboardService {
	return &dashboardService{
		repo: repo,
		now:  time.Now,
	}
}

func (s *dashboardService) Summary(ctx context.Context) (*dto.DashboardSummaryResponse, error) {
	summary, err := s.repo.Summary(ctx)
	if err != nil {
		return nil, err
	}

	return &dto.DashboardSummaryResponse{
		TotalResumes:     summary.TotalResumes,
		PendingScreening: summary.PendingScreening,
		Recommended:      summary.Recommended,
		Rejected:         summary.Rejected,
		GeneratedAt:      s.now().UTC(),
	}, nil
}
