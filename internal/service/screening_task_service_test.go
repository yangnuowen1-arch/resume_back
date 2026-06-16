package service

import (
	"testing"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
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
