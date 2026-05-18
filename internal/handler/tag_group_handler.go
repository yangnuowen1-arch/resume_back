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

type TagGroupHandler struct {
	service service.TagGroupService
}

func NewTagGroupHandler(service service.TagGroupService) *TagGroupHandler {
	return &TagGroupHandler{
		service: service,
	}
}

// Create 创建标签分组
// @Summary 创建标签分组
// @Description 创建一个新的标签分组，例如技术能力、经验要求、面试评价
// @Tags 标签分组
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateTagGroupRequest true "创建标签分组请求"
// @Success 201 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /tag-groups [post]
func (h *TagGroupHandler) Create(c *gin.Context) {
	var req dto.CreateTagGroupRequest
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

// List 查询标签分组列表
// @Summary 查询标签分组列表
// @Description 分页查询标签分组，可按名称关键词和状态筛选
// @Tags 标签分组
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Param keyword query string false "关键词"
// @Param status query string false "状态 active/disabled" default(active)
// @Success 200 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /tag-groups [get]
func (h *TagGroupHandler) List(c *gin.Context) {
	page, pageSize := parsePageParams(c)

	query := dto.TagGroupQuery{
		Page:     page,
		PageSize: pageSize,
		Keyword:  c.Query("keyword"),
		Status:   c.DefaultQuery("status", "active"),
	}

	items, total, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "查询标签分组失败", err.Error())
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

func parsePageParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	return page, pageSize
}

func totalPages(total int64, pageSize int) int {
	return int((total + int64(pageSize) - 1) / int64(pageSize))
}
