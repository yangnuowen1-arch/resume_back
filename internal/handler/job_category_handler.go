package handler

//Handler 负责：真正处理这个请求
import (
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

	// 登录功能没做之前，先传 nil
	var operatorID *int64 = nil

	id, err := h.service.Create(c.Request.Context(), req, operatorID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
		return
	}

	response.Created(c, gin.H{
		"id": id,
	})
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
// @Param status query string false "状态 active/disabled"
// @Success 200 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /job-categories [get]
func (h *JobCategoryHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	query := dto.JobCategoryQuery{
		Page:     page,
		PageSize: pageSize,
		Keyword:  c.Query("keyword"),
		Status:   c.DefaultQuery("status", "active"),
	}

	items, total, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "查询岗位分类失败", err.Error())
		return
	}

	if query.Page < 1 {
		query.Page = 1
	}

	if query.PageSize < 1 || query.PageSize > 100 {
		query.PageSize = 20
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
