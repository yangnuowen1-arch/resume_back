package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"mime/multipart"
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
		name      string
		filename  string
		fromName  string
		fromEmail string
		want      string
	}{
		{"文件名优先", "张三.pdf", "李四", "zhangsan@x.com", "张三"},
		{"去扩展名", "王五-后端.docx", "", "", "王五-后端"},
		{"无意义回退发件人名", "resume.pdf", "赵六", "zhao@x.com", "赵六"},
		{"通用词大小写回退", "Resume.PDF", "钱七", "qian@x.com", "钱七"},
		{"中文简历回退发件人名", "简历.pdf", "孙八", "sun@x.com", "孙八"},
		{"无名回退邮箱前缀", "cv.docx", "", "john.doe@example.com", "john.doe"},
		{"全部为空则用文件名", "resume.pdf", "", "", "resume"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := deriveCandidateName(c.filename, c.fromName, c.fromEmail)
			if got != c.want {
				t.Fatalf("deriveCandidateName(%q,%q,%q) = %q, want %q", c.filename, c.fromName, c.fromEmail, got, c.want)
			}
		})
	}
}

// 附件白名单过滤：只导入 .pdf/.docx，签名图等被忽略。
func TestProcessMessageFiltersAttachments(t *testing.T) {
	svc, deps := newTestMailboxService()
	msg := mailbox.Message{ID: "m1", FromEmail: "a@x.com", FromName: "甲", HasAttachments: true}
	deps.provider.attachments = []mailbox.Attachment{
		{Filename: "resume-甲.pdf", Data: []byte("PDF-A")},
		{Filename: "signature.png", Data: []byte("IMG")},
		{Filename: "notes.txt", Data: []byte("TXT")},
	}

	scanned, imported, skipped, err := svc.processMessage(context.Background(), deps.provider, &oauth2.Token{}, 1, msg)
	if err != nil {
		t.Fatalf("processMessage: %v", err)
	}
	if !scanned || imported != 1 || skipped != 0 {
		t.Fatalf("scanned=%v imported=%d skipped=%d, want true/1/0", scanned, imported, skipped)
	}
	if len(deps.candidate.created) != 1 {
		t.Fatalf("expected 1 candidate created, got %d", len(deps.candidate.created))
	}
	if len(deps.uploader.uploaded) != 1 {
		t.Fatalf("expected 1 upload, got %d", len(deps.uploader.uploaded))
	}
}

// file hash 命中已有简历则跳过，不新建候选人。
func TestProcessMessageSkipsDuplicateByHash(t *testing.T) {
	svc, deps := newTestMailboxService()
	data := []byte("DUP-RESUME")
	sum := sha256.Sum256(data)
	deps.resume.byHash[hex.EncodeToString(sum[:])] = &model.Resume{ID: 99}

	msg := mailbox.Message{ID: "m1", FromEmail: "a@x.com", HasAttachments: true}
	deps.provider.attachments = []mailbox.Attachment{{Filename: "resume.pdf", Data: data}}

	scanned, imported, skipped, err := svc.processMessage(context.Background(), deps.provider, &oauth2.Token{}, 1, msg)
	if err != nil {
		t.Fatalf("processMessage: %v", err)
	}
	if !scanned || imported != 0 || skipped != 1 {
		t.Fatalf("scanned=%v imported=%d skipped=%d, want true/0/1", scanned, imported, skipped)
	}
	if len(deps.candidate.created) != 0 {
		t.Fatalf("expected no candidate created, got %d", len(deps.candidate.created))
	}
	if len(deps.uploader.uploaded) != 0 {
		t.Fatalf("expected no upload on hash hit, got %d", len(deps.uploader.uploaded))
	}
}

// 发件人 email 命中已有候选人：挂到旧候选人，不新建。
func TestProcessMessageMergesByEmail(t *testing.T) {
	svc, deps := newTestMailboxService()
	deps.candidate.byEmail["dup@x.com"] = &model.Candidate{ID: 42}

	msg := mailbox.Message{ID: "m1", FromEmail: "dup@x.com", FromName: "老候选人", HasAttachments: true}
	deps.provider.attachments = []mailbox.Attachment{{Filename: "new-resume.pdf", Data: []byte("NEW")}}

	scanned, imported, skipped, err := svc.processMessage(context.Background(), deps.provider, &oauth2.Token{}, 1, msg)
	if err != nil {
		t.Fatalf("processMessage: %v", err)
	}
	if !scanned || imported != 1 || skipped != 0 {
		t.Fatalf("scanned=%v imported=%d skipped=%d, want true/1/0", scanned, imported, skipped)
	}
	if len(deps.candidate.created) != 0 {
		t.Fatalf("expected no new candidate, got %d", len(deps.candidate.created))
	}
	if deps.candidate.resumeForID != 42 {
		t.Fatalf("expected resume attached to candidate 42, got %d", deps.candidate.resumeForID)
	}
}

// 已处理过的邮件（MarkProcessed 返回 false）直接跳过，不拉附件。
func TestProcessMessageSkipsAlreadyProcessed(t *testing.T) {
	svc, deps := newTestMailboxService()
	deps.message.processed["m1"] = struct{}{}

	msg := mailbox.Message{ID: "m1", FromEmail: "a@x.com", HasAttachments: true}
	deps.provider.attachments = []mailbox.Attachment{{Filename: "resume.pdf", Data: []byte("X")}}

	scanned, imported, skipped, err := svc.processMessage(context.Background(), deps.provider, &oauth2.Token{}, 1, msg)
	if err != nil {
		t.Fatalf("processMessage: %v", err)
	}
	if scanned || imported != 0 || skipped != 0 {
		t.Fatalf("scanned=%v imported=%d skipped=%d, want false/0/0", scanned, imported, skipped)
	}
	if deps.provider.fetchCalls != 0 {
		t.Fatalf("expected no FetchAttachments on already-processed, got %d", deps.provider.fetchCalls)
	}
}

// 一封邮件多个附件、同一发件人：只建一个候选人，其余简历挂到同一人。
func TestProcessMessageMultipleAttachmentsOneCandidate(t *testing.T) {
	svc, deps := newTestMailboxService()
	msg := mailbox.Message{ID: "m1", FromEmail: "a@x.com", FromName: "甲", HasAttachments: true}
	deps.provider.attachments = []mailbox.Attachment{
		{Filename: "简历-甲.pdf", Data: []byte("A1")},
		{Filename: "作品集.pdf", Data: []byte("A2")},
		{Filename: "证书.docx", Data: []byte("A3")},
	}

	scanned, imported, skipped, err := svc.processMessage(context.Background(), deps.provider, &oauth2.Token{}, 1, msg)
	if err != nil {
		t.Fatalf("processMessage: %v", err)
	}
	if !scanned || imported != 3 || skipped != 0 {
		t.Fatalf("scanned=%v imported=%d skipped=%d, want true/3/0", scanned, imported, skipped)
	}
	if len(deps.candidate.created) != 1 {
		t.Fatalf("expected exactly 1 candidate created, got %d", len(deps.candidate.created))
	}
	// 首份走 CreateWithResume，其余 2 份走 CreateResumeForCandidate。
	if deps.candidate.resumeForCalls != 2 {
		t.Fatalf("expected 2 CreateResumeForCandidate calls, got %d", deps.candidate.resumeForCalls)
	}
	if len(deps.uploader.uploaded) != 3 {
		t.Fatalf("expected 3 uploads, got %d", len(deps.uploader.uploaded))
	}
}

// 附件中途失败：邮件不登记 mailbox_messages，下次扫描可重试。
func TestProcessMessageDoesNotMarkProcessedOnFailure(t *testing.T) {
	svc, deps := newTestMailboxService()
	deps.uploader.failAfter = 2 // 第 2 次上传起失败
	msg := mailbox.Message{ID: "m1", FromEmail: "a@x.com", HasAttachments: true}
	deps.provider.attachments = []mailbox.Attachment{
		{Filename: "a.pdf", Data: []byte("A1")},
		{Filename: "b.pdf", Data: []byte("A2")},
	}

	scanned, _, _, err := svc.processMessage(context.Background(), deps.provider, &oauth2.Token{}, 1, msg)
	if err == nil {
		t.Fatal("expected error when an attachment upload fails")
	}
	if scanned {
		t.Fatal("scanned should be false on failure")
	}
	if _, ok := deps.message.processed["m1"]; ok {
		t.Fatal("message must NOT be marked processed on failure (so next scan retries)")
	}
	if len(deps.provider.markedRead) != 0 {
		t.Fatalf("message must NOT be marked read on failure, got %v", deps.provider.markedRead)
	}
}

// --- 测试脚手架 ---

type mailboxTestDeps struct {
	provider  *fakeProvider
	message   *fakeMessageRepo
	candidate *fakeCandidateRepo
	resume    *fakeMailboxResumeRepo
	account   *fakeAccountRepo
	uploader  *fakeBytesUploader
}

func newTestMailboxService() (*mailboxService, *mailboxTestDeps) {
	deps := &mailboxTestDeps{
		provider:  &fakeProvider{},
		message:   &fakeMessageRepo{processed: map[string]struct{}{}},
		candidate: &fakeCandidateRepo{byEmail: map[string]*model.Candidate{}},
		resume:    &fakeMailboxResumeRepo{byHash: map[string]*model.Resume{}},
		account:   &fakeAccountRepo{},
		uploader:  &fakeBytesUploader{},
	}
	svc := &mailboxService{
		accountRepo:   deps.account,
		messageRepo:   deps.message,
		candidateRepo: deps.candidate,
		resumeRepo:    deps.resume,
		uploader:      deps.uploader,
		providers:     map[string]mailbox.Provider{"google": deps.provider},
		allowedExt:    mailbox.AllowedExtSet(".pdf,.docx"),
		running:       map[int64]struct{}{},
	}
	return svc, deps
}

type fakeProvider struct {
	attachments []mailbox.Attachment
	fetchCalls  int
	markedRead  []string
}

func (p *fakeProvider) Provider() string                       { return "google" }
func (p *fakeProvider) AuthURL(state string) string            { return "" }
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

type fakeMessageRepo struct {
	processed map[string]struct{}
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

type fakeCandidateRepo struct {
	byEmail        map[string]*model.Candidate
	created        []*model.Candidate
	resumeForID    int64
	resumeForCalls int
}

func (r *fakeCandidateRepo) FindByEmail(ctx context.Context, email string) (*model.Candidate, error) {
	return r.byEmail[email], nil
}
func (r *fakeCandidateRepo) CreateWithResume(ctx context.Context, candidate *model.Candidate, resume *model.Resume) error {
	candidate.ID = int64(len(r.created) + 1)
	r.created = append(r.created, candidate)
	return nil
}
func (r *fakeCandidateRepo) CreateResumeForCandidate(ctx context.Context, candidateID int64, resume *model.Resume, candidateStatus string) error {
	r.resumeForID = candidateID
	r.resumeForCalls++
	return nil
}
func (r *fakeCandidateRepo) Create(ctx context.Context, candidate *model.Candidate) error {
	return nil
}
func (r *fakeCandidateRepo) EnqueueScreening(ctx context.Context, candidateID int64, jobID *int64, createdBy int64, candidateStatus string) repository.CandidateAnalysisResult {
	return repository.CandidateAnalysisResult{}
}
func (r *fakeCandidateRepo) Update(ctx context.Context, candidate *model.Candidate) error { return nil }
func (r *fakeCandidateRepo) UpdateWithResume(ctx context.Context, candidate *model.Candidate, resume *model.Resume) error {
	return nil
}
func (r *fakeCandidateRepo) FindByID(ctx context.Context, id int64) (*model.Candidate, error) {
	return nil, nil
}
func (r *fakeCandidateRepo) List(ctx context.Context, filter repository.CandidateListFilter) ([]repository.CandidateListItem, int64, error) {
	return nil, 0, nil
}
func (r *fakeCandidateRepo) ActivePositionCategoryExists(ctx context.Context, id int64) (bool, error) {
	return false, nil
}
func (r *fakeCandidateRepo) FindJobSelectionByID(ctx context.Context, id int64) (repository.CandidateJobSelection, error) {
	return repository.CandidateJobSelection{}, nil
}

type fakeMailboxResumeRepo struct {
	byHash  map[string]*model.Resume
	created []*model.Resume
}

func (r *fakeMailboxResumeRepo) FindByFileHash(ctx context.Context, fileHash string) (*model.Resume, error) {
	return r.byHash[fileHash], nil
}
func (r *fakeMailboxResumeRepo) Create(ctx context.Context, resume *model.Resume) error {
	r.created = append(r.created, resume)
	return nil
}
func (r *fakeMailboxResumeRepo) FindByID(ctx context.Context, id int64) (*model.Resume, error) {
	return nil, nil
}
func (r *fakeMailboxResumeRepo) List(ctx context.Context, keyword string, candidateID *int64, language string, page int, pageSize int) ([]repository.ResumeListItem, int64, error) {
	return nil, 0, nil
}
func (r *fakeMailboxResumeRepo) MarkParsing(ctx context.Context, id int64) error { return nil }
func (r *fakeMailboxResumeRepo) MarkParsed(ctx context.Context, id int64, rawText string, parsedData *string, language *string, parsedAt time.Time) error {
	return nil
}
func (r *fakeMailboxResumeRepo) MarkParseFailed(ctx context.Context, id int64, message string) error {
	return nil
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
	uploaded []string
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
	return &storage.UploadResult{Key: key, URL: "/" + key}, nil
}
func (u *fakeBytesUploader) Open(ctx context.Context, key string) (*storage.Object, error) {
	return nil, errors.New("unused")
}
func (u *fakeBytesUploader) Delete(ctx context.Context, key string) error { return nil }
