package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yangnuowen1-arch/resume_back/internal/response"
	"github.com/yangnuowen1-arch/resume_back/internal/service"
)

type DashboardHandler struct {
	service service.DashboardService
}

func NewDashboardHandler(service service.DashboardService) *DashboardHandler {
	return &DashboardHandler{service: service}
}

// Summary 查询仪表盘摘要
// @Summary 查询仪表盘摘要
// @Description 实时统计简历、待筛选任务、推荐和拒绝数量；响应不使用缓存，确保仪表盘与当前业务数据同步
// @Tags 仪表盘
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /dashboard/summary [get]
func (h *DashboardHandler) Summary(c *gin.Context) {
	summary, err := h.service.Summary(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "查询仪表盘摘要失败", err.Error())
		return
	}

	// Dashboard totals are expected to update immediately after writes. Prevent
	// browsers and intermediaries from serving a stale GET response.
	c.Header("Cache-Control", "no-store")
	response.Success(c, summary)
}
