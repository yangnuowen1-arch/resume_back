package repository

import (
	"context"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"gorm.io/gorm"
)

type MailboxScanTaskRepository interface {
	Create(ctx context.Context, task *model.MailboxScanTask) error
	FindByID(ctx context.Context, id int64) (*model.MailboxScanTask, error)
	MarkRunning(ctx context.Context, id int64) error
	MarkDone(ctx context.Context, id int64, scanned, imported, skipped int) error
	MarkFailed(ctx context.Context, id int64, scanned, imported, skipped int, errorMessage string) error
}

type mailboxScanTaskRepository struct {
	db *gorm.DB
}

func NewMailboxScanTaskRepository(db *gorm.DB) MailboxScanTaskRepository {
	return &mailboxScanTaskRepository{db: db}
}

func (r *mailboxScanTaskRepository) Create(ctx context.Context, task *model.MailboxScanTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *mailboxScanTaskRepository) FindByID(ctx context.Context, id int64) (*model.MailboxScanTask, error) {
	var task model.MailboxScanTask
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *mailboxScanTaskRepository) MarkRunning(ctx context.Context, id int64) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.MailboxScanTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     "running",
			"started_at": now,
		}).Error
}

func (r *mailboxScanTaskRepository) MarkDone(ctx context.Context, id int64, scanned, imported, skipped int) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.MailboxScanTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      "done",
			"scanned":     scanned,
			"imported":    imported,
			"skipped":     skipped,
			"finished_at": now,
		}).Error
}

func (r *mailboxScanTaskRepository) MarkFailed(ctx context.Context, id int64, scanned, imported, skipped int, errorMessage string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.MailboxScanTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      "failed",
			"scanned":     scanned,
			"imported":    imported,
			"skipped":     skipped,
			"error":       errorMessage,
			"finished_at": now,
		}).Error
}
