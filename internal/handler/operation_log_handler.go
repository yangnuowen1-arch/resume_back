package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/response"
	"github.com/yangnuowen1-arch/resume_back/internal/service"
	"github.com/yangnuowen1-arch/resume_back/internal/timeutil"
)

var operationLogTimeLocation = timeutil.Shanghai

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
// @Param date query string false "操作日期，格式 YYYY-MM-DD（兼容 YYYY/MM/DD），按 Asia/Shanghai 自然日筛选"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /operation-logs [get]
func (h *OperationLogHandler) List(c *gin.Context) {
	page, pageSize := parsePageParams(c)
	date, err := parseOperationLogDate(c.Query("date"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "date 参数格式不合法，应为 YYYY-MM-DD", nil)
		return
	}

	query := dto.OperationLogQuery{
		Page:     page,
		PageSize: pageSize,
		User:     c.Query("user"),
		Date:     date,
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

func parseOperationLogDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	for _, layout := range []string{time.DateOnly, "2006/01/02"} {
		date, err := time.ParseInLocation(layout, value, operationLogTimeLocation)
		if err == nil {
			return &date, nil
		}
	}

	return nil, errors.New("invalid operation log date")
}
