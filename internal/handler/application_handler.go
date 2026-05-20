package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/response"
	"github.com/yangnuowen1-arch/resume_back/internal/service"
)

type ApplicationHandler struct {
	service service.ApplicationService
}

func NewApplicationHandler(service service.ApplicationService) *ApplicationHandler {
	return &ApplicationHandler{
		service: service,
	}
}

// Create 创建投递记录
// @Summary 创建投递记录
// @Description 将简历投递到指定岗位，生成一条投递记录
// @Tags 投递记录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateApplicationRequest true "创建投递记录请求"
// @Success 201 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /applications [post]
func (h *ApplicationHandler) Create(c *gin.Context) {
	var req dto.CreateApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}

	result, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}

		response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
		return
	}

	response.Created(c, result)
}

// List 查询投递记录列表
// @Summary 查询投递记录列表
// @Description 分页查询投递记录，可按岗位、候选人、简历、状态、来源和关键词筛选
// @Tags 投递记录
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Param keyword query string false "岗位名/候选人名/简历文件名关键词"
// @Param jobId query int false "岗位 ID"
// @Param candidateId query int false "候选人 ID"
// @Param resumeId query int false "简历 ID"
// @Param status query string false "投递状态"
// @Param source query string false "投递来源"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /applications [get]
func (h *ApplicationHandler) List(c *gin.Context) {
	page, pageSize := parsePageParams(c)

	jobID, ok := parseOptionalInt64Query(c, "jobId")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "jobId 参数不合法", nil)
		return
	}

	candidateID, ok := parseOptionalInt64Query(c, "candidateId")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "candidateId 参数不合法", nil)
		return
	}

	resumeID, ok := parseOptionalInt64Query(c, "resumeId")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "resumeId 参数不合法", nil)
		return
	}

	query := dto.ApplicationQuery{
		Page:        page,
		PageSize:    pageSize,
		Keyword:     c.Query("keyword"),
		JobID:       jobID,
		CandidateID: candidateID,
		ResumeID:    resumeID,
		Status:      c.Query("status"),
		Source:      c.Query("source"),
	}

	items, total, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "查询投递记录失败", err.Error())
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
