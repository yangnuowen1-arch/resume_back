package repository

import (
	"strings"
	"testing"
)

func TestDashboardSummaryQueryUsesCurrentStateAndLatestRecommendation(t *testing.T) {
	for _, condition := range []string{
		"FROM resumes",
		"status IN ('queued', 'running')",
		"DISTINCT ON (application_id)",
		"status = 'success'",
		"LOWER(BTRIM(COALESCE(recommendation, ''))) = 'recommend_interview'",
		"FROM candidates WHERE status = 'rejected'",
	} {
		if !strings.Contains(dashboardSummaryQuery, condition) {
			t.Fatalf("dashboard summary query is missing %q", condition)
		}
	}
}
