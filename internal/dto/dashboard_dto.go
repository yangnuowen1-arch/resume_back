package dto

import "time"

// DashboardSummaryResponse contains the live totals displayed on the dashboard.
// Every value is calculated when the endpoint is requested; it is not a cached
// or asynchronously maintained aggregate.
type DashboardSummaryResponse struct {
	TotalResumes     int64     `json:"totalResumes"`
	PendingScreening int64     `json:"pendingScreening"`
	Recommended      int64     `json:"recommended"`
	Rejected         int64     `json:"rejected"`
	GeneratedAt      time.Time `json:"generatedAt"`
}
