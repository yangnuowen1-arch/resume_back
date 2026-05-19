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
