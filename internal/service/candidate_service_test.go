package service

import (
	"errors"
	"testing"

	"github.com/yangnuowen1-arch/resume_back/internal/dto"
)

func TestValidateCandidateEnumsAcceptsConfiguredValues(t *testing.T) {
	req := dto.CreateCandidateRequest{
		Gender:           stringPtr(CandidateGenderMale),
		Source:           stringPtr(CandidateSourceEmail),
		HighestEducation: stringPtr(CandidateEducationBachelor),
	}

	if err := validateCandidateEnums(req); err != nil {
		t.Fatalf("validate candidate enums: %v", err)
	}
}

func TestValidateCandidateEnumsRejectsDisplaySourceLabel(t *testing.T) {
	req := dto.CreateCandidateRequest{
		Source: stringPtr("邮箱"),
	}
	normalizeCreateCandidateRequest(&req)

	err := validateCandidateEnums(req)
	if !errors.Is(err, ErrInvalidParameter) {
		t.Fatalf("expected invalid parameter error, got %v", err)
	}
}

func TestNormalizeCandidateSourceFilter(t *testing.T) {
	source := normalizeCandidateSourceFilter(" Email ")
	if source != CandidateSourceEmail {
		t.Fatalf("expected normalized source %q, got %q", CandidateSourceEmail, source)
	}
	if err := validateCandidateSourceValue(source); err != nil {
		t.Fatalf("expected normalized source to be valid: %v", err)
	}
}

func stringPtr(value string) *string {
	return &value
}
