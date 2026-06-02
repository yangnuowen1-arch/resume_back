package service

import (
	"testing"

	"github.com/yangnuowen1-arch/resume_back/internal/dto"
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
