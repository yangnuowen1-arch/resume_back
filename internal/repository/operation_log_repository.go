package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"gorm.io/gorm"
)

type OperationLogListItem struct {
	ID         int64
	UserID     *int64
	Username   *string
	RealName   *string
	Action     string
	Module     *string
	TargetType *string
	TargetID   *int64
	BeforeData *string
	AfterData  *string
	IPAddress  *string
	UserAgent  *string
	CreatedAt  time.Time
}

type OperationLogListFilter struct {
	User     string
	Page     int
	PageSize int
}

type OperationLogRepository interface {
	Create(ctx context.Context, operationLog *model.OperationLog) error
	List(ctx context.Context, filter OperationLogListFilter) ([]OperationLogListItem, int64, error)
}

type operationLogRepository struct {
	db *gorm.DB
}

func NewOperationLogRepository(db *gorm.DB) OperationLogRepository {
	return &operationLogRepository{
		db: db,
	}
}

func (r *operationLogRepository) Create(ctx context.Context, operationLog *model.OperationLog) error {
	values := map[string]interface{}{
		"user_id":     operationLog.UserID,
		"action":      operationLog.Action,
		"module":      operationLog.Module,
		"target_type": operationLog.TargetType,
		"target_id":   operationLog.TargetID,
		"ip_address":  operationLog.IPAddress,
		"user_agent":  operationLog.UserAgent,
	}

	if operationLog.BeforeData != nil {
		values["before_data"] = gorm.Expr("?::jsonb", *operationLog.BeforeData)
	}
	if operationLog.AfterData != nil {
		values["after_data"] = gorm.Expr("?::jsonb", *operationLog.AfterData)
	}

	return r.db.WithContext(ctx).Table(model.TableNameOperationLog).Create(values).Error
}

func (r *operationLogRepository) List(ctx context.Context, filter OperationLogListFilter) ([]OperationLogListItem, int64, error) {
	queryBuilder := r.db.WithContext(ctx).
		Table(model.TableNameOperationLog).
		Joins("LEFT JOIN " + model.TableNameUser + " ON users.id = operation_logs.user_id")

	if filter.User != "" {
		like := "%" + filter.User + "%"
		queryBuilder = queryBuilder.Where(
			"(users.username LIKE ? OR users.real_name LIKE ? OR CAST(operation_logs.user_id AS TEXT) = ?)",
			like,
			like,
			filter.User,
		)

		if userID, err := strconv.ParseInt(filter.User, 10, 64); err == nil && userID > 0 {
			queryBuilder = queryBuilder.Or("operation_logs.user_id = ?", userID)
		}
	}

	var total int64
	if err := queryBuilder.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]OperationLogListItem, 0)
	err := queryBuilder.
		Select("operation_logs.id, operation_logs.user_id, users.username, users.real_name, operation_logs.action, operation_logs.module, operation_logs.target_type, operation_logs.target_id, operation_logs.before_data, operation_logs.after_data, operation_logs.ip_address, operation_logs.user_agent, operation_logs.created_at").
		Order("operation_logs.created_at DESC, operation_logs.id DESC").
		Limit(filter.PageSize).
		Offset((filter.Page - 1) * filter.PageSize).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
