package handler

//Handler 负责：真正处理这个请求
import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/response"
	"github.com/yangnuowen1-arch/resume_back/internal/service"
)

// 依赖注入
type JobCategoryHandler struct {
	service service.JobCategoryService
}

// 创建 Handler 的构造函数 把 Service 注入到 Handler 里面
func NewJobCategoryHandler(service service.JobCategoryService) *JobCategoryHandler {
	return &JobCategoryHandler{
		service: service,
	}
}

// c *gin.Context 是 Gin 框架传进来的上下文
// 请求参数
// 请求头
// 响应方法
// 上下文信息

// Create 创建岗位分类
// @Summary 创建岗位分类
// @Description 创建一个新的岗位分类，例如技术、客服/运营、产品
// @Tags 岗位分类
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateJobCategoryRequest true "创建岗位分类请求"
// @Success 201 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /job-categories [post]
func (h *JobCategoryHandler) Create(c *gin.Context) {
	var req dto.CreateJobCategoryRequest

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

	response.Created(c, gin.H{
		"id": id,
	})
}

// Update 编辑岗位分类
// @Summary 编辑岗位分类
// @Description 根据 ID 编辑岗位分类名称、描述和状态
// @Tags 岗位分类
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "岗位分类 ID"
// @Param request body dto.UpdateJobCategoryRequest true "编辑岗位分类请求"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /job-categories/{id} [put]
func (h *JobCategoryHandler) Update(c *gin.Context) {
	id, ok := parseInt64Param(c, "id")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "岗位分类 ID 不合法", nil)
		return
	}
	// 创建一个请求参数对象，后面会把 JSON 请求体解析到这个对象里
	var req dto.UpdateJobCategoryRequest
	//把请求 JSON 解析到 req 这个结构体变量里面
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

// List 查询岗位分类列表
// @Summary 查询岗位分类列表
// @Description 分页查询岗位分类
// @Tags 岗位分类
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Param keyword query string false "关键词"
// @Param status query string false "状态 active/disabled，不传或传 all 表示全部"
// @Success 200 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /job-categories [get]
func (h *JobCategoryHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	if page < 1 {
		page = 1
	}

	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	query := dto.JobCategoryQuery{
		Page:     page,
		PageSize: pageSize,
		Keyword:  c.Query("keyword"),
		Status:   c.Query("status"),
	}

	items, total, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "查询岗位分类失败", err.Error())
		return
	}

	totalPages := int((total + int64(query.PageSize) - 1) / int64(query.PageSize))

	response.Success(c, response.PageResult{
		Items: items,
		Pagination: response.Pagination{
			Page:       query.Page,
			PageSize:   query.PageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}
