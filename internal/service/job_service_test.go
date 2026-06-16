package service

import (
	"encoding/json"
	"testing"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
	"gorm.io/datatypes"
)

func TestNormalizeJobQueryEmptyStatusDisablesStatusFilter(t *testing.T) {
	query := normalizeJobQuery(dto.JobQuery{})

	if query.Status != "" {
		t.Fatalf("expected empty status to disable filtering, got %q", query.Status)
	}
}

func TestNormalizeJobQueryAllStatusDisablesStatusFilter(t *testing.T) {
	query := normalizeJobQuery(dto.JobQuery{
		Status: " all ",
	})

	if query.Status != "" {
		t.Fatalf("expected all status to disable filtering, got %q", query.Status)
	}
}

func TestBuildJobScreeningContextResponseIncludesDynamicFieldsAndTags(t *testing.T) {
	job := &repository.JobDetailItem{
		Job: model.Job{
			ID:               12,
			Title:            "Go Backend Engineer",
			Headcount:        2,
			ExperienceMin:    int32Ptr(3),
			ExperienceMax:    int32Ptr(5),
			Requirements:     jobStringPtr("熟悉 Go、PostgreSQL、Redis"),
			Responsibilities: jobStringPtr("负责招聘系统后端服务开发"),
			Priority:         "high",
			DynamicFields: datatypes.JSONMap{
				"must_have_skills": []interface{}{"Go", "PostgreSQL"},
				"nice_to_have":     []interface{}{"Docker"},
			},
		},
	}
	tags := []repository.JobTagWithTag{
		{TagID: 7, Name: "后端"},
	}

	resp, err := buildJobScreeningContextResponse(job, tags)
	if err != nil {
		t.Fatalf("build job screening context: %v", err)
	}
	if resp.JobID != job.ID || resp.JobTitle != job.Title {
		t.Fatalf("unexpected response identifiers: %#v", resp)
	}
	if len(resp.Payload.Tags) != 1 || resp.Payload.Tags[0].Name != "后端" {
		t.Fatalf("expected tag in payload, got %#v", resp.Payload.Tags)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(resp.JobContext), &payload); err != nil {
		t.Fatalf("job context should be valid JSON: %v", err)
	}
	if payload["job_title"] != "Go Backend Engineer" {
		t.Fatalf("expected job title in job context, got %v", payload["job_title"])
	}

	dynamicFields, ok := payload["dynamic_fields"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected dynamic fields in job context, got %#v", payload["dynamic_fields"])
	}
	if _, ok := dynamicFields["must_have_skills"]; !ok {
		t.Fatalf("expected must_have_skills in dynamic fields, got %#v", dynamicFields)
	}
}

func int32Ptr(value int32) *int32 {
	return &value
}

func jobStringPtr(value string) *string {
	return &value
}
