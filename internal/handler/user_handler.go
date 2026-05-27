package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/response"
	"github.com/yangnuowen1-arch/resume_back/internal/service"
)

type UserHandler struct {
	service service.UserService
}

func NewUserHandler(service service.UserService) *UserHandler {
	return &UserHandler{
		service: service,
	}
}

// Create 创建用户
// @Summary 创建用户
// @Description 创建系统用户，并可指定用户角色；不传 roles 时默认分配 user 角色
// @Tags 用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateUserRequest true "创建用户请求"
// @Success 201 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 409 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /users [post]
func (h *UserHandler) Create(c *gin.Context) {
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}

	id, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		writeUserServiceError(c, err, "创建用户失败")
		return
	}

	response.Created(c, gin.H{"id": id})
}

// Update 编辑用户
// @Summary 编辑用户
// @Description 根据 ID 编辑用户基础信息和状态
// @Tags 用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Param request body dto.UpdateUserRequest true "编辑用户请求"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 404 {object} response.APIResponse
// @Failure 409 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /users/{id} [put]
func (h *UserHandler) Update(c *gin.Context) {
	id, ok := parseInt64Param(c, "id")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "用户 ID 不合法", nil)
		return
	}

	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}

	if err := h.service.Update(c.Request.Context(), id, req); err != nil {
		writeUserServiceError(c, err, "编辑用户失败")
		return
	}

	response.Success(c, gin.H{"id": id})
}

// Delete 禁用用户
// @Summary 禁用用户
// @Description 根据 ID 禁用用户账号；账号不会被物理删除
// @Tags 用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 404 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /users/{id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	id, ok := parseInt64Param(c, "id")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "用户 ID 不合法", nil)
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		writeUserServiceError(c, err, "禁用用户失败")
		return
	}

	response.Success(c, gin.H{"id": id})
}

// Get 查询用户详情
// @Summary 查询用户详情
// @Description 根据 ID 查询用户基础信息和角色
// @Tags 用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 404 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /users/{id} [get]
func (h *UserHandler) Get(c *gin.Context) {
	id, ok := parseInt64Param(c, "id")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "用户 ID 不合法", nil)
		return
	}

	item, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		writeUserServiceError(c, err, "查询用户失败")
		return
	}

	response.Success(c, item)
}

// List 查询用户列表
// @Summary 查询用户列表
// @Description 分页查询用户，可按用户名、姓名、邮箱、手机号关键词和状态筛选；可用于岗位负责人下拉选择
// @Tags 用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Param keyword query string false "用户名/姓名/邮箱/手机号关键词"
// @Param status query string false "状态 active/disabled，不传或传 all 表示全部"
// @Success 200 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /users [get]
func (h *UserHandler) List(c *gin.Context) {
	page, pageSize := parsePageParams(c)

	query := dto.UserQuery{
		Page:     page,
		PageSize: pageSize,
		Keyword:  c.Query("keyword"),
		Status:   c.Query("status"),
	}

	items, total, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		writeUserServiceError(c, err, "查询用户失败")
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

// AssignRoles 分配用户角色
// @Summary 分配用户角色
// @Description 覆盖式设置用户角色
// @Tags 用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Param request body dto.AssignUserRolesRequest true "用户角色分配请求"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 404 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /users/{id}/roles [put]
func (h *UserHandler) AssignRoles(c *gin.Context) {
	id, ok := parseInt64Param(c, "id")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "用户 ID 不合法", nil)
		return
	}

	var req dto.AssignUserRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}

	if err := h.service.AssignRoles(c.Request.Context(), id, req); err != nil {
		writeUserServiceError(c, err, "分配用户角色失败")
		return
	}

	response.Success(c, gin.H{"id": id})
}

// ListRoles 查询角色列表
// @Summary 查询角色列表
// @Description 查询系统角色列表，用于用户创建和角色分配时选择
// @Tags 用户
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /roles [get]
func (h *UserHandler) ListRoles(c *gin.Context) {
	items, err := h.service.ListRoles(c.Request.Context())
	if err != nil {
		writeUserServiceError(c, err, "查询角色失败")
		return
	}

	response.Success(c, items)
}

func writeUserServiceError(c *gin.Context, err error, fallbackMessage string) {
	if errors.Is(err, service.ErrUnauthenticated) {
		response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
		return
	}

	if errors.Is(err, service.ErrUserNotFound) {
		response.Error(c, http.StatusNotFound, 40401, err.Error(), nil)
		return
	}

	if errors.Is(err, service.ErrUsernameExists) || errors.Is(err, service.ErrEmailExists) {
		response.Error(c, http.StatusConflict, 40901, err.Error(), nil)
		return
	}

	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, err.Error(), nil)
		return
	}

	response.Error(c, http.StatusInternalServerError, 50001, fallbackMessage, nil)
}
