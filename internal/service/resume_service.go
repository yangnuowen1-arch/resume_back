package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/filemime"
	"github.com/yangnuowen1-arch/resume_back/internal/parser"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
	"github.com/yangnuowen1-arch/resume_back/internal/storage"
)

type ResumeService interface {
	CreateUploadedResume(ctx context.Context, req dto.UploadResumeRequest) (*dto.ResumeResponse, error)
	List(ctx context.Context, query dto.ResumeQuery) ([]dto.ResumeResponse, int64, error)
	Parse(ctx context.Context, id int64) (*dto.ResumeResponse, error)
	GetPreviewFile(ctx context.Context, id int64) (*ResumePreviewFile, error)
}

// ResumePreviewFile is the trusted metadata needed to stream a PDF to the
// browser. ObjectKey never comes from the request, so callers cannot use this
// endpoint to read arbitrary objects from the configured storage backend.
type ResumePreviewFile struct {
	ObjectKey   string
	Filename    string
	ContentType string
	Size        int64
}

type resumeService struct {
	resumeRepo    repository.ResumeRepository
	candidateRepo repository.CandidateRepository
	uploader      storage.Uploader
	resumeParser  parser.Parser
}

func NewResumeService(resumeRepo repository.ResumeRepository, candidateRepo repository.CandidateRepository, uploader storage.Uploader, resumeParser parser.Parser) ResumeService {
	if resumeParser == nil {
		defaultParser := parser.NewPlainTextParser()
		resumeParser = defaultParser
	}

	return &resumeService{
		resumeRepo:    resumeRepo,
		candidateRepo: candidateRepo,
		uploader:      uploader,
		resumeParser:  resumeParser,
	}
}

func (s *resumeService) CreateUploadedResume(ctx context.Context, req dto.UploadResumeRequest) (*dto.ResumeResponse, error) {
	userID, err := currentUserID(ctx)
	if err != nil {
		return nil, err
	}

	req.OriginalFilename = strings.TrimSpace(req.OriginalFilename)
	req.FileKey = strings.TrimSpace(req.FileKey)
	req.FileURL = strings.TrimSpace(req.FileURL)
	req.FileType = strings.TrimSpace(req.FileType)
	req.RawText = trimOptionalString(req.RawText)
	req.Language = trimOptionalString(req.Language)
	if req.OriginalFilename == "" {
		return nil, errors.New("原始文件名不能为空")
	}
	if req.FileKey == "" {
		return nil, errors.New("简历文件 key 不能为空")
	}
	if req.FileURL == "" {
		return nil, errors.New("简历文件地址不能为空")
	}
	if req.FileSize <= 0 {
		return nil, errors.New("简历文件大小不合法")
	}

	if req.CandidateID != nil {
		if _, err := s.candidateRepo.FindByID(ctx, *req.CandidateID); err != nil {
			return nil, errors.New("候选人不存在")
		}
	}

	resume := &model.Resume{
		CandidateID:      req.CandidateID,
		OriginalFilename: &req.OriginalFilename,
		FileKey:          &req.FileKey,
		FileURL:          &req.FileURL,
		FileType:         &req.FileType,
		FileSize:         &req.FileSize,
		RawText:          req.RawText,
		ParseStatus:      initialResumeParseStatus(req.RawText),
		Language:         req.Language,
		UploadBy:         &userID,
	}
	if err := s.resumeRepo.Create(ctx, resume); err != nil {
		return nil, err
	}

	return toResumeResponse(resume), nil
}

func (s *resumeService) List(ctx context.Context, query dto.ResumeQuery) ([]dto.ResumeResponse, int64, error) {
	query = normalizeResumeQuery(query)

	if query.CandidateID != nil {
		if _, err := s.candidateRepo.FindByID(ctx, *query.CandidateID); err != nil {
			return nil, 0, errors.New("候选人不存在")
		}
	}

	items, total, err := s.resumeRepo.List(ctx, strings.TrimSpace(query.Keyword), query.CandidateID, strings.TrimSpace(query.Language), query.Page, query.PageSize)
	if err != nil {
		return nil, 0, err
	}

	result := make([]dto.ResumeResponse, 0, len(items))
	for _, item := range items {
		result = append(result, dto.ResumeResponse{
			ID:               item.ID,
			CandidateID:      item.CandidateID,
			CandidateName:    item.CandidateName,
			OriginalFilename: item.OriginalFilename,
			FileKey:          item.FileKey,
			FileURL:          item.FileURL,
			PreviewURL:       resumePreviewURL(pointerToInt64(item.ID), item.OriginalFilename, item.FileType),
			FileType:         item.FileType,
			FileSize:         item.FileSize,
			RawText:          item.RawText,
			ParsedData:       item.ParsedData,
			ParseStatus:      item.ParseStatus,
			ParseError:       item.ParseError,
			ParsedAt:         item.ParsedAt,
			Language:         item.Language,
			UploadBy:         item.UploadBy,
			UploadedAt:       item.UploadedAt,
			CreatedAt:        item.CreatedAt,
			UpdatedAt:        item.UpdatedAt,
		})
	}

	return result, total, nil
}

// GetPreviewFile returns metadata for a PDF preview after checking that the
// caller is authenticated and that the requested resume exists.
func (s *resumeService) GetPreviewFile(ctx context.Context, id int64) (*ResumePreviewFile, error) {
	if _, err := currentUserID(ctx); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, newInvalidParameterError("简历 ID 不合法")
	}

	resume, err := s.resumeRepo.FindByID(ctx, id)
	if err != nil || resume == nil {
		return nil, ErrResumeNotFound
	}

	objectKey := resumeObjectKey(resume)
	if objectKey == "" {
		return nil, ErrResumeFileUnavailable
	}

	filename := resumeStringValue(resume.OriginalFilename)
	if filename == "" {
		filename = "resume.pdf"
	}
	contentType := filemime.Normalize(resumeStringValue(resume.FileType), filename)
	if contentType != "application/pdf" {
		return nil, ErrResumeNotPDF
	}

	var size int64
	if resume.FileSize != nil && *resume.FileSize > 0 {
		size = *resume.FileSize
	}

	return &ResumePreviewFile{
		ObjectKey:   objectKey,
		Filename:    filename,
		ContentType: contentType,
		Size:        size,
	}, nil
}

func (s *resumeService) Parse(ctx context.Context, id int64) (*dto.ResumeResponse, error) {
	if _, err := currentUserID(ctx); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, errors.New("简历 ID 不合法")
	}
	if s.uploader == nil {
		return nil, errors.New("简历文件存储未初始化")
	}
	if s.resumeParser == nil {
		return nil, errors.New("简历解析器未初始化")
	}

	resume, err := s.resumeRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.New("简历不存在")
	}

	objectKey := resumeObjectKey(resume)
	if objectKey == "" {
		message := "简历文件 key 不能为空"
		_ = s.resumeRepo.MarkParseFailed(ctx, id, message)
		return nil, errors.New(message)
	}

	if err := s.resumeRepo.MarkParsing(ctx, id); err != nil {
		return nil, err
	}

	object, err := s.uploader.Open(ctx, objectKey)
	if err != nil {
		message := "打开简历文件失败: " + err.Error()
		_ = s.resumeRepo.MarkParseFailed(ctx, id, message)
		return nil, errors.New(message)
	}
	if object == nil || object.Body == nil {
		message := "简历文件内容为空"
		_ = s.resumeRepo.MarkParseFailed(ctx, id, message)
		return nil, errors.New(message)
	}
	defer object.Body.Close()

	parseResult, err := s.resumeParser.Parse(object.Body)
	if err != nil {
		message := "解析简历失败: " + err.Error()
		_ = s.resumeRepo.MarkParseFailed(ctx, id, message)
		return nil, errors.New(message)
	}
	if parseResult == nil {
		message := "简历解析结果为空"
		_ = s.resumeRepo.MarkParseFailed(ctx, id, message)
		return nil, errors.New(message)
	}

	rawText := strings.TrimSpace(parseResult.RawText)
	if rawText == "" {
		message := "简历文本为空"
		_ = s.resumeRepo.MarkParseFailed(ctx, id, message)
		return nil, errors.New(message)
	}

	parsedAt := time.Now()
	if err := s.resumeRepo.MarkParsed(ctx, id, rawText, parseResult.ParsedData, trimOptionalString(parseResult.Language), parsedAt); err != nil {
		return nil, err
	}

	parsedResume, err := s.resumeRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return toResumeResponse(parsedResume), nil
}

func toResumeResponse(resume *model.Resume) *dto.ResumeResponse {
	resumeID := resume.ID
	return &dto.ResumeResponse{
		ID:               resume.ID,
		CandidateID:      resume.CandidateID,
		OriginalFilename: resume.OriginalFilename,
		FileKey:          resume.FileKey,
		FileURL:          resume.FileURL,
		PreviewURL:       resumePreviewURL(&resumeID, resume.OriginalFilename, resume.FileType),
		FileType:         resume.FileType,
		FileSize:         resume.FileSize,
		RawText:          resume.RawText,
		ParsedData:       resume.ParsedData,
		ParseStatus:      resume.ParseStatus,
		ParseError:       resume.ParseError,
		ParsedAt:         resume.ParsedAt,
		Language:         resume.Language,
		UploadBy:         resume.UploadBy,
		UploadedAt:       resume.UploadedAt,
		CreatedAt:        resume.CreatedAt,
		UpdatedAt:        resume.UpdatedAt,
	}
}

func resumePreviewURL(resumeID *int64, filename, fileType *string) *string {
	if resumeID == nil || *resumeID <= 0 {
		return nil
	}
	if filemime.Normalize(resumeStringValue(fileType), resumeStringValue(filename)) != "application/pdf" {
		return nil
	}

	url := fmt.Sprintf("/api/v1/resumes/%d/file", *resumeID)
	return &url
}

func resumeStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func pointerToInt64(value int64) *int64 {
	return &value
}

func normalizeResumeQuery(query dto.ResumeQuery) dto.ResumeQuery {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
	}

	return query
}

func resumeObjectKey(resume *model.Resume) string {
	if resume == nil {
		return ""
	}
	if resume.FileKey != nil {
		if key := strings.TrimSpace(*resume.FileKey); key != "" {
			return key
		}
	}
	if resume.FileURL == nil {
		return ""
	}

	fileURL := strings.TrimSpace(*resume.FileURL)
	if fileURL == "" {
		return ""
	}
	if strings.HasPrefix(fileURL, "/uploads/") {
		return strings.TrimPrefix(fileURL, "/uploads/")
	}
	if strings.HasPrefix(fileURL, "uploads/") {
		return strings.TrimPrefix(fileURL, "uploads/")
	}

	parsed, err := url.Parse(fileURL)
	if err == nil && parsed.Scheme == "r2" {
		return strings.TrimPrefix(parsed.Path, "/")
	}
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		return resumeKeyFromURLPath(parsed.Path)
	}

	return ""
}

func resumeKeyFromURLPath(path string) string {
	path = strings.TrimPrefix(strings.TrimSpace(path), "/")
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "resumes/") {
		return path
	}
	if index := strings.Index(path, "/resumes/"); index >= 0 {
		return strings.TrimPrefix(path[index:], "/")
	}

	return ""
}
