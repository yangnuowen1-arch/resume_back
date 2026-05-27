package service

import (
	"errors"
	"strings"
)

const (
	statusActive   = "active"
	statusDisabled = "disabled"
)

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
