package service

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/mailbox"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
	"github.com/yangnuowen1-arch/resume_back/internal/storage"
)

func TestDeriveCandidateName(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		want     string
	}{
		{"文件名去扩展名", "张三.pdf", "张三"},
		{"保留通用文件名", "resume.pdf", "resume"},
		{"保留中文通用文件名", "简历.pdf", "简历"},
		{"空文件名有稳定兜底", "", "未命名候选人"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := deriveCandidateName(c.filename); got != c.want {
				t.Fatalf("deriveCandidateName(%q) = %q, want %q", c.filename, got, c.want)
			}
		})
	}
}

func TestMailboxMetadataIsBoundedBeforePersistence(t *testing.T) {
	filename := strings.Repeat("名", 300) + ".pdf"
	storedFilename := mailboxFilename(filename)
	if got := len([]rune(storedFilename)); got > maxMailboxShortTextLength {
		t.Fatalf("stored filename length=%d, max=%d", got, maxMailboxShortTextLength)
	}
	if !strings.HasSuffix(storedFilename, ".pdf") {
		t.Fatalf("stored filename must keep the file extension, got %q", storedFilename)
	}
	if got := len([]rune(deriveCandidateName(filename))); got > maxMailboxShortTextLength {
		t.Fatalf("candidate shell name length=%d, max=%d", got, maxMailboxShortTextLength)
	}

	candidate := buildCandidate("候选人", strings.Repeat("x", maxMailboxShortTextLength+1))
	if candidate.Email == nil || len([]rune(*candidate.Email)) != maxMailboxShortTextLength {
		t.Fatalf("candidate email should be bounded to %d chars, got %#v", maxMailboxShortTextLength, candidate.Email)
	}
}

// 附件白名单过滤：只导入 .pdf/.docx，签名图等被忽略。
func TestProcessMessageFiltersAttachments(t *testing.T) {
	svc, deps := newTestMailboxService()
	msg := mailbox.Message{ID: "m1", FromEmail: "a@x.com", FromName: "甲", Subject: "应聘", HasAttachments: true}
	deps.provider.attachments = []mailbox.Attachment{
		{ID: "a1", Filename: "resume-甲.pdf", ContentType: "pdf", Data: []byte("PDF-A")},
		{ID: "a2", Filename: "signature.png", Data: []byte("IMG")},
		{ID: "a3", Filename: "notes.txt", Data: []byte("TXT")},
	}

	scanned, imported, skipped, err := svc.processMessage(context.Background(), deps.provider, &oauth2.Token{}, 1, msg)
	if err != nil {
		t.Fatalf("processMessage: %v", err)
	}
	if !scanned || imported != 1 || skipped != 0 {
		t.Fatalf("scanned=%v imported=%d skipped=%d, want true/1/0", scanned, imported, skipped)
	}
	if len(deps.message.persisted) != 1 {
		t.Fatalf("expected 1 persisted attachment, got %d", len(deps.message.persisted))
	}
	if len(deps.uploader.uploaded) != 1 {
		t.Fatalf("expected 1 upload, got %d", len(deps.uploader.uploaded))
	}

	persisted := deps.message.persisted[0]
	if persisted.candidate.Name == nil || *persisted.candidate.Name != "resume-甲" {
		t.Fatalf("candidate name = %#v, want filename-derived resume-甲", persisted.candidate.Name)
	}
	if persisted.candidate.Email == nil || *persisted.candidate.Email != "a@x.com" {
		t.Fatalf("candidate email = %#v, want sender email", persisted.candidate.Email)
	}
	if persisted.candidate.CurrentJobID != nil || persisted.candidate.CurrentPosition != nil || persisted.candidate.PositionCategoryID != nil || persisted.candidate.CurrentPositionCategory != nil {
		t.Fatalf("mail attachment must not infer a current position: %#v", persisted.candidate)
	}
	if persisted.message.Subject != "应聘" || persisted.attachment.Filename != "resume-甲.pdf" {
		t.Fatalf("mail metadata not persisted: %#v", persisted)
	}
	if persisted.attachment.ContentType != "application/pdf" {
		t.Fatalf("mail attachment content type = %q, want application/pdf", persisted.attachment.ContentType)
	}
	if persisted.resume.FileType == nil || *persisted.resume.FileType != "application/pdf" {
		t.Fatalf("resume file type = %#v, want application/pdf", persisted.resume.FileType)
	}
	if len(deps.uploader.contentTypes) != 1 || deps.uploader.contentTypes[0] != "application/pdf" {
		t.Fatalf("uploaded content types = %#v, want [application/pdf]", deps.uploader.contentTypes)
	}
	if persisted.resume.FileURL == nil || *persisted.resume.FileURL == "" {
		t.Fatal("resume file URL must be persisted for the candidate list")
	}
}

// 相同内容来自不同邮件时仍视为两次独立投递，每个附件都创建候选人壳。
func TestProcessMessageImportsSameHashFromDifferentMessages(t *testing.T) {
	svc, deps := newTestMailboxService()
	deps.provider.attachments = []mailbox.Attachment{{ID: "gmail-a1", Filename: "resume.pdf", Data: []byte("SAME")}}

	for _, messageID := range []string{"m1", "m2"} {
		scanned, imported, skipped, err := svc.processMessage(context.Background(), deps.provider, &oauth2.Token{}, 1, mailbox.Message{
			ID: messageID, FromEmail: "a@x.com", HasAttachments: true,
		})
		if err != nil {
			t.Fatalf("process %s: %v", messageID, err)
		}
		if !scanned || imported != 1 || skipped != 0 {
			t.Fatalf("message %s: scanned=%v imported=%d skipped=%d, want true/1/0", messageID, scanned, imported, skipped)
		}
	}

	if len(deps.message.persisted) != 2 {
		t.Fatalf("expected 2 independent persisted attachments, got %d", len(deps.message.persisted))
	}
	if deps.message.persisted[0].attachment.FileHash != deps.message.persisted[1].attachment.FileHash {
		t.Fatal("fixture should produce the same file hash")
	}
	if deps.message.persisted[0].attachment.ObjectKey == deps.message.persisted[1].attachment.ObjectKey {
		t.Fatal("different mailbox messages must not share an R2 object key")
	}
}

// 已完整处理的邮件直接跳过，不再下载附件。
func TestProcessMessageSkipsAlreadyProcessed(t *testing.T) {
	svc, deps := newTestMailboxService()
	deps.message.processed["m1"] = struct{}{}
	deps.provider.attachments = []mailbox.Attachment{{ID: "a1", Filename: "resume.pdf", Data: []byte("X")}}

	scanned, imported, skipped, err := svc.processMessage(context.Background(), deps.provider, &oauth2.Token{}, 1, mailbox.Message{
		ID: "m1", FromEmail: "a@x.com", HasAttachments: true,
	})
	if err != nil {
		t.Fatalf("processMessage: %v", err)
	}
	if scanned || imported != 0 || skipped != 0 {
		t.Fatalf("scanned=%v imported=%d skipped=%d, want false/0/0", scanned, imported, skipped)
	}
	if deps.provider.fetchCalls != 0 {
		t.Fatalf("already processed mail must not fetch attachments, got %d calls", deps.provider.fetchCalls)
	}
	if len(deps.provider.markedRead) != 1 || deps.provider.markedRead[0] != "m1" {
		t.Fatalf("already processed unread mail should retry MarkRead, got %v", deps.provider.markedRead)
	}
}

// 一封邮件多个简历附件：每个附件独立建 candidate + resume，而不是按发件人合并。
func TestProcessMessageCreatesCandidatePerAttachment(t *testing.T) {
	svc, deps := newTestMailboxService()
	msg := mailbox.Message{ID: "m1", FromEmail: "a@x.com", FromName: "甲", HasAttachments: true}
	deps.provider.attachments = []mailbox.Attachment{
		{ID: "a1", Filename: "简历-甲.pdf", Data: []byte("A1")},
		{ID: "a2", Filename: "作品集.pdf", Data: []byte("A2")},
		{ID: "a3", Filename: "证书.docx", Data: []byte("A3")},
	}

	scanned, imported, skipped, err := svc.processMessage(context.Background(), deps.provider, &oauth2.Token{}, 1, msg)
	if err != nil {
		t.Fatalf("processMessage: %v", err)
	}
	if !scanned || imported != 3 || skipped != 0 {
		t.Fatalf("scanned=%v imported=%d skipped=%d, want true/3/0", scanned, imported, skipped)
	}
	if len(deps.message.persisted) != 3 {
		t.Fatalf("expected exactly 3 candidate/resume records, got %d", len(deps.message.persisted))
	}
	if len(deps.uploader.uploaded) != 3 {
		t.Fatalf("expected 3 uploads, got %d", len(deps.uploader.uploaded))
	}
	for index, item := range deps.message.persisted {
		if item.candidate.ID != int64(index+1) || item.resume.CandidateID == nil || *item.resume.CandidateID != item.candidate.ID {
			t.Fatalf("attachment %d was not persisted as an independent candidate/resume pair: %#v", index, item)
		}
	}
}

// 某附件已在上次失败扫描中成功入库时，重试会上传到同一个 key，但不会重复创建候选人。
func TestProcessMessageSkipsPersistedAttachmentOnRetry(t *testing.T) {
	svc, deps := newTestMailboxService()
	msg := mailbox.Message{ID: "m1", FromEmail: "a@x.com", HasAttachments: true}
	att := mailbox.Attachment{ID: "gmail-a1", Filename: "resume.pdf", Data: []byte("X")}
	deps.provider.attachments = []mailbox.Attachment{att}

	fileHash := attachmentHash(att.Data)
	key := attachmentIdentity(att, 0, fileHash)
	if _, err := deps.message.PersistAttachment(context.Background(),
		repository.MailboxMessageMetadata{AccountID: 1, MessageID: msg.ID, FromEmail: msg.FromEmail},
		repository.MailboxAttachmentMetadata{AttachmentKey: key, AttachmentIndex: 0, Filename: att.Filename, FileHash: fileHash, ObjectKey: attachmentObjectKey(1, msg.ID, key, att)},
		buildCandidate(deriveCandidateName(att.Filename), msg.FromEmail),
		&model.Resume{ParseStatus: ResumeParseStatusPending},
	); err != nil {
		t.Fatalf("seed persisted attachment: %v", err)
	}

	scanned, imported, skipped, err := svc.processMessage(context.Background(), deps.provider, &oauth2.Token{}, 1, msg)
	if err != nil {
		t.Fatalf("processMessage: %v", err)
	}
	if !scanned || imported != 0 || skipped != 1 {
		t.Fatalf("scanned=%v imported=%d skipped=%d, want true/0/1", scanned, imported, skipped)
	}
	if len(deps.message.persisted) != 1 {
		t.Fatalf("expected no duplicate candidate/resume record, got %d", len(deps.message.persisted))
	}
	if len(deps.uploader.uploaded) != 1 {
		t.Fatalf("retry should re-upload the deterministic object once, got %d", len(deps.uploader.uploaded))
	}
}

// 附件中途失败时邮件不标记完成；下一次扫描跳过首份已持久化附件并继续补齐。
func TestProcessMessageRetriesPartialMailWithoutDuplicateCandidates(t *testing.T) {
	svc, deps := newTestMailboxService()
	deps.uploader.failAfter = 2 // 第 2 次上传起失败
	msg := mailbox.Message{ID: "m1", FromEmail: "a@x.com", HasAttachments: true}
	deps.provider.attachments = []mailbox.Attachment{
		{ID: "a1", Filename: "a.pdf", Data: []byte("A1")},
		{ID: "a2", Filename: "b.pdf", Data: []byte("A2")},
	}

	scanned, _, _, err := svc.processMessage(context.Background(), deps.provider, &oauth2.Token{}, 1, msg)
	if err == nil {
		t.Fatal("expected error when the second attachment upload fails")
	}
	if scanned {
		t.Fatal("scanned should be false on failure")
	}
	if _, ok := deps.message.processed[msg.ID]; ok {
		t.Fatal("message must not be marked processed on partial failure")
	}
	if len(deps.message.persisted) != 1 {
		t.Fatalf("first attachment should have been persisted, got %d", len(deps.message.persisted))
	}

	deps.uploader.failAfter = 0
	scanned, imported, skipped, err := svc.processMessage(context.Background(), deps.provider, &oauth2.Token{}, 1, msg)
	if err != nil {
		t.Fatalf("retry processMessage: %v", err)
	}
	if !scanned || imported != 1 || skipped != 1 {
		t.Fatalf("retry scanned=%v imported=%d skipped=%d, want true/1/1", scanned, imported, skipped)
	}
	if len(deps.message.persisted) != 2 {
		t.Fatalf("expected 2 total candidate/resume records after retry, got %d", len(deps.message.persisted))
	}
	if _, ok := deps.message.processed[msg.ID]; !ok {
		t.Fatal("message should be marked processed after all attachments succeed")
	}
}

func TestAttachmentObjectKeyIsStableAndScopedToMessageAttachment(t *testing.T) {
	att := mailbox.Attachment{ID: "gmail-a1", Filename: "resume.PDF"}
	attachmentKey := attachmentIdentity(att, 0, "hash")
	first := attachmentObjectKey(9, "message-1", attachmentKey, att)
	if again := attachmentObjectKey(9, "message-1", attachmentKey, att); again != first {
		t.Fatalf("object key must be deterministic: got %q, want %q", again, first)
	}
	if other := attachmentObjectKey(9, "message-2", attachmentKey, att); other == first {
		t.Fatal("different messages must use different object keys")
	}
	if first[len(first)-4:] != ".pdf" {
		t.Fatalf("object key should keep normalized extension, got %q", first)
	}
}

func TestMailboxPersistenceIdentifiersAreBoundedAndStable(t *testing.T) {
	longProviderID := strings.Repeat("附件", 1000)
	att := mailbox.Attachment{ID: longProviderID, Filename: "resume.pdf"}
	key := attachmentIdentity(att, 0, "file-hash")
	if len([]rune(key)) > 128 {
		t.Fatalf("attachment key length=%d, want <= 128", len([]rune(key)))
	}
	if strings.Contains(key, longProviderID) {
		t.Fatal("attachment key must not persist an unbounded provider identifier")
	}
	if again := attachmentIdentity(att, 0, "file-hash"); again != key {
		t.Fatalf("attachment key must be stable: got %q, want %q", again, key)
	}

	longMessageID := strings.Repeat("邮件", 1000)
	persisted := mailboxMessagePersistenceID(longMessageID)
	if len([]rune(persisted)) > 512 {
		t.Fatalf("message ID length=%d, want <= 512", len([]rune(persisted)))
	}
	if again := mailboxMessagePersistenceID(longMessageID); again != persisted {
		t.Fatalf("message ID must be stable: got %q, want %q", again, persisted)
	}
}

// --- 测试脚手架 ---

type mailboxTestDeps struct {
	provider *fakeProvider
	message  *fakeMessageRepo
	account  *fakeAccountRepo
	uploader *fakeBytesUploader
}

func newTestMailboxService() (*mailboxService, *mailboxTestDeps) {
	deps := &mailboxTestDeps{
		provider: &fakeProvider{},
		message:  &fakeMessageRepo{processed: map[string]struct{}{}, attachments: map[string]struct{}{}},
		account:  &fakeAccountRepo{},
		uploader: &fakeBytesUploader{},
	}
	svc := &mailboxService{
		accountRepo: deps.account,
		messageRepo: deps.message,
		uploader:    deps.uploader,
		providers:   map[string]mailbox.Provider{"google": deps.provider},
		allowedExt:  mailbox.AllowedExtSet(".pdf,.docx"),
		running:     map[int64]struct{}{},
	}
	return svc, deps
}

type fakeProvider struct {
	attachments []mailbox.Attachment
	fetchCalls  int
	markedRead  []string
}

func (p *fakeProvider) Provider() string            { return "google" }
func (p *fakeProvider) AuthURL(state string) string { return "" }
func (p *fakeProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return &oauth2.Token{}, nil
}
func (p *fakeProvider) RefreshToken(ctx context.Context, token *oauth2.Token) (*oauth2.Token, error) {
	return token, nil
}
func (p *fakeProvider) GetUserEmail(ctx context.Context, token *oauth2.Token) (string, error) {
	return "test@example.com", nil
}
func (p *fakeProvider) ListUnread(ctx context.Context, token *oauth2.Token) ([]mailbox.Message, error) {
	return nil, nil
}
func (p *fakeProvider) FetchAttachments(ctx context.Context, token *oauth2.Token, messageID string) ([]mailbox.Attachment, error) {
	p.fetchCalls++
	return p.attachments, nil
}
func (p *fakeProvider) MarkRead(ctx context.Context, token *oauth2.Token, messageID string) error {
	p.markedRead = append(p.markedRead, messageID)
	return nil
}

type persistedMailboxAttachment struct {
	message    repository.MailboxMessageMetadata
	attachment repository.MailboxAttachmentMetadata
	candidate  *model.Candidate
	resume     *model.Resume
}

type fakeMessageRepo struct {
	processed   map[string]struct{}
	attachments map[string]struct{}
	persisted   []persistedMailboxAttachment
}

func (r *fakeMessageRepo) PersistAttachment(
	ctx context.Context,
	message repository.MailboxMessageMetadata,
	attachment repository.MailboxAttachmentMetadata,
	candidate *model.Candidate,
	resume *model.Resume,
) (bool, error) {
	key := fakeMailboxAttachmentKey(message, attachment)
	if _, ok := r.attachments[key]; ok {
		return false, nil
	}

	candidate.ID = int64(len(r.persisted) + 1)
	resume.ID = candidate.ID
	resume.CandidateID = &candidate.ID
	r.attachments[key] = struct{}{}
	r.persisted = append(r.persisted, persistedMailboxAttachment{
		message:    message,
		attachment: attachment,
		candidate:  candidate,
		resume:     resume,
	})
	return true, nil
}

func (r *fakeMessageRepo) Exists(ctx context.Context, accountID int64, messageID string) (bool, error) {
	_, ok := r.processed[messageID]
	return ok, nil
}

func (r *fakeMessageRepo) MarkProcessed(ctx context.Context, accountID int64, messageID string) (bool, error) {
	if _, ok := r.processed[messageID]; ok {
		return false, nil
	}
	r.processed[messageID] = struct{}{}
	return true, nil
}

func fakeMailboxAttachmentKey(message repository.MailboxMessageMetadata, attachment repository.MailboxAttachmentMetadata) string {
	return fmt.Sprintf("%d:%s:%s", message.AccountID, message.MessageID, attachment.AttachmentKey)
}

type fakeAccountRepo struct {
	account *model.MailboxAccount
}

func (r *fakeAccountRepo) FindByID(ctx context.Context, id int64) (*model.MailboxAccount, error) {
	if r.account != nil {
		return r.account, nil
	}
	return &model.MailboxAccount{ID: id, Provider: "google"}, nil
}
func (r *fakeAccountRepo) Create(ctx context.Context, provider, email, accessToken string, refreshToken, tokenExpiry *string) error {
	return nil
}
func (r *fakeAccountRepo) FindByProviderEmail(ctx context.Context, provider, email string) (*model.MailboxAccount, error) {
	return nil, nil
}
func (r *fakeAccountRepo) List(ctx context.Context) ([]model.MailboxAccount, error) {
	return nil, nil
}
func (r *fakeAccountRepo) UpdateToken(ctx context.Context, id int64, accessToken string, refreshToken *string, tokenExpiry *time.Time) error {
	return nil
}
func (r *fakeAccountRepo) UpdateTokenByID(ctx context.Context, id int64, accessToken string, refreshToken, tokenExpiry *string) error {
	return nil
}
func (r *fakeAccountRepo) UpdateLastScanAt(ctx context.Context, id int64, scannedAt time.Time) error {
	return nil
}
func (r *fakeAccountRepo) Delete(ctx context.Context, id int64) error {
	return nil
}

type fakeBytesUploader struct {
	uploaded     []string
	contentTypes []string
	// failAfter>0 时，第 failAfter 次 UploadBytes 起返回错误（模拟中途失败）。
	failAfter int
}

func (u *fakeBytesUploader) Upload(ctx context.Context, key string, file *multipart.FileHeader, contentType string) (*storage.UploadResult, error) {
	return nil, errors.New("unused")
}
func (u *fakeBytesUploader) UploadBytes(ctx context.Context, key string, data []byte, contentType string) (*storage.UploadResult, error) {
	if u.failAfter > 0 && len(u.uploaded) >= u.failAfter-1 {
		return nil, errors.New("upload boom")
	}
	u.uploaded = append(u.uploaded, key)
	u.contentTypes = append(u.contentTypes, contentType)
	return &storage.UploadResult{Key: key, URL: "/" + key}, nil
}
func (u *fakeBytesUploader) Open(ctx context.Context, key string) (*storage.Object, error) {
	return nil, errors.New("unused")
}
func (u *fakeBytesUploader) Delete(ctx context.Context, key string) error { return nil }
