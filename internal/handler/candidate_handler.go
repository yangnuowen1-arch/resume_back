package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/response"
	"github.com/yangnuowen1-arch/resume_back/internal/service"
)

type CandidateHandler struct {
	service service.CandidateService
}

func NewCandidateHandler(service service.CandidateService) *CandidateHandler {
	return &CandidateHandler{
		service: service,
	}
}

// Create 创建候选人
// @Summary 创建候选人
// @Description 创建候选人基础档案
// @Tags 候选人
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateCandidateRequest true "创建候选人请求"
// @Success 201 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /candidates [post]
func (h *CandidateHandler) Create(c *gin.Context) {
	var req dto.CreateCandidateRequest
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

// Update 编辑候选人
// @Summary 编辑候选人
// @Description 根据 ID 编辑候选人基础档案
// @Tags 候选人
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "候选人 ID"
// @Param request body dto.UpdateCandidateRequest true "编辑候选人请求"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /candidates/{id} [put]
func (h *CandidateHandler) Update(c *gin.Context) {
	id, ok := parseInt64Param(c, "id")
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "候选人 ID 不合法", nil)
		return
	}

	var req dto.UpdateCandidateRequest
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

// List 查询候选人列表
// @Summary 查询候选人列表
// @Description 分页查询候选人，可按姓名、邮箱、手机号关键词和来源筛选
// @Tags 候选人
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param pageSize query int false "每页数量" default(20)
// @Param keyword query string false "姓名/邮箱/手机号关键词"
// @Param source query string false "候选人来源"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /candidates [get]
func (h *CandidateHandler) List(c *gin.Context) {
	page, pageSize := parsePageParams(c)

	query := dto.CandidateQuery{
		Page:     page,
		PageSize: pageSize,
		Keyword:  c.Query("keyword"),
		Source:   c.Query("source"),
	}

	items, total, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "查询候选人失败", err.Error())
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
