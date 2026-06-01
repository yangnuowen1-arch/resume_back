package service

import (
	"errors"
	"strings"
)

const (
	statusActive   = "active"
	statusDisabled = "disabled"
)

const (
	CandidateStatusNew           = "new"
	CandidateStatusPendingReview = "pending_review"
	CandidateStatusEvaluating    = "evaluating"
	CandidateStatusEvaluated     = "evaluated"
	CandidateStatusInterview     = "interview"
	CandidateStatusOffered       = "offered"
	CandidateStatusHired         = "hired"
	CandidateStatusRejected      = "rejected"
)

var candidateStatusLabels = map[string]string{
	CandidateStatusNew:           "新候选人",
	CandidateStatusPendingReview: "待评估",
	CandidateStatusEvaluating:    "评估中",
	CandidateStatusEvaluated:     "已评估",
	CandidateStatusInterview:     "面试中",
	CandidateStatusOffered:       "已发 Offer",
	CandidateStatusHired:         "已录用",
	CandidateStatusRejected:      "已拒绝",
}

func normalizeActiveDisabledStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return statusActive
	}

	return status
}

func normalizeStatusFilter(status string) string {
	status = strings.TrimSpace(status)
	if status == "all" {
		return ""
	}

	return status
}

func validateActiveDisabledStatus(status string, fieldName string) error {
	if status == statusActive || status == statusDisabled {
		return nil
	}

	return errors.New(fieldName + "只能是 active 或 disabled")
}

func normalizeCandidateStatus(status string, defaultStatus string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return defaultStatus
	}

	return status
}

func validateCandidateStatus(status string) error {
	if _, ok := candidateStatusLabels[status]; ok {
		return nil
	}

	return newInvalidParameterError("候选人状态不合法")
}
