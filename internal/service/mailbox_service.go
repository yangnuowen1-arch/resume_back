package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/filemime"
	"github.com/yangnuowen1-arch/resume_back/internal/mailbox"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
	"github.com/yangnuowen1-arch/resume_back/internal/storage"
)

// ErrScanInProgress 表示同一账号已有扫描在进行，拒绝并发触发。
var ErrScanInProgress = errors.New("该邮箱账号正在扫描中")

// ScanResult 汇总一次扫描导入的统计数字。
type ScanResult struct {
	Scanned  int // 本次实际处理的未读邮件数（跳过已处理的不计）
	Imported int // 新建的简历数
	Skipped  int // 同一封邮件中此前已持久化、重试时跳过的附件数
}

// MailboxService 是邮箱扫描导入的核心，手动触发与定时任务共用。
type MailboxService interface {
	ScanAndImport(ctx context.Context, accountID int64) (ScanResult, error)
	EnqueueScan(ctx context.Context, accountID int64, triggerSource string) (int64, error)
	GetScanTaskStatus(ctx context.Context, taskID int64) (*ScanTaskStatus, error)
}

// ScanTaskStatus 返回扫描任务的状态与统计。
type ScanTaskStatus struct {
	ID            int64      `json:"id"`
	AccountID     int64      `json:"accountId"`
	TriggerSource string     `json:"triggerSource"`
	Status        string     `json:"status"`
	Scanned       int32      `json:"scanned"`
	Imported      int32      `json:"imported"`
	Skipped       int32      `json:"skipped"`
	Error         *string    `json:"error"`
	StartedAt     *time.Time `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
	CreatedAt     time.Time  `json:"createdAt"`
}

const (
	ScanTaskStatusPending = "pending"
	ScanTaskStatusRunning = "running"
	ScanTaskStatusDone    = "done"
	ScanTaskStatusFailed  = "failed"

	ScanTriggerManual    = "manual"
	ScanTriggerScheduled = "scheduled"

	defaultScanQueueSize   = 50
	defaultScanWorkerCount = 2
	maxScanWorkerCount     = 5

	maxMailboxShortTextLength = 255
)

// MailboxDependencies 汇聚扫描导入所需的仓储、存储与 Provider 工厂。
type MailboxDependencies struct {
	AccountRepo  repository.MailboxAccountRepository
	MessageRepo  repository.MailboxMessageRepository
	ScanTaskRepo repository.MailboxScanTaskRepository
	Uploader     storage.Uploader
	// Providers 按平台标识（"google"）索引已配置的 Provider。
	Providers   map[string]mailbox.Provider
	AllowedExt  string // 逗号分隔白名单，如 ".pdf,.docx"
	QueueSize   int    // 任务队列容量，默认 50
	WorkerCount int    // worker 数量，默认 2
}

type scanTaskJob struct {
	TaskID    int64
	AccountID int64
}

type mailboxService struct {
	accountRepo  repository.MailboxAccountRepository
	messageRepo  repository.MailboxMessageRepository
	scanTaskRepo repository.MailboxScanTaskRepository
	uploader     storage.Uploader
	providers    map[string]mailbox.Provider
	allowedExt   map[string]struct{}
	queue        chan scanTaskJob

	// running 记录正在扫描的账号 ID，保证同一账号同时只跑一个扫描。
	mu      sync.Mutex
	running map[int64]struct{}
}

func NewMailboxService(deps MailboxDependencies) MailboxService {
	service := &mailboxService{
		accountRepo:  deps.AccountRepo,
		messageRepo:  deps.MessageRepo,
		scanTaskRepo: deps.ScanTaskRepo,
		uploader:     deps.Uploader,
		providers:    deps.Providers,
		allowedExt:   mailbox.AllowedExtSet(deps.AllowedExt),
		running:      make(map[int64]struct{}),
	}

	// 启动任务队列与 worker 池（仅当 ScanTaskRepo 配置时）
	if deps.ScanTaskRepo != nil {
		queueSize := deps.QueueSize
		if queueSize <= 0 {
			queueSize = defaultScanQueueSize
		}
		service.queue = make(chan scanTaskJob, queueSize)
		service.startScanWorkers(normalizeScanWorkerCount(deps.WorkerCount))
	}

	return service
}

func normalizeScanWorkerCount(workerCount int) int {
	if workerCount <= 0 {
		return defaultScanWorkerCount
	}
	if workerCount > maxScanWorkerCount {
		return maxScanWorkerCount
	}
	return workerCount
}

func (s *mailboxService) startScanWorkers(workerCount int) {
	for i := 0; i < workerCount; i++ {
		go s.runScanWorker(context.Background())
	}
}

// acquire 尝试占用某账号的扫描位；已在扫描则返回 false。
func (s *mailboxService) acquire(accountID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.running[accountID]; ok {
		return false
	}
	s.running[accountID] = struct{}{}
	return true
}

func (s *mailboxService) release(accountID int64) {
	s.mu.Lock()
	delete(s.running, accountID)
	s.mu.Unlock()
}

// runScanWorker 是后台 worker，从队列中取扫描任务并执行 ScanAndImport。
func (s *mailboxService) runScanWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-s.queue:
			start := time.Now()
			log.Printf("mailbox scan task started taskId=%d accountId=%d", job.TaskID, job.AccountID)

			if err := s.processScanTask(context.Background(), job); err != nil {
				log.Printf("mailbox scan task failed taskId=%d accountId=%d duration=%s error=%v",
					job.TaskID, job.AccountID, time.Since(start), err)
				continue
			}

			log.Printf("mailbox scan task succeeded taskId=%d accountId=%d duration=%s",
				job.TaskID, job.AccountID, time.Since(start))
		}
	}
}

// EnqueueScan 创建扫描任务并入队，返回任务 ID。triggerSource 为 "manual" 或 "scheduled"。
func (s *mailboxService) EnqueueScan(ctx context.Context, accountID int64, triggerSource string) (int64, error) {
	if s.queue == nil || s.scanTaskRepo == nil {
		return 0, errors.New("邮箱扫描任务队列未启动")
	}
	if accountID <= 0 {
		return 0, errors.New("邮箱账号 ID 不合法")
	}

	task := &model.MailboxScanTask{
		AccountID:     accountID,
		TriggerSource: triggerSource,
		Status:        ScanTaskStatusPending,
	}
	if err := s.scanTaskRepo.Create(ctx, task); err != nil {
		return 0, fmt.Errorf("创建扫描任务失败: %w", err)
	}

	select {
	case s.queue <- scanTaskJob{TaskID: task.ID, AccountID: accountID}:
		log.Printf("mailbox scan task enqueued taskId=%d accountId=%d triggerSource=%s", task.ID, accountID, triggerSource)
		return task.ID, nil
	case <-ctx.Done():
		_ = s.scanTaskRepo.MarkFailed(ctx, task.ID, 0, 0, 0, "任务入队超时")
		return 0, ctx.Err()
	default:
		_ = s.scanTaskRepo.MarkFailed(ctx, task.ID, 0, 0, 0, "扫描任务队列已满")
		return 0, errors.New("扫描任务队列已满")
	}
}

// GetScanTaskStatus 查询扫描任务的状态与统计。
func (s *mailboxService) GetScanTaskStatus(ctx context.Context, taskID int64) (*ScanTaskStatus, error) {
	if s.scanTaskRepo == nil {
		return nil, errors.New("邮箱扫描任务未启动")
	}
	if taskID <= 0 {
		return nil, errors.New("任务 ID 不合法")
	}

	task, err := s.scanTaskRepo.FindByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("查询扫描任务失败: %w", err)
	}

	return &ScanTaskStatus{
		ID:            task.ID,
		AccountID:     task.AccountID,
		TriggerSource: task.TriggerSource,
		Status:        task.Status,
		Scanned:       task.Scanned,
		Imported:      task.Imported,
		Skipped:       task.Skipped,
		Error:         task.Error,
		StartedAt:     task.StartedAt,
		FinishedAt:    task.FinishedAt,
		CreatedAt:     task.CreatedAt,
	}, nil
}

// processScanTask 执行单个扫描任务：标记 running → 执行 ScanAndImport → 回写状态与统计。
func (s *mailboxService) processScanTask(ctx context.Context, job scanTaskJob) error {
	if err := s.scanTaskRepo.MarkRunning(ctx, job.TaskID); err != nil {
		return err
	}
	log.Printf("mailbox scan task marked running taskId=%d accountId=%d", job.TaskID, job.AccountID)

	result, err := s.ScanAndImport(ctx, job.AccountID)
	if err != nil {
		errMsg := err.Error()
		if merr := s.scanTaskRepo.MarkFailed(ctx, job.TaskID, result.Scanned, result.Imported, result.Skipped, errMsg); merr != nil {
			log.Printf("mailbox scan task mark failed error taskId=%d error=%v originalError=%s", job.TaskID, merr, errMsg)
		}
		return err
	}

	if err := s.scanTaskRepo.MarkDone(ctx, job.TaskID, result.Scanned, result.Imported, result.Skipped); err != nil {
		log.Printf("mailbox scan task mark done error taskId=%d error=%v", job.TaskID, err)
		return err
	}
	log.Printf("mailbox scan task result saved taskId=%d scanned=%d imported=%d skipped=%d",
		job.TaskID, result.Scanned, result.Imported, result.Skipped)

	return nil
}

// ScanAndImport 扫描单个邮箱账号的未读邮件，提取简历附件并导入候选人库。
// 手动触发与定时任务共用此核心。同一账号同时只允许一个扫描在跑。
func (s *mailboxService) ScanAndImport(ctx context.Context, accountID int64) (ScanResult, error) {
	var result ScanResult

	if !s.acquire(accountID) {
		return result, ErrScanInProgress
	}
	defer s.release(accountID)

	account, err := s.accountRepo.FindByID(ctx, accountID)
	if err != nil {
		return result, fmt.Errorf("加载邮箱账号失败: %w", err)
	}

	provider, ok := s.providers[account.Provider]
	if !ok {
		return result, fmt.Errorf("邮箱平台 %q 未配置", account.Provider)
	}

	token, err := s.validToken(ctx, account, provider)
	if err != nil {
		return result, fmt.Errorf("获取有效 token 失败: %w", err)
	}

	messages, err := provider.ListUnread(ctx, token)
	if err != nil {
		return result, fmt.Errorf("拉取未读邮件失败: %w", err)
	}

	for _, msg := range messages {
		scanned, imported, skipped, perr := s.processMessage(ctx, provider, token, account.ID, msg)
		if perr != nil {
			// 单封邮件失败不阻断整体扫描：记日志后继续下一封。
			log.Printf("mailbox scan message failed accountId=%d messageId=%s error=%v", account.ID, msg.ID, perr)
			continue
		}
		// 已处理过的邮件（此前扫描已入库）不计入 Scanned。
		if !scanned {
			continue
		}
		result.Scanned++
		result.Imported += imported
		result.Skipped += skipped
	}

	if err := s.accountRepo.UpdateLastScanAt(ctx, account.ID, time.Now()); err != nil {
		log.Printf("mailbox update last_scan_at failed accountId=%d error=%v", account.ID, err)
	}
	return result, nil
}

// validToken 从账号记录重建 token，必要时刷新，并把刷新后的新 token 回写。
func (s *mailboxService) validToken(ctx context.Context, account *model.MailboxAccount, provider mailbox.Provider) (*oauth2.Token, error) {
	token := &oauth2.Token{AccessToken: account.AccessToken}
	if account.RefreshToken != nil {
		token.RefreshToken = *account.RefreshToken
	}
	if account.TokenExpiry != nil {
		token.Expiry = *account.TokenExpiry
	}

	// RefreshToken 内部使用 oauth2.TokenSource：token 未过期时原样返回，过期才真正刷新。
	refreshed, err := provider.RefreshToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// AccessToken 变化说明发生了刷新，回写新 token；写库失败不阻断本次扫描。
	if refreshed.AccessToken != token.AccessToken {
		var refreshPtr *string
		if refreshed.RefreshToken != "" {
			refreshPtr = &refreshed.RefreshToken
		}
		var expiryPtr *time.Time
		if !refreshed.Expiry.IsZero() {
			expiryPtr = &refreshed.Expiry
		}
		if err := s.accountRepo.UpdateToken(ctx, account.ID, refreshed.AccessToken, refreshPtr, expiryPtr); err != nil {
			log.Printf("mailbox update token failed accountId=%d error=%v", account.ID, err)
		}
	}
	return refreshed, nil
}

// processMessage 处理单封邮件：跳过已处理 → 拉附件 → 过滤 → 导入 → 登记已处理 → 标已读。
// 返回 scanned=false 表示该邮件此前已成功处理（跳过、不计入统计）；
// imported 为本封新建的候选人/简历对数，skipped 为此前已持久化的同邮件附件数。
//
// 关键顺序：先导入全部附件、都成功后才 MarkProcessed。任一附件失败时邮件不会标记完成，
// 下次扫描会重试；已入库的附件靠 (邮件, 附件标识) 命中跳过，不会重复建候选人。
// 同账号并发由上层 running 锁串行化；跨进程并发由附件唯一索引兜底。
func (s *mailboxService) processMessage(ctx context.Context, provider mailbox.Provider, token *oauth2.Token, accountID int64, msg mailbox.Message) (scanned bool, imported int, skipped int, err error) {
	persistedMessageID := mailboxMessagePersistenceID(msg.ID)

	// 快速跳过：此前已成功处理的邮件不再拉附件（拉附件字节较贵）。
	already, err := s.messageRepo.Exists(ctx, accountID, persistedMessageID)
	if err != nil {
		return false, 0, 0, fmt.Errorf("查询邮件处理状态失败: %w", err)
	}
	if already {
		// 上一次已落库但标已读失败时，邮件仍会出现在未读列表；此处补偿性重试，
		// 不必重新下载附件。
		if merr := provider.MarkRead(ctx, token, msg.ID); merr != nil {
			log.Printf("mailbox retry mark read failed accountId=%d messageId=%s error=%v", accountID, msg.ID, merr)
		}
		return false, 0, 0, nil
	}

	if msg.HasAttachments {
		attachments, ferr := provider.FetchAttachments(ctx, token, msg.ID)
		if ferr != nil {
			return false, 0, 0, fmt.Errorf("拉取附件失败: %w", ferr)
		}
		imported, skipped, err = s.importAttachments(ctx, accountID, persistedMessageID, msg, mailbox.FilterAttachments(attachments, s.allowedExt))
		if err != nil {
			// 不登记为已处理：下次扫描重试，已入库附件会靠附件幂等键跳过。
			return false, 0, 0, err
		}
	}

	// 附件全部处理成功后再登记，作为后续扫描的幂等去重记录。
	if _, err := s.messageRepo.MarkProcessed(ctx, accountID, persistedMessageID); err != nil {
		return false, 0, 0, fmt.Errorf("登记已处理邮件失败: %w", err)
	}

	// 标已读失败不阻断：简历已入库，且已登记 mailbox_messages 防重复。
	if merr := provider.MarkRead(ctx, token, msg.ID); merr != nil {
		log.Printf("mailbox mark read failed accountId=%d messageId=%s error=%v", accountID, msg.ID, merr)
	}
	return true, imported, skipped, nil
}

// importAttachments 对每个简历附件构造独立的 candidate 壳：
// 文件名 -> candidate.name，原文件 -> R2，随后在一个 PostgreSQL 事务中写入
// candidate + resume 元数据 + mailbox_message_attachment。附件级幂等确保失败重试
// 不会重复建候选人；不同邮件即使文件内容相同也各自保留一份投递记录。
func (s *mailboxService) importAttachments(ctx context.Context, accountID int64, persistedMessageID string, msg mailbox.Message, attachments []mailbox.Attachment) (imported int, skipped int, err error) {
	for index, att := range attachments {
		fileHash := attachmentHash(att.Data)
		attachmentKey := attachmentIdentity(att, index, fileHash)
		objectKey := attachmentObjectKey(accountID, persistedMessageID, attachmentKey, att)
		contentType := filemime.Normalize(att.ContentType, att.Filename)

		// 先构造候选人壳，再上传原始文件；候选人和简历会在上传成功后一起落库。
		candidate := buildCandidate(deriveCandidateName(att.Filename), msg.FromEmail)
		uploaded, uerr := s.uploader.UploadBytes(ctx, objectKey, att.Data, contentType)
		if uerr != nil {
			return imported, skipped, fmt.Errorf("上传附件失败: %w", uerr)
		}

		resume := s.buildResume(att, fileHash, uploaded)
		created, perr := s.messageRepo.PersistAttachment(
			ctx,
			repository.MailboxMessageMetadata{
				AccountID: accountID,
				MessageID: persistedMessageID,
				FromEmail: normalizeMailboxShortText(msg.FromEmail),
				FromName:  normalizeMailboxShortText(msg.FromName),
				Subject:   msg.Subject,
			},
			repository.MailboxAttachmentMetadata{
				AttachmentKey:   attachmentKey,
				AttachmentIndex: int32(index),
				Filename:        mailboxFilename(att.Filename),
				ContentType:     normalizeMailboxShortText(contentType),
				FileHash:        fileHash,
				ObjectKey:       uploaded.Key,
			},
			candidate,
			resume,
		)
		if perr != nil {
			// objectKey 是由邮件 + 附件标识确定的。保留对象可使下一次扫描安全地
			// 覆盖/复用它；这样也避免跨进程重试时误删另一个成功事务的原文件。
			return imported, skipped, fmt.Errorf("持久化候选人、简历及邮件记录失败: %w", perr)
		}
		if !created {
			skipped++
			continue
		}
		imported++
	}
	return imported, skipped, nil
}

// buildResume 由附件与上传结果组装一条待入库的简历（先只存文件不解析）。
func (s *mailboxService) buildResume(att mailbox.Attachment, fileHash string, uploaded *storage.UploadResult) *model.Resume {
	filename := mailboxFilename(att.Filename)
	fileType := filemime.Normalize(att.ContentType, filename)
	size := int64(len(att.Data))
	return &model.Resume{
		OriginalFilename: &filename,
		FileKey:          &uploaded.Key,
		FileURL:          &uploaded.URL,
		FileType:         &fileType,
		FileSize:         &size,
		FileHash:         &fileHash,
		ParseStatus:      ResumeParseStatusPending,
	}
}

// attachmentIdentity 返回邮件内附件的稳定幂等标识。优先用 Provider 标识；
// 没有时以附件序号 + 内容 hash 兜底，保证同一封邮件重试时仍可识别。
func attachmentIdentity(att mailbox.Attachment, index int, fileHash string) string {
	if id := strings.TrimSpace(att.ID); id != "" {
		// Provider attachment IDs are not needed outside the provider API. Keep
		// a fixed-size digest in PostgreSQL so its unique B-tree index remains
		// safe even if a future provider emits a very long identifier.
		return "provider-sha256:" + attachmentHash([]byte(id))
	}
	return fmt.Sprintf("index:%d:sha256:%s", index, fileHash)
}

// mailboxMessagePersistenceID bounds an untrusted provider message ID before
// it reaches the indexed varchar column. Gmail IDs are far shorter and remain
// unchanged; a future oversized ID gets a stable digest instead.
func mailboxMessagePersistenceID(messageID string) string {
	messageID = strings.ToValidUTF8(strings.TrimSpace(messageID), "")
	if len([]rune(messageID)) <= 512 {
		return messageID
	}
	return "sha256:" + attachmentHash([]byte(messageID))
}

// attachmentObjectKey 生成确定性的 R2 key。它不包含不可信的文件名，
// 重试同一邮件附件会复用同一对象，不同邮件附件不会相互覆盖。
func attachmentObjectKey(accountID int64, messageID, attachmentKey string, att mailbox.Attachment) string {
	scope := fmt.Sprintf("%d:%s:%s", accountID, messageID, attachmentKey)
	sum := sha256.Sum256([]byte(scope))
	return fmt.Sprintf("resumes/mailbox/%s%s", hex.EncodeToString(sum[:]), att.Ext())
}

// attachmentHash 计算附件字节的 SHA-256，返回 64 位十六进制串（对齐 resumes.file_hash）。
func attachmentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// buildCandidate 组装一条邮箱来源的新候选人（source=email，状态待评估）。
func buildCandidate(name, fromEmail string) *model.Candidate {
	source := CandidateSourceEmail
	candidate := &model.Candidate{
		Name:   &name,
		Status: CandidateStatusPendingReview,
		Source: &source,
	}
	// 邮件附件文件名只用于候选人壳的 name，绝不推断当前职位、岗位 ID 或岗位分类。
	// 这些字段保持 NULL，后续仅由人工编辑或可信的解析结果填写。
	if email := normalizeMailboxShortText(fromEmail); email != "" {
		candidate.Email = &email
	}
	return candidate
}

// deriveCandidateName 直接以简历附件名（去扩展名）作为候选人壳名称。
// 真实姓名的补全/校正留给后续解析或人工编辑，避免邮件导入阶段依赖解析结果。
func deriveCandidateName(filename string) string {
	baseFilename := mailboxFilename(filename)
	base := strings.TrimSpace(strings.TrimSuffix(baseFilename, filepath.Ext(baseFilename)))
	if base != "" && base != "." {
		return base
	}
	return "未命名候选人"
}

// mailboxFilename strips any path supplied by the mail provider and bounds
// the result to the varchar(255) columns used by resume and attachment metadata.
func mailboxFilename(filename string) string {
	base := filepath.Base(strings.ToValidUTF8(strings.TrimSpace(filename), ""))
	if base == "." {
		return ""
	}
	if len([]rune(base)) <= maxMailboxShortTextLength {
		return base
	}

	// Preserve the extension when shortening a long filename, so a stored
	// original filename still tells operators which file type was imported.
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	maxStemLength := maxMailboxShortTextLength - len([]rune(ext))
	if maxStemLength <= 0 {
		return normalizeMailboxShortText(base)
	}
	return truncateMailboxText(stem, maxStemLength) + ext
}

// normalizeMailboxShortText makes untrusted mail header values safe for the
// varchar(255) persistence columns. PostgreSQL measures VARCHAR in characters,
// so truncating by runes avoids splitting a UTF-8 character.
func normalizeMailboxShortText(value string) string {
	return truncateMailboxText(strings.ToValidUTF8(strings.TrimSpace(value), ""), maxMailboxShortTextLength)
}

func truncateMailboxText(value string, maxLength int) string {
	runes := []rune(value)
	if len(runes) <= maxLength {
		return value
	}
	return string(runes[:maxLength])
}
