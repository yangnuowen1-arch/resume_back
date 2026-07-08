package repository

import (
	"context"
	"errors"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MailboxAccountRepository 管理已连接的 OAuth 邮箱账号。
type MailboxAccountRepository interface {
	Create(ctx context.Context, provider, email, accessToken string, refreshToken, tokenExpiry *string) error
	FindByID(ctx context.Context, id int64) (*model.MailboxAccount, error)
	FindByProviderEmail(ctx context.Context, provider, email string) (*model.MailboxAccount, error)
	List(ctx context.Context) ([]model.MailboxAccount, error)
	UpdateToken(ctx context.Context, id int64, accessToken string, refreshToken *string, tokenExpiry *time.Time) error
	UpdateTokenByID(ctx context.Context, id int64, accessToken string, refreshToken, tokenExpiry *string) error
	UpdateLastScanAt(ctx context.Context, id int64, scannedAt time.Time) error
	Delete(ctx context.Context, id int64) error
}

// MailboxMessageRepository 记录已处理邮件，防止重复扫描入库。
type MailboxMessageRepository interface {
	Exists(ctx context.Context, accountID int64, messageID string) (bool, error)
	// MarkProcessed 登记一封已处理邮件，返回 true 表示本次首次插入。
	MarkProcessed(ctx context.Context, accountID int64, messageID string) (bool, error)
}

type mailboxAccountRepository struct {
	db *gorm.DB
}

func NewMailboxAccountRepository(db *gorm.DB) MailboxAccountRepository {
	return &mailboxAccountRepository{db: db}
}

func (r *mailboxAccountRepository) Create(ctx context.Context, provider, email, accessToken string, refreshToken, tokenExpiry *string) error {
	now := time.Now()
	account := &model.MailboxAccount{
		Provider:    provider,
		Email:       email,
		AccessToken: accessToken,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if refreshToken != nil {
		account.RefreshToken = refreshToken
	}
	if tokenExpiry != nil {
		expiry, err := time.Parse("2006-01-02 15:04:05", *tokenExpiry)
		if err == nil {
			account.TokenExpiry = &expiry
		}
	}
	return r.db.WithContext(ctx).Create(account).Error
}

func (r *mailboxAccountRepository) FindByID(ctx context.Context, id int64) (*model.MailboxAccount, error) {
	account := &model.MailboxAccount{}
	err := r.db.WithContext(ctx).Where("id = ?", id).First(account).Error
	if err != nil {
		return nil, err
	}
	return account, nil
}

// FindByProviderEmail 按平台 + 邮箱查账号（大小写不敏感，对齐唯一索引）。
// 用于 OAuth 回调时判断是新增绑定还是刷新既有账号的 token；未命中返回 (nil, nil)。
func (r *mailboxAccountRepository) FindByProviderEmail(ctx context.Context, provider, email string) (*model.MailboxAccount, error) {
	account := &model.MailboxAccount{}
	err := r.db.WithContext(ctx).
		Where("provider = ? AND lower(email) = lower(?)", provider, email).
		First(account).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return account, nil
}

func (r *mailboxAccountRepository) List(ctx context.Context) ([]model.MailboxAccount, error) {
	accounts := make([]model.MailboxAccount, 0)
	err := r.db.WithContext(ctx).Order("id DESC").Find(&accounts).Error
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *mailboxAccountRepository) UpdateToken(ctx context.Context, id int64, accessToken string, refreshToken *string, tokenExpiry *time.Time) error {
	updates := map[string]interface{}{
		"access_token": accessToken,
		"token_expiry": tokenExpiry,
		"updated_at":   time.Now(),
	}
	// refresh_token 可能在刷新响应里为空，为空时保留旧值，避免把已有的刷新令牌覆盖掉。
	if refreshToken != nil {
		updates["refresh_token"] = *refreshToken
	}
	return r.db.WithContext(ctx).
		Model(&model.MailboxAccount{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *mailboxAccountRepository) UpdateTokenByID(ctx context.Context, id int64, accessToken string, refreshToken, tokenExpiry *string) error {
	updates := map[string]interface{}{
		"access_token": accessToken,
		"updated_at":   time.Now(),
	}
	if refreshToken != nil {
		updates["refresh_token"] = *refreshToken
	}
	if tokenExpiry != nil {
		expiry, err := time.Parse("2006-01-02 15:04:05", *tokenExpiry)
		if err == nil {
			updates["token_expiry"] = expiry
		}
	}
	return r.db.WithContext(ctx).
		Model(&model.MailboxAccount{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *mailboxAccountRepository) UpdateLastScanAt(ctx context.Context, id int64, scannedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.MailboxAccount{}).
		Where("id = ?", id).
		Update("last_scan_at", scannedAt).Error
}

func (r *mailboxAccountRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&model.MailboxAccount{}).Error
}

type mailboxMessageRepository struct {
	db *gorm.DB
}

func NewMailboxMessageRepository(db *gorm.DB) MailboxMessageRepository {
	return &mailboxMessageRepository{db: db}
}

// MarkProcessed 记录一封已处理邮件。依赖 uq_mailbox_messages_account_message 唯一索引，
// 用 ON CONFLICT DO NOTHING 保证并发安全：返回 true 表示本次是首次插入（应继续导入），
// false 表示该邮件已被（可能是并发的另一次扫描）处理过，调用方应跳过。
func (r *mailboxMessageRepository) MarkProcessed(ctx context.Context, accountID int64, messageID string) (bool, error) {
	message := &model.MailboxMessage{
		AccountID: accountID,
		MessageID: messageID,
	}
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(message)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// Exists 判断某封邮件是否已处理过。用于扫描前的快速跳过（MarkProcessed 仍是并发安全底线）。
func (r *mailboxMessageRepository) Exists(ctx context.Context, accountID int64, messageID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.MailboxMessage{}).
		Where("account_id = ? AND message_id = ?", accountID, messageID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
