package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/response"
	"github.com/yangnuowen1-arch/resume_back/internal/service"
)

type AuthHandler struct {
	service service.AuthService
}

func NewAuthHandler(service service.AuthService) *AuthHandler {
	return &AuthHandler{
		service: service,
	}
}

// Register 用户注册
// @Summary 用户注册
// @Description 创建用户并返回登录 token
// @Tags 登录鉴权
// @Accept json
// @Produce json
// @Param request body dto.RegisterRequest true "注册请求"
// @Success 201 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 409 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}

	result, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrUsernameExists) || errors.Is(err, service.ErrEmailExists) {
			response.Error(c, http.StatusConflict, 40901, err.Error(), nil)
			return
		}

		response.Error(c, http.StatusInternalServerError, 50001, "注册失败", err.Error())
		return
	}

	response.Created(c, result)
}

// Login 用户登录
// @Summary 用户登录
// @Description 使用用户名或邮箱登录，返回 JWT token
// @Tags 登录鉴权
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "登录请求"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 403 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}

	result, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredential) {
			response.Error(c, http.StatusUnauthorized, 40101, err.Error(), nil)
			return
		}

		if errors.Is(err, service.ErrUserDisabled) {
			response.Error(c, http.StatusForbidden, 40301, err.Error(), nil)
			return
		}

		response.Error(c, http.StatusInternalServerError, 50001, "登录失败", err.Error())
		return
	}

	response.Success(c, result)
}
