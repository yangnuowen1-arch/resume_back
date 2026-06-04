package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/response"
	"github.com/yangnuowen1-arch/resume_back/internal/service"
)

type OperationLogHandler struct {
	service service.OperationLogService
}

func NewOperationLogHandler(service service.OperationLogService) *OperationLogHandler {
	return &OperationLogHandler{
		service: service,
	}
}

// List 查询操作日志列表
// @Summary 查询操作日志列表
// @Description 分页查询用户操作日志，返回前端表格可直接展示的 Timestamp/User/Action/Details 字段；user 可按用户名、姓名或用户 ID 筛选
// @Tags 操作日志
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Param user query string false "用户筛选：用户名/姓名/用户 ID"
// @Success 200 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /operation-logs [get]
func (h *OperationLogHandler) List(c *gin.Context) {
	page, pageSize := parsePageParams(c)

	query := dto.OperationLogQuery{
		Page:     page,
		PageSize: pageSize,
		User:     c.Query("user"),
	}

	items, total, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		if errors.Is(err, service.ErrUnauthenticated) {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			return
		}

		response.Error(c, http.StatusInternalServerError, 50001, "查询操作日志失败", err.Error())
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
