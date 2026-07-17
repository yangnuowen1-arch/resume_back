package repository

import (
	"context"
	"errors"
	"strings"
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

// MailboxMessageMetadata 是一封邮件落库所需的稳定元数据。
// 空字符串会作为 NULL 持久化，避免把缺失的邮件头误当作业务值。
type MailboxMessageMetadata struct {
	AccountID int64
	MessageID string
	FromEmail string
	FromName  string
	Subject   string
}

// MailboxAttachmentMetadata 描述一封邮件中的一个附件。
// AttachmentKey 必须由邮件提供方给出或由调用方以稳定规则构造；它是导入幂等键的一部分。
type MailboxAttachmentMetadata struct {
	AttachmentKey   string
	AttachmentIndex int32
	Filename        string
	ContentType     string
	FileHash        string
	ObjectKey       string
}

// MailboxMessageRepository 记录邮件及其附件导入状态。
type MailboxMessageRepository interface {
	// PersistAttachment 原子地创建候选人、简历和邮件附件关联。
	// 返回 false, nil 表示同一邮件附件已经持久化，candidate/resume 不会重复落库。
	PersistAttachment(
		ctx context.Context,
		message MailboxMessageMetadata,
		attachment MailboxAttachmentMetadata,
		candidate *model.Candidate,
		resume *model.Resume,
	) (created bool, err error)
	Exists(ctx context.Context, accountID int64, messageID string) (bool, error)
	// MarkProcessed 将一封邮件标记为完整处理，返回 true 表示本次首次完成该邮件。
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

var errMailboxAttachmentAlreadyPersisted = errors.New("mailbox attachment already persisted")

const (
	mailboxMessageImportStatusProcessing = "processing"
	mailboxMessageImportStatusProcessed  = "processed"
	maxMailboxAttachmentKeyLength        = 128
	maxMailboxMessageIDLength            = 512
)

// PersistAttachment uses the attachment record as the idempotency boundary.
// It marks a new mailbox message as processing; the caller only marks it
// complete after every attachment has succeeded. Keeping processed_at intact
// preserves compatibility with an old worker during a rolling deployment.
func (r *mailboxMessageRepository) PersistAttachment(
	ctx context.Context,
	message MailboxMessageMetadata,
	attachment MailboxAttachmentMetadata,
	candidate *model.Candidate,
	resume *model.Resume,
) (created bool, err error) {
	if err := validatePersistAttachmentInput(message, attachment, candidate, resume); err != nil {
		return false, err
	}

	// GORM assigns generated primary keys to these input objects. Restore the
	// identifiers when a concurrent transaction wins the attachment key so a
	// caller never observes IDs for rows that were rolled back.
	originalCandidateID := candidate.ID
	originalResumeID := resume.ID
	originalResumeCandidateID := resume.CandidateID
	restoreModels := func() {
		candidate.ID = originalCandidateID
		resume.ID = originalResumeID
		resume.CandidateID = originalResumeCandidateID
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		storedMessage, err := upsertMailboxMessage(tx, message)
		if err != nil {
			return err
		}
		if storedMessage.ImportStatus == mailboxMessageImportStatusProcessed {
			// A legacy worker may have completed this message before this worker
			// obtained the transaction. Treat it as done rather than importing it
			// a second time under the new per-attachment semantics.
			return nil
		}

		// Avoid candidate/resume writes for the normal retry path. The insert
		// below still handles a race between this check and the write.
		var existingCount int64
		if err := tx.Model(&model.MailboxMessageAttachment{}).
			Where("message_id = ? AND attachment_key = ?", storedMessage.ID, attachment.AttachmentKey).
			Count(&existingCount).Error; err != nil {
			return err
		}
		if existingCount > 0 {
			return nil
		}

		if err := tx.Create(candidate).Error; err != nil {
			return err
		}
		resume.CandidateID = &candidate.ID
		if err := tx.Create(resume).Error; err != nil {
			return err
		}

		link := &model.MailboxMessageAttachment{
			MessageID:       storedMessage.ID,
			AttachmentKey:   attachment.AttachmentKey,
			AttachmentIndex: attachment.AttachmentIndex,
			Filename:        nullableString(attachment.Filename),
			ContentType:     nullableString(attachment.ContentType),
			FileHash:        nullableString(attachment.FileHash),
			ObjectKey:       nullableString(attachment.ObjectKey),
			CandidateID:     candidate.ID,
			ResumeID:        resume.ID,
		}
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "message_id"}, {Name: "attachment_key"}},
			DoNothing: true,
		}).Create(link)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			// Returning an error rolls back the candidate/resume just created
			// above. The public method translates this expected race to
			// (false, nil).
			return errMailboxAttachmentAlreadyPersisted
		}

		created = true
		return nil
	})
	if errors.Is(err, errMailboxAttachmentAlreadyPersisted) {
		restoreModels()
		return false, nil
	}
	if err != nil {
		restoreModels()
		return false, err
	}
	return created, nil
}

func validatePersistAttachmentInput(
	message MailboxMessageMetadata,
	attachment MailboxAttachmentMetadata,
	candidate *model.Candidate,
	resume *model.Resume,
) error {
	if message.AccountID <= 0 {
		return errors.New("mailbox account ID must be positive")
	}
	if strings.TrimSpace(message.MessageID) == "" {
		return errors.New("mailbox message ID is required")
	}
	if len([]rune(message.MessageID)) > maxMailboxMessageIDLength {
		return errors.New("mailbox message ID is too long")
	}
	if strings.TrimSpace(attachment.AttachmentKey) == "" {
		return errors.New("mailbox attachment key is required")
	}
	if len([]rune(attachment.AttachmentKey)) > maxMailboxAttachmentKeyLength {
		return errors.New("mailbox attachment key is too long")
	}
	if attachment.AttachmentIndex < 0 {
		return errors.New("mailbox attachment index must not be negative")
	}
	if candidate == nil {
		return errors.New("candidate is required")
	}
	if resume == nil {
		return errors.New("resume is required")
	}
	return nil
}

func upsertMailboxMessage(tx *gorm.DB, metadata MailboxMessageMetadata) (*model.MailboxMessage, error) {
	message := &model.MailboxMessage{
		AccountID:    metadata.AccountID,
		MessageID:    metadata.MessageID,
		FromEmail:    nullableString(metadata.FromEmail),
		FromName:     nullableString(metadata.FromName),
		Subject:      nullableString(metadata.Subject),
		ImportStatus: mailboxMessageImportStatusProcessing,
	}
	result := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "account_id"}, {Name: "message_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"from_email": gorm.Expr("COALESCE(EXCLUDED.from_email, mailbox_messages.from_email)"),
			"from_name":  gorm.Expr("COALESCE(EXCLUDED.from_name, mailbox_messages.from_name)"),
			"subject":    gorm.Expr("COALESCE(EXCLUDED.subject, mailbox_messages.subject)"),
		}),
	}).Create(message)
	if result.Error != nil {
		return nil, result.Error
	}

	storedMessage := &model.MailboxMessage{}
	if err := tx.Where("account_id = ? AND message_id = ?", metadata.AccountID, metadata.MessageID).
		First(storedMessage).Error; err != nil {
		return nil, err
	}
	return storedMessage, nil
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// MarkProcessed marks a message complete. PersistAttachment may already have
// created a processing row, so update that row first; if none exists, create
// a completed row for compatibility with messages that have no attachments.
func (r *mailboxMessageRepository) MarkProcessed(ctx context.Context, accountID int64, messageID string) (bool, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&model.MailboxMessage{}).
		Where("account_id = ? AND message_id = ? AND import_status <> ?", accountID, messageID, mailboxMessageImportStatusProcessed).
		Updates(map[string]interface{}{
			"import_status": mailboxMessageImportStatusProcessed,
			"processed_at":  now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected > 0 {
		return true, nil
	}

	message := &model.MailboxMessage{
		AccountID:    accountID,
		MessageID:    messageID,
		ImportStatus: mailboxMessageImportStatusProcessed,
		ProcessedAt:  now,
	}
	result = r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "account_id"}, {Name: "message_id"}},
			DoNothing: true,
		}).
		Create(message)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// Exists 判断某封邮件是否已完整处理过。processing 状态的记录必须允许后续扫描重试。
func (r *mailboxMessageRepository) Exists(ctx context.Context, accountID int64, messageID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.MailboxMessage{}).
		Where("account_id = ? AND message_id = ? AND import_status = ?", accountID, messageID, mailboxMessageImportStatusProcessed).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
