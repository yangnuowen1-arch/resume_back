package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
)

type OperationLogService interface {
	Record(ctx context.Context, req dto.RecordOperationLogRequest) error
	List(ctx context.Context, query dto.OperationLogQuery) ([]dto.OperationLogResponse, int64, error)
}

type operationLogService struct {
	repo repository.OperationLogRepository
}

func NewOperationLogService(repo repository.OperationLogRepository) OperationLogService {
	return &operationLogService{
		repo: repo,
	}
}

func (s *operationLogService) Record(ctx context.Context, req dto.RecordOperationLogRequest) error {
	userID, err := currentUserID(ctx)
	if err != nil {
		return err
	}

	req = normalizeRecordOperationLogRequest(req)
	if req.Action == "" {
		return errors.New("操作类型不能为空")
	}

	return s.repo.Create(ctx, &model.OperationLog{
		UserID:     &userID,
		Action:     req.Action,
		Module:     req.Module,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		BeforeData: req.BeforeData,
		AfterData:  req.AfterData,
		IPAddress:  req.IPAddress,
		UserAgent:  req.UserAgent,
	})
}

func (s *operationLogService) List(ctx context.Context, query dto.OperationLogQuery) ([]dto.OperationLogResponse, int64, error) {
	if _, err := currentUserID(ctx); err != nil {
		return nil, 0, err
	}

	query = normalizeOperationLogQuery(query)
	items, total, err := s.repo.List(ctx, repository.OperationLogListFilter{
		User:     strings.TrimSpace(query.User),
		Date:     query.Date,
		Page:     query.Page,
		PageSize: query.PageSize,
	})
	if err != nil {
		return nil, 0, err
	}

	result := make([]dto.OperationLogResponse, 0, len(items))
	for _, item := range items {
		result = append(result, dto.OperationLogResponse{
			ID:         item.ID,
			Timestamp:  item.CreatedAt,
			UserID:     item.UserID,
			User:       displayOperationLogUser(item),
			Action:     item.Action,
			Details:    buildOperationLogDetails(item),
			Module:     item.Module,
			TargetType: item.TargetType,
			TargetID:   item.TargetID,
			IPAddress:  item.IPAddress,
			UserAgent:  item.UserAgent,
			BeforeData: item.BeforeData,
			AfterData:  item.AfterData,
			CreatedAt:  item.CreatedAt,
		})
	}

	return result, total, nil
}

func normalizeOperationLogQuery(query dto.OperationLogQuery) dto.OperationLogQuery {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 200 {
		query.PageSize = 20
	}
	query.User = strings.TrimSpace(query.User)

	return query
}

func normalizeRecordOperationLogRequest(req dto.RecordOperationLogRequest) dto.RecordOperationLogRequest {
	req.Action = truncateString(strings.TrimSpace(req.Action), 100)
	req.Module = trimOptionalStringMax(req.Module, 100)
	req.TargetType = trimOptionalStringMax(req.TargetType, 100)
	req.BeforeData = trimOptionalString(req.BeforeData)
	req.AfterData = trimOptionalString(req.AfterData)
	req.IPAddress = trimOptionalStringMax(req.IPAddress, 100)
	req.UserAgent = trimOptionalString(req.UserAgent)

	return req
}

func displayOperationLogUser(item repository.OperationLogListItem) string {
	if item.RealName != nil && strings.TrimSpace(*item.RealName) != "" {
		return strings.TrimSpace(*item.RealName)
	}
	if item.Username != nil && strings.TrimSpace(*item.Username) != "" {
		return strings.TrimSpace(*item.Username)
	}
	if item.UserID != nil {
		return fmt.Sprintf("User #%d", *item.UserID)
	}

	return "System"
}

func buildOperationLogDetails(item repository.OperationLogListItem) string {
	parts := make([]string, 0, 4)
	if item.Module != nil && strings.TrimSpace(*item.Module) != "" {
		parts = append(parts, "Module: "+strings.TrimSpace(*item.Module))
	}
	if item.TargetType != nil && strings.TrimSpace(*item.TargetType) != "" {
		target := "Target: " + strings.TrimSpace(*item.TargetType)
		if item.TargetID != nil {
			target += fmt.Sprintf(" #%d", *item.TargetID)
		}
		parts = append(parts, target)
	} else if item.TargetID != nil {
		parts = append(parts, fmt.Sprintf("Target ID: %d", *item.TargetID))
	}
	if item.IPAddress != nil && strings.TrimSpace(*item.IPAddress) != "" {
		parts = append(parts, "IP: "+strings.TrimSpace(*item.IPAddress))
	}
	if item.BeforeData != nil || item.AfterData != nil {
		parts = append(parts, "Data changed")
	}
	if len(parts) == 0 {
		return "-"
	}

	return strings.Join(parts, " | ")
}

func trimOptionalStringMax(value *string, maxLength int) *string {
	trimmed := trimOptionalString(value)
	if trimmed == nil {
		return nil
	}

	result := truncateString(*trimmed, maxLength)
	return &result
}

func truncateString(value string, maxLength int) string {
	if maxLength <= 0 || len(value) <= maxLength {
		return value
	}

	return value[:maxLength]
}
