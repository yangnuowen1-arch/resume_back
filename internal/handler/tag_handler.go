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

type TagHandler struct {
	service service.TagService
}

func NewTagHandler(service service.TagService) *TagHandler {
	return &TagHandler{
		service: service,
	}
}

// Create 创建标签
// @Summary 创建标签
// @Description 创建一个新的岗位标签，可选择归属标签分组
// @Tags 标签
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateTagRequest true "创建标签请求"
// @Success 201 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /tags [post]
func (h *TagHandler) Create(c *gin.Context) {
	var req dto.CreateTagRequest
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

// Update 编辑标签
// @Summary 编辑标签
// @Description 根据 ID 编辑标签名称、所属分组、颜色和状态
// @Tags 标签
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "标签 ID"
// @Param request body dto.UpdateTagRequest true "编辑标签请求"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /tags/{id} [put]
func (h *TagHandler) Update(c *gin.Context) {
	id, ok := parseInt64Param(c, "id")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "标签 ID 不合法", nil)
		return
	}

	var req dto.UpdateTagRequest
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

// List 查询标签列表
// @Summary 查询标签列表
// @Description 分页查询标签，可按标签分组、名称关键词和状态筛选
// @Tags 标签
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Param keyword query string false "关键词"
// @Param groupId query int false "标签分组 ID"
// @Param status query string false "状态 active/disabled" default(active)
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /tags [get]
func (h *TagHandler) List(c *gin.Context) {
	page, pageSize := parsePageParams(c)

	groupID, ok := parseOptionalInt64Query(c, "groupId")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "groupId 参数不合法", nil)
		return
	}

	query := dto.TagQuery{
		Page:     page,
		PageSize: pageSize,
		Keyword:  c.Query("keyword"),
		GroupID:  groupID,
		Status:   c.DefaultQuery("status", "active"),
	}

	items, total, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "查询标签失败", err.Error())
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

func parseOptionalInt64Query(c *gin.Context, key string) (*int64, bool) {
	value := c.Query(key)
	if value == "" {
		return nil, true
	}

	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return nil, false
	}

	return &id, true
}
