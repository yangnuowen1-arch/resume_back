package service

import (
	"testing"

	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
)

func TestNormalizeOperationLogQuery(t *testing.T) {
	query := normalizeOperationLogQuery(dto.OperationLogQuery{
		Page:     0,
		PageSize: 500,
		User:     " alice ",
	})

	if query.Page != 1 {
		t.Fatalf("expected page 1, got %d", query.Page)
	}
	if query.PageSize != 20 {
		t.Fatalf("expected default page size 20, got %d", query.PageSize)
	}
	if query.User != "alice" {
		t.Fatalf("expected trimmed user filter, got %q", query.User)
	}
}

func TestBuildOperationLogDetails(t *testing.T) {
	module := "users"
	targetType := "user"
	targetID := int64(42)
	ip := "127.0.0.1"
	afterData := `{"status":"disabled"}`

	details := buildOperationLogDetails(repository.OperationLogListItem{
		Module:     &module,
		TargetType: &targetType,
		TargetID:   &targetID,
		IPAddress:  &ip,
		AfterData:  &afterData,
	})

	expected := "Module: users | Target: user #42 | IP: 127.0.0.1 | Data changed"
	if details != expected {
		t.Fatalf("expected details %q, got %q", expected, details)
	}
}
