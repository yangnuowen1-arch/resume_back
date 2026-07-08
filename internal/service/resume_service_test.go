package service

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"strings"
	"testing"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/auth"
	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/parser"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
	"github.com/yangnuowen1-arch/resume_back/internal/storage"
)

func TestResumeServiceParseMarksParsed(t *testing.T) {
	fileKey := "resumes/resume.txt"
	repo := &fakeResumeRepository{
		resume: &model.Resume{
			ID:          12,
			FileKey:     ptrString(fileKey),
			ParseStatus: ResumeParseStatusPending,
		},
	}
	uploader := &fakeResumeUploader{
		body: "  李四\n后端工程师  ",
	}
	service := &resumeService{
		resumeRepo:   repo,
		uploader:     uploader,
		resumeParser: parser.NewPlainTextParser(),
	}

	result, err := service.Parse(testUserContext(), 12)
	if err != nil {
		t.Fatalf("parse resume: %v", err)
	}

	if uploader.openedKey != fileKey {
		t.Fatalf("expected uploader to open %q, got %q", fileKey, uploader.openedKey)
	}
	if !repo.markParsingCalled {
		t.Fatal("expected resume to be marked parsing")
	}
	if repo.markParsedRawText != "李四\n后端工程师" {
		t.Fatalf("unexpected parsed raw text %q", repo.markParsedRawText)
	}
	if result.ParseStatus != ResumeParseStatusParsed {
		t.Fatalf("expected parsed status, got %q", result.ParseStatus)
	}
	if result.RawText == nil || *result.RawText != "李四\n后端工程师" {
		t.Fatalf("unexpected response raw text %#v", result.RawText)
	}
	if result.ParsedAt == nil {
		t.Fatal("expected parsedAt in response")
	}
}

func TestResumeServiceParseMarksFailedWhenParserFails(t *testing.T) {
	repo := &fakeResumeRepository{
		resume: &model.Resume{
			ID:          12,
			FileKey:     ptrString("resumes/resume.pdf"),
			ParseStatus: ResumeParseStatusPending,
		},
	}
	service := &resumeService{
		resumeRepo:   repo,
		uploader:     &fakeResumeUploader{body: "resume"},
		resumeParser: failingResumeParser{},
	}

	_, err := service.Parse(testUserContext(), 12)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "解析简历失败") {
		t.Fatalf("unexpected error %v", err)
	}
	if repo.failedMessage == "" {
		t.Fatal("expected failed message to be recorded")
	}
	if repo.resume.ParseStatus != ResumeParseStatusFailed {
		t.Fatalf("expected failed status, got %q", repo.resume.ParseStatus)
	}
}

func TestResumeObjectKeyDerivesKeyFromPublicR2URL(t *testing.T) {
	resume := &model.Resume{
		FileURL: ptrString("https://pub-208ce3c7a43045d9855c32b73760e0d5.r2.dev/resumes/ab086be6-5739-444a-9441-6ff219b4aca9.pdf"),
	}

	key := resumeObjectKey(resume)
	if key != "resumes/ab086be6-5739-444a-9441-6ff219b4aca9.pdf" {
		t.Fatalf("expected R2 object key, got %q", key)
	}
}

func testUserContext() context.Context {
	return auth.WithClaims(context.Background(), &auth.Claims{UserID: 1})
}

type fakeResumeRepository struct {
	resume            *model.Resume
	markParsingCalled bool
	markParsedRawText string
	failedMessage     string
}

func (r *fakeResumeRepository) Create(ctx context.Context, resume *model.Resume) error {
	return nil
}

func (r *fakeResumeRepository) FindByID(ctx context.Context, id int64) (*model.Resume, error) {
	if r.resume == nil || r.resume.ID != id {
		return nil, errors.New("not found")
	}

	copy := *r.resume
	return &copy, nil
}

func (r *fakeResumeRepository) FindByFileHash(ctx context.Context, fileHash string) (*model.Resume, error) {
	return nil, nil
}

func (r *fakeResumeRepository) List(ctx context.Context, keyword string, candidateID *int64, language string, page int, pageSize int) ([]repository.ResumeListItem, int64, error) {
	return nil, 0, nil
}

func (r *fakeResumeRepository) MarkParsing(ctx context.Context, id int64) error {
	r.markParsingCalled = true
	r.resume.ParseStatus = ResumeParseStatusParsing
	return nil
}

func (r *fakeResumeRepository) MarkParsed(ctx context.Context, id int64, rawText string, parsedData *string, language *string, parsedAt time.Time) error {
	r.markParsedRawText = rawText
	r.resume.RawText = &rawText
	r.resume.ParsedData = parsedData
	r.resume.ParseStatus = ResumeParseStatusParsed
	r.resume.ParseError = nil
	r.resume.ParsedAt = &parsedAt
	if language != nil {
		r.resume.Language = language
	}
	return nil
}

func (r *fakeResumeRepository) MarkParseFailed(ctx context.Context, id int64, message string) error {
	r.failedMessage = message
	r.resume.ParseStatus = ResumeParseStatusFailed
	r.resume.ParseError = &message
	return nil
}

type fakeResumeUploader struct {
	body      string
	openedKey string
	openErr   error
}

func (u *fakeResumeUploader) Upload(ctx context.Context, key string, file *multipart.FileHeader, contentType string) (*storage.UploadResult, error) {
	return nil, nil
}

func (u *fakeResumeUploader) UploadBytes(ctx context.Context, key string, data []byte, contentType string) (*storage.UploadResult, error) {
	return nil, nil
}

func (u *fakeResumeUploader) Open(ctx context.Context, key string) (*storage.Object, error) {
	u.openedKey = key
	if u.openErr != nil {
		return nil, u.openErr
	}

	return &storage.Object{
		Body: io.NopCloser(strings.NewReader(u.body)),
	}, nil
}

func (u *fakeResumeUploader) Delete(ctx context.Context, key string) error {
	return nil
}

type failingResumeParser struct{}

func (failingResumeParser) Parse(reader io.Reader) (*parser.Result, error) {
	return nil, errors.New("boom")
}

func ptrString(value string) *string {
	return &value
}
