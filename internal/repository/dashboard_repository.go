package repository

import (
	"context"

	"gorm.io/gorm"
)

// DashboardSummary is the current aggregate state used by the dashboard cards.
// The values are deliberately read directly from the source tables rather than
// being persisted separately, so the summary cannot drift from the application
// data.
type DashboardSummary struct {
	TotalResumes     int64 `gorm:"column:total_resumes"`
	PendingScreening int64 `gorm:"column:pending_screening"`
	Recommended      int64 `gorm:"column:recommended"`
	Rejected         int64 `gorm:"column:rejected"`
}

type DashboardRepository interface {
	Summary(ctx context.Context) (DashboardSummary, error)
}

type dashboardRepository struct {
	db *gorm.DB
}

func NewDashboardRepository(db *gorm.DB) DashboardRepository {
	return &dashboardRepository{db: db}
}

// dashboardSummaryQuery performs all counts in one PostgreSQL statement. This
// gives the cards one database snapshot and avoids a partially updated response
// when records are changed while the dashboard is being refreshed.
//
// Recommended counts the latest screening result for each application. Re-runs
// therefore replace the application's current recommendation instead of
// inflating the KPI with historical attempts.
const dashboardSummaryQuery = `
WITH latest_screening_results AS (
	SELECT DISTINCT ON (application_id)
		application_id,
		status,
		recommendation
	FROM screening_results
	ORDER BY application_id, created_at DESC, id DESC
)
SELECT
	(SELECT COUNT(*) FROM resumes) AS total_resumes,
	(SELECT COUNT(*) FROM screening_results WHERE status IN ('queued', 'running')) AS pending_screening,
	(
		SELECT COUNT(*)
		FROM latest_screening_results
		WHERE status = 'success'
			AND LOWER(BTRIM(COALESCE(recommendation, ''))) = 'recommend_interview'
	) AS recommended,
	(SELECT COUNT(*) FROM candidates WHERE status = 'rejected') AS rejected
`

func (r *dashboardRepository) Summary(ctx context.Context) (DashboardSummary, error) {
	var summary DashboardSummary
	err := r.db.WithContext(ctx).Raw(dashboardSummaryQuery).Scan(&summary).Error
	return summary, err
}
