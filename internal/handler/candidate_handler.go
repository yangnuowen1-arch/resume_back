package handler

import (
	"errors"
	"mime/multipart"
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

type CandidateHandler struct {
	service  service.CandidateService
	uploader storage.Uploader
}

func NewCandidateHandler(service service.CandidateService, uploader storage.Uploader) *CandidateHandler {
	return &CandidateHandler{
		service:  service,
		uploader: uploader,
	}
}

// Create 创建候选人
// @Summary 创建候选人
// @Description 创建候选人基础档案；支持 positionCategoryId/currentJobId 关联岗位分类和岗位；gender 只接受 男/女，source 只接受 boss/email，highestEducation 只接受 专科/本科/硕士/博士；推荐使用 multipart/form-data 同时上传 file 简历文件，兼容旧 JSON 创建
// @Tags 候选人
// @Accept json
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateCandidateRequest true "创建候选人请求"
// @Param file formData file false "简历文件，multipart 创建时必传"
// @Success 201 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /candidates [post]
func (h *CandidateHandler) Create(c *gin.Context) {
	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		h.createWithResume(c)
		return
	}

	var req dto.CreateCandidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}

	id, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}

		response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
		return
	}

	response.Created(c, gin.H{"id": id})
}

func (h *CandidateHandler) createWithResume(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "请上传简历文件", err.Error())
		return
	}

	yearsOfExperience, ok := optionalFloat64Form(c, "yearsOfExperience")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "yearsOfExperience 参数不合法", nil)
		return
	}
	positionCategoryID, ok := parseOptionalInt64Form(c, "positionCategoryId")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "positionCategoryId 参数不合法", nil)
		return
	}
	currentJobID, ok := parseOptionalInt64Form(c, "currentJobId")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "currentJobId 参数不合法", nil)
		return
	}

	resumeReq, objectKey, err := h.uploadCandidateResumeFile(c, file)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "上传简历文件失败", err.Error())
		return
	}

	candidateReq := dto.CreateCandidateRequest{
		Name:                    strings.TrimSpace(c.PostForm("name")),
		Email:                   optionalFormString(c, "email"),
		Phone:                   optionalFormString(c, "phone"),
		Gender:                  optionalFormString(c, "gender"),
		CurrentCompany:          optionalFormString(c, "currentCompany"),
		PositionCategoryID:      positionCategoryID,
		CurrentJobID:            currentJobID,
		CurrentPosition:         optionalFormString(c, "currentPosition"),
		CurrentPositionCategory: optionalFormString(c, "currentPositionCategory"),
		YearsOfExperience:       yearsOfExperience,
		HighestEducation:        optionalFormString(c, "highestEducation"),
		School:                  optionalFormString(c, "school"),
		Major:                   optionalFormString(c, "major"),
		Location:                optionalFormString(c, "location"),
		Source:                  optionalFormString(c, "source"),
		Status:                  strings.TrimSpace(c.PostForm("status")),
	}
	candidateID, resumeID, err := h.service.CreateWithResume(c.Request.Context(), candidateReq, resumeReq)
	if err != nil {
		_ = h.uploader.Delete(c.Request.Context(), objectKey)
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}

		response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
		return
	}

	response.Created(c, gin.H{"id": candidateID, "resumeId": resumeID, "language": resumeReq.Language, "resumeLanguage": resumeReq.Language})
}

// UploadResume 给候选人上传/替换简历
// @Summary 给候选人上传/替换简历
// @Description 给已有候选人上传一份新的简历；列表会以最新简历作为展示和分析对象
// @Tags 候选人
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path int true "候选人 ID"
// @Param file formData file true "简历文件"
// @Param rawText formData string false "简历原始文本"
// @Param language formData string false "简历语言"
// @Success 201 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /candidates/{id}/resume [post]
func (h *CandidateHandler) UploadResume(c *gin.Context) {
	candidateID, ok := parseInt64Param(c, "id")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "候选人 ID 不合法", nil)
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "请上传简历文件", err.Error())
		return
	}

	resumeReq, objectKey, err := h.uploadCandidateResumeFile(c, file)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "上传简历文件失败", err.Error())
		return
	}

	resumeID, err := h.service.UploadResume(c.Request.Context(), candidateID, resumeReq)
	if err != nil {
		_ = h.uploader.Delete(c.Request.Context(), objectKey)
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}

		response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
		return
	}

	response.Created(c, gin.H{"id": candidateID, "resumeId": resumeID})
}

// BatchAnalyze 批量分析候选人简历
// @Summary 批量分析候选人简历
// @Description 按候选人 ID 批量创建分析任务；不传 jobId 时使用候选人最新简历已有投递岗位，传 jobId 时会为最新简历创建/复用投递记录
// @Tags 候选人
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.BatchAnalyzeCandidatesRequest true "批量分析请求"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /candidates/batch-analyze [post]
func (h *CandidateHandler) BatchAnalyze(c *gin.Context) {
	var req dto.BatchAnalyzeCandidatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}

	result, err := h.service.BatchAnalyze(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}

		response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
		return
	}

	response.Success(c, result)
}

// Update 编辑候选人
// @Summary 编辑候选人
// @Description 根据 ID 编辑候选人基础档案；支持 positionCategoryId/currentJobId 关联岗位分类和岗位；gender 只接受 男/女，source 只接受 boss/email，highestEducation 只接受 专科/本科/硕士/博士
// @Tags 候选人
// @Accept json
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path int true "候选人 ID"
// @Param request body dto.UpdateCandidateRequest true "编辑候选人请求"
// @Param file formData file false "新的简历文件，multipart 更新时可选"
// @Param rawText formData string false "简历原始文本"
// @Param language formData string false "简历语言"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /candidates/{id} [put]
func (h *CandidateHandler) Update(c *gin.Context) {
	id, ok := parseInt64Param(c, "id")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "候选人 ID 不合法", nil)
		return
	}
	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		h.updateWithMultipart(c, id)
		return
	}

	var req dto.UpdateCandidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}

	if err := h.service.Update(c.Request.Context(), id, req); err != nil {
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}

		response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
		return
	}

	response.Success(c, gin.H{"id": id})
}

func (h *CandidateHandler) updateWithMultipart(c *gin.Context, id int64) {
	yearsOfExperience, ok := optionalFloat64Form(c, "yearsOfExperience")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "yearsOfExperience 参数不合法", nil)
		return
	}
	positionCategoryID, ok := parseOptionalInt64Form(c, "positionCategoryId")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "positionCategoryId 参数不合法", nil)
		return
	}
	currentJobID, ok := parseOptionalInt64Form(c, "currentJobId")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "currentJobId 参数不合法", nil)
		return
	}

	req := dto.UpdateCandidateRequest{
		Name:                    strings.TrimSpace(c.PostForm("name")),
		Email:                   optionalFormString(c, "email"),
		Phone:                   optionalFormString(c, "phone"),
		Gender:                  optionalFormString(c, "gender"),
		CurrentCompany:          optionalFormString(c, "currentCompany"),
		PositionCategoryID:      positionCategoryID,
		CurrentJobID:            currentJobID,
		CurrentPosition:         optionalFormString(c, "currentPosition"),
		CurrentPositionCategory: optionalFormString(c, "currentPositionCategory"),
		YearsOfExperience:       yearsOfExperience,
		HighestEducation:        optionalFormString(c, "highestEducation"),
		School:                  optionalFormString(c, "school"),
		Major:                   optionalFormString(c, "major"),
		Location:                optionalFormString(c, "location"),
		Source:                  optionalFormString(c, "source"),
		Status:                  strings.TrimSpace(c.PostForm("status")),
	}
	if req.Status == "" {
		response.Error(c, http.StatusBadRequest, 40001, "status 不能为空", nil)
		return
	}

	file, err := c.FormFile("file")
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		response.Error(c, http.StatusBadRequest, 40001, "简历文件参数错误", err.Error())
		return
	}
	if file == nil {
		if err := h.service.Update(c.Request.Context(), id, req); err != nil {
			if errors.Is(err, service.ErrUnauthenticated) {
				response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
				return
			}

			response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
			return
		}

		response.Success(c, gin.H{"id": id})
		return
	}

	resumeReq, objectKey, err := h.uploadCandidateResumeFile(c, file)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "上传简历文件失败", err.Error())
		return
	}

	resumeID, err := h.service.UpdateWithResume(c.Request.Context(), id, req, resumeReq)
	if err != nil {
		_ = h.uploader.Delete(c.Request.Context(), objectKey)
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}

		response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
		return
	}

	response.Success(c, gin.H{"id": id, "resumeId": resumeID, "language": resumeReq.Language, "resumeLanguage": resumeReq.Language})
}

// List 查询候选人列表
// @Summary 查询候选人列表
// @Description 分页查询候选人，可按姓名、邮箱、手机号关键词、来源和状态筛选，并返回 positionCategoryId/positionCategoryName/currentJobId；source 只接受 boss/email
// @Tags 候选人
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Param keyword query string false "姓名/邮箱/手机号关键词"
// @Param source query string false "候选人来源 boss/email"
// @Param status query string false "候选人状态"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /candidates [get]
func (h *CandidateHandler) List(c *gin.Context) {
	page, pageSize := parsePageParams(c)

	query := dto.CandidateQuery{
		Page:     page,
		PageSize: pageSize,
		Keyword:  c.Query("keyword"),
		Source:   c.Query("source"),
		Status:   c.Query("status"),
	}

	items, total, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		if errors.Is(err, service.ErrInvalidParameter) {
			response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
			return
		}

		response.Error(c, http.StatusInternalServerError, 50001, "查询候选人失败", err.Error())
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

// ListStatuses 查询候选人状态枚举
// @Summary 查询候选人状态枚举
// @Description 返回候选人/简历状态枚举，前端写死枚举时也应与这里保持一致
// @Tags 候选人
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Router /candidate-statuses [get]
func (h *CandidateHandler) ListStatuses(c *gin.Context) {
	response.Success(c, h.service.StatusOptions())
}

func optionalFloat64Form(c *gin.Context, key string) (*float64, bool) {
	value := strings.TrimSpace(c.PostForm(key))
	if value == "" {
		return nil, true
	}

	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, false
	}

	return &number, true
}

func (h *CandidateHandler) uploadCandidateResumeFile(c *gin.Context, fileHeader *multipart.FileHeader) (dto.UploadResumeRequest, string, error) {
	originalFilename := filepath.Base(fileHeader.Filename)
	ext := strings.ToLower(filepath.Ext(originalFilename))
	fileType := fileHeader.Header.Get("Content-Type")
	if fileType == "" {
		fileType = strings.TrimPrefix(ext, ".")
	}

	objectKey := "resumes/" + uuid.NewString() + ext
	uploadResult, err := h.uploader.Upload(c.Request.Context(), objectKey, fileHeader, fileType)
	if err != nil {
		return dto.UploadResumeRequest{}, "", err
	}

	return dto.UploadResumeRequest{
		OriginalFilename: originalFilename,
		FileURL:          uploadResult.URL,
		FileType:         fileType,
		FileSize:         fileHeader.Size,
		RawText:          optionalFormString(c, "rawText"),
		Language:         optionalFormString(c, "language"),
	}, uploadResult.Key, nil
}
