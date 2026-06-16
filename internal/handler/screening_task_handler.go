package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/response"
	"github.com/yangnuowen1-arch/resume_back/internal/service"
)

type ScreeningTaskHandler struct {
	service service.ScreeningTaskService
}

func NewScreeningTaskHandler(service service.ScreeningTaskService) *ScreeningTaskHandler {
	return &ScreeningTaskHandler{service: service}
}

// RunResumeScreening 运行 Dify 简历筛选
// @Summary 运行 Dify 简历筛选
// @Description Go 后端根据 resumeId 和 jobId 读取简历文件与岗位上下文，调用 Dify workflow 后保存 screening_results
// @Tags 筛选任务
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.RunResumeScreeningRequest true "运行简历筛选请求"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /screening-tasks/run [post]
func (h *ScreeningTaskHandler) RunResumeScreening(c *gin.Context) {
	var req dto.RunResumeScreeningRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}

	result, err := h.service.RunResumeScreening(c.Request.Context(), req)
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

// List 查询简历筛选任务列表
// @Summary 查询简历筛选任务列表
// @Description 分页查询 screening_results 任务记录，可按候选人/岗位关键词、状态、岗位和候选人筛选；返回 candidate/position/aiScore/status/date 供前端表格展示
// @Tags 筛选任务
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Param keyword query string false "候选人姓名/邮箱/手机号/岗位名称关键词"
// @Param status query string false "筛选任务状态，传 all 表示全部"
// @Param jobId query int false "岗位 ID"
// @Param candidateId query int false "候选人 ID"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /screening-tasks [get]
func (h *ScreeningTaskHandler) List(c *gin.Context) {
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

	query := dto.ScreeningTaskQuery{
		Page:        page,
		PageSize:    pageSize,
		Keyword:     c.Query("keyword"),
		Status:      c.Query("status"),
		JobID:       jobID,
		CandidateID: candidateID,
	}

	items, total, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "查询筛选任务失败", err.Error())
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
