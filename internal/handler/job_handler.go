package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/response"
	"github.com/yangnuowen1-arch/resume_back/internal/service"
)

type JobHandler struct {
	service service.JobService
}

func NewJobHandler(service service.JobService) *JobHandler {
	return &JobHandler{
		service: service,
	}
}

// Create 创建岗位
// @Summary 创建岗位
// @Description 创建一个新的招聘岗位，支持关联岗位分类、负责人和岗位标签
// @Tags 岗位
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateJobRequest true "创建岗位请求"
// @Success 201 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /jobs [post]
func (h *JobHandler) Create(c *gin.Context) {
	var req dto.CreateJobRequest
	if err := bindCreateJobRequest(c, &req); err != nil {
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

// Get 查询岗位详情
// @Summary 查询岗位详情
// @Description 根据 ID 查询岗位详情，并返回岗位标签和动态字段
// @Tags 岗位
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "岗位 ID"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /jobs/{id} [get]
func (h *JobHandler) Get(c *gin.Context) {
	id, ok := parseInt64Param(c, "id")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "岗位 ID 不合法", nil)
		return
	}

	item, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
		return
	}

	response.Success(c, item)
}

// Update 编辑岗位
// @Summary 编辑岗位
// @Description 根据 ID 编辑岗位基础信息、要求、状态、优先级、负责人、动态字段和可选岗位标签；不传 tagIds 时不改标签，传空数组表示清空
// @Tags 岗位
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "岗位 ID"
// @Param request body dto.UpdateJobRequest true "编辑岗位请求"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /jobs/{id} [put]
func (h *JobHandler) Update(c *gin.Context) {
	id, ok := parseInt64Param(c, "id")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "岗位 ID 不合法", nil)
		return
	}

	var req dto.UpdateJobRequest
	if err := bindUpdateJobRequest(c, &req); err != nil {
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

// Delete 删除岗位
// @Summary 删除岗位
// @Description 根据 ID 删除岗位；岗位下存在投递记录时不允许删除
// @Tags 岗位
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "岗位 ID"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /jobs/{id} [delete]
func (h *JobHandler) Delete(c *gin.Context) {
	id, ok := parseInt64Param(c, "id")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "岗位 ID 不合法", nil)
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}

		response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
		return
	}

	response.Success(c, gin.H{"id": id})
}

// List 查询岗位列表
// @Summary 查询岗位列表
// @Description 分页查询岗位，可按岗位分类、岗位名称关键词和状态筛选，并返回岗位标签
// @Tags 岗位
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Param keyword query string false "岗位名称关键词"
// @Param categoryId query int false "岗位分类 ID"
// @Param status query string false "状态 all/draft/published/closed，不传或传 all 表示全部"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /jobs [get]
func (h *JobHandler) List(c *gin.Context) {
	page, pageSize := parsePageParams(c)

	categoryID, ok := parseOptionalInt64Query(c, "categoryId")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "categoryId 参数不合法", nil)
		return
	}

	query := dto.JobQuery{
		Page:       page,
		PageSize:   pageSize,
		Keyword:    c.Query("keyword"),
		CategoryID: categoryID,
		Status:     c.Query("status"),
	}

	items, total, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "查询岗位失败", err.Error())
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

// BindTags 给岗位绑定标签
// @Summary 给岗位绑定标签
// @Description 覆盖式设置岗位标签，传空数组表示清空岗位标签
// @Tags 岗位
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "岗位 ID"
// @Param request body dto.BindJobTagsRequest true "岗位标签绑定请求"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /jobs/{id}/tags [put]
func (h *JobHandler) BindTags(c *gin.Context) {
	jobID, ok := parseInt64Param(c, "id")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "岗位 ID 不合法", nil)
		return
	}

	var req dto.BindJobTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}

	if err := h.service.BindTags(c.Request.Context(), jobID, req); err != nil {
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}

		response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
		return
	}

	response.Success(c, gin.H{"id": jobID})
}

// ListTags 查询岗位已绑定标签
// @Summary 查询岗位已绑定标签
// @Description 查询指定岗位当前已经绑定的标签列表，用于编辑岗位时回显标签选择
// @Tags 岗位
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "岗位 ID"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /jobs/{id}/tags [get]
func (h *JobHandler) ListTags(c *gin.Context) {
	jobID, ok := parseInt64Param(c, "id")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "岗位 ID 不合法", nil)
		return
	}

	items, err := h.service.ListTags(c.Request.Context(), jobID)
	if err != nil {
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}

		response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
		return
	}

	response.Success(c, items)
}

// AssignMember 给岗位分配成员
// @Summary 给岗位分配成员
// @Description 给岗位分配协作成员；同一岗位同一用户已存在时更新成员角色
// @Tags 岗位
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "岗位 ID"
// @Param request body dto.AssignJobMemberRequest true "岗位成员分配请求"
// @Success 201 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /jobs/{id}/members [post]
func (h *JobHandler) AssignMember(c *gin.Context) {
	jobID, ok := parseInt64Param(c, "id")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "岗位 ID 不合法", nil)
		return
	}

	var req dto.AssignJobMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}

	memberID, err := h.service.AssignMember(c.Request.Context(), jobID, req)
	if err != nil {
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}

		response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
		return
	}

	response.Created(c, gin.H{"id": memberID})
}

// ListMembers 查询岗位成员列表
// @Summary 查询岗位成员列表
// @Description 查询指定岗位当前已分配的协作成员列表
// @Tags 岗位
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "岗位 ID"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /jobs/{id}/members [get]
func (h *JobHandler) ListMembers(c *gin.Context) {
	jobID, ok := parseInt64Param(c, "id")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "岗位 ID 不合法", nil)
		return
	}

	items, err := h.service.ListMembers(c.Request.Context(), jobID)
	if err != nil {
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}

		response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
		return
	}

	response.Success(c, items)
}

func parseInt64Param(c *gin.Context, key string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(key), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}

	return id, true
}
