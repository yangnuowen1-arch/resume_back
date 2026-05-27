package handler

import (
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/response"
	"github.com/yangnuowen1-arch/resume_back/internal/service"
	"github.com/yangnuowen1-arch/resume_back/internal/storage"
)

type ResumeHandler struct {
	service  service.ResumeService
	uploader storage.Uploader
}

func NewResumeHandler(service service.ResumeService, uploader storage.Uploader) *ResumeHandler {
	return &ResumeHandler{
		service:  service,
		uploader: uploader,
	}
}

// Upload 上传简历
// @Summary 上传简历
// @Description 上传简历文件并写入简历记录，支持可选绑定候选人
// @Tags 简历
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "简历文件"
// @Param candidateId formData int false "候选人 ID"
// @Param rawText formData string false "简历原始文本"
// @Param language formData string false "简历语言"
// @Success 201 {object} dto.UploadResumeResponse "上传成功，data 为简历记录"
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /resumes/upload [post]
func (h *ResumeHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "请上传简历文件", err.Error())
		return
	}

	candidateID, ok := parseOptionalInt64Form(c, "candidateId")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "candidateId 参数不合法", nil)
		return
	}

	// 只保留原始文件名，避免客户端传入带目录的文件名影响服务端保存路径。
	originalFilename := filepath.Base(file.Filename)
	// 取出文件扩展名并转成小写，后面生成服务端文件名时继续保留文件类型。
	ext := strings.ToLower(filepath.Ext(originalFilename))

	fileType := file.Header.Get("Content-Type")
	if fileType == "" {
		fileType = strings.TrimPrefix(ext, ".")
	}

	objectKey := "resumes/" + uuid.NewString() + ext
	uploadResult, err := h.uploader.Upload(c.Request.Context(), objectKey, file, fileType)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "上传简历文件失败", err.Error())
		return
	}

	req := dto.UploadResumeRequest{
		CandidateID:      candidateID,
		OriginalFilename: originalFilename,
		FileURL:          uploadResult.URL,
		FileType:         fileType,
		FileSize:         file.Size,
		RawText:          optionalFormString(c, "rawText"),
		Language:         optionalFormString(c, "language"),
	}

	result, err := h.service.CreateUploadedResume(c.Request.Context(), req)
	if err != nil {
		_ = h.uploader.Delete(c.Request.Context(), uploadResult.Key)
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}

		response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
		return
	}

	response.Created(c, result)
}

// List 查询简历列表
// @Summary 查询简历列表
// @Description 分页查询简历，可按候选人、关键词和语言筛选
// @Tags 简历
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Param keyword query string false "文件名/简历文本/候选人姓名关键词"
// @Param candidateId query int false "候选人 ID"
// @Param language query string false "简历语言"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /resumes [get]
func (h *ResumeHandler) List(c *gin.Context) {
	page, pageSize := parsePageParams(c)

	candidateID, ok := parseOptionalInt64Query(c, "candidateId")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "candidateId 参数不合法", nil)
		return
	}

	query := dto.ResumeQuery{
		Page:        page,
		PageSize:    pageSize,
		Keyword:     c.Query("keyword"),
		CandidateID: candidateID,
		Language:    c.Query("language"),
	}

	items, total, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
		return
	}

	response.Success(c, response.PageResult{
		Items: items,
		Pagination: response.Pagination{
			Page:       query.Page,
			PageSize:   query.PageSize,
			Total:      total,
			TotalPages: totalPages(total, query.PageSize),
		},
	})
}

func parseOptionalInt64Form(c *gin.Context, key string) (*int64, bool) {
	value := strings.TrimSpace(c.PostForm(key))
	if value == "" {
		return nil, true
	}

	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return nil, false
	}

	return &id, true
}

func optionalFormString(c *gin.Context, key string) *string {
	value := strings.TrimSpace(c.PostForm(key))
	if value == "" {
		return nil
	}

	return &value
}
