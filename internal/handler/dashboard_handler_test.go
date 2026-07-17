package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangnuowen1-arch/resume_back/internal/auth"
	"github.com/yangnuowen1-arch/resume_back/internal/config"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/middleware"
)

func TestDashboardSummaryReturnsLivePayloadWithoutCaching(t *testing.T) {
	gin.SetMode(gin.TestMode)
	generatedAt := time.Date(2026, time.July, 16, 8, 0, 0, 0, time.UTC)
	service := &testDashboardService{summary: &dto.DashboardSummaryResponse{
		TotalResumes:     1247,
		PendingScreening: 38,
		Recommended:      156,
		Rejected:         892,
		GeneratedAt:      generatedAt,
	}}
	router := newDashboardTestRouter(NewDashboardHandler(service))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary", nil)
	request.Header.Set("Authorization", "Bearer "+testDashboardToken(t))
	result := httptest.NewRecorder()
	router.ServeHTTP(result, request)

	if result.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", result.Code, result.Body.String())
	}
	if result.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control=%q, want no-store", result.Header().Get("Cache-Control"))
	}

	var payload struct {
		Code int                          `json:"code"`
		Data dto.DashboardSummaryResponse `json:"data"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Code != 0 {
		t.Fatalf("code=%d, want 0", payload.Code)
	}
	if payload.Data.TotalResumes != 1247 || payload.Data.PendingScreening != 38 || payload.Data.Recommended != 156 || payload.Data.Rejected != 892 {
		t.Fatalf("unexpected dashboard data: %#v", payload.Data)
	}
	if !payload.Data.GeneratedAt.Equal(generatedAt) {
		t.Fatalf("generatedAt=%s, want %s", payload.Data.GeneratedAt, generatedAt)
	}
}

func TestDashboardSummaryRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newDashboardTestRouter(NewDashboardHandler(&testDashboardService{}))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary", nil)
	result := httptest.NewRecorder()
	router.ServeHTTP(result, request)

	if result.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", result.Code, result.Body.String())
	}
}

func TestDashboardSummaryHandlesServiceFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newDashboardTestRouter(NewDashboardHandler(&testDashboardService{err: errors.New("database unavailable")}))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary", nil)
	request.Header.Set("Authorization", "Bearer "+testDashboardToken(t))
	result := httptest.NewRecorder()
	router.ServeHTTP(result, request)

	if result.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", result.Code, result.Body.String())
	}
}

func newDashboardTestRouter(h *DashboardHandler) *gin.Engine {
	router := gin.New()
	api := router.Group("/api/v1")
	private := api.Group("")
	private.Use(middleware.AuthMiddleware(&config.Config{JWTSecret: "dashboard-test-secret"}))
	private.GET("/dashboard/summary", h.Summary)
	return router
}

func testDashboardToken(t *testing.T) string {
	t.Helper()
	token, err := auth.GenerateToken(7, "tester", []string{"user"}, "dashboard-test-secret", "1")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	return token
}

type testDashboardService struct {
	summary *dto.DashboardSummaryResponse
	err     error
}

func (s *testDashboardService) Summary(context.Context) (*dto.DashboardSummaryResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.summary, nil
}
