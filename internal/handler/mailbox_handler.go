package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yangnuowen1-arch/resume_back/internal/mailbox"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
	"github.com/yangnuowen1-arch/resume_back/internal/response"
	"github.com/yangnuowen1-arch/resume_back/internal/service"
)

type MailboxHandler struct {
	mailboxService  service.MailboxService
	accountRepo     repository.MailboxAccountRepository
	providers       map[string]mailbox.Provider
	oauthRedirectURL string
}

func NewMailboxHandler(
	mailboxService service.MailboxService,
	accountRepo repository.MailboxAccountRepository,
	providers map[string]mailbox.Provider,
	oauthRedirectURL string,
) *MailboxHandler {
	return &MailboxHandler{
		mailboxService:   mailboxService,
		accountRepo:      accountRepo,
		providers:        providers,
		oauthRedirectURL: oauthRedirectURL,
	}
}

// GetOAuthURL 获取邮箱平台的 OAuth 授权跳转地址
// @Summary 获取 OAuth 授权地址
// @Description 返回指定邮箱平台（google）的 OAuth 授权跳转地址，用户点击后授权
// @Tags 邮箱
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param provider path string true "邮箱平台" Enums(google)
// @Success 200 {object} response.APIResponse{data=object{url=string}}
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Router /mailbox/oauth/{provider}/url [get]
func (h *MailboxHandler) GetOAuthURL(c *gin.Context) {
	provider := c.Param("provider")
	if provider == "" {
		response.Error(c, http.StatusBadRequest, 40001, "邮箱平台不能为空", nil)
		return
	}

	p, ok := h.providers[provider]
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "不支持的邮箱平台", nil)
		return
	}

	url := p.AuthURL("state")
	response.Success(c, gin.H{"url": url})
}

// OAuthCallback OAuth 授权回调，保存 token 到 mailbox_accounts
// @Summary OAuth 授权回调
// @Description 邮箱平台授权后的回调地址，用 code 换取 token 并存储到 mailbox_accounts
// @Tags 邮箱
// @Accept json
// @Produce json
// @Param provider path string true "邮箱平台" Enums(google)
// @Param code query string true "OAuth 授权码"
// @Param state query string false "状态码"
// @Success 200 {object} response.APIResponse{data=object{accountId=int64}}
// @Failure 400 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /mailbox/oauth/{provider}/callback [get]
func (h *MailboxHandler) OAuthCallback(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")

	if provider == "" || code == "" {
		response.Error(c, http.StatusBadRequest, 40001, "参数不完整", nil)
		return
	}

	p, ok := h.providers[provider]
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "不支持的邮箱平台", nil)
		return
	}

	ctx := c.Request.Context()
	token, err := p.Exchange(ctx, code)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "OAuth 授权失败: "+err.Error(), nil)
		return
	}

	// 获取邮箱地址（通过 provider 查询用户信息）
	email, err := p.GetUserEmail(ctx, token)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "获取邮箱地址失败: "+err.Error(), nil)
		return
	}

	// 查找或创建账号
	account, err := h.accountRepo.FindByProviderEmail(ctx, provider, email)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "查询邮箱账号失败", err.Error())
		return
	}

	var refreshToken *string
	if token.RefreshToken != "" {
		refreshToken = &token.RefreshToken
	}
	var expiry *string
	if !token.Expiry.IsZero() {
		exp := token.Expiry.Format("2006-01-02 15:04:05")
		expiry = &exp
	}

	if account == nil {
		// 新建账号
		if err := h.accountRepo.Create(ctx, provider, email, token.AccessToken, refreshToken, expiry); err != nil {
			response.Error(c, http.StatusInternalServerError, 50001, "创建邮箱账号失败", err.Error())
			return
		}
		account, _ = h.accountRepo.FindByProviderEmail(ctx, provider, email)
	} else {
		// 更新 token
		if err := h.accountRepo.UpdateTokenByID(ctx, account.ID, token.AccessToken, refreshToken, expiry); err != nil {
			response.Error(c, http.StatusInternalServerError, 50001, "更新 token 失败", err.Error())
			return
		}
	}

	response.Success(c, gin.H{"accountId": account.ID, "email": email})
}

// ListAccounts 列出已连接的邮箱账号
// @Summary 列出已连接邮箱账号
// @Description 返回当前已授权连接的所有邮箱账号列表
// @Tags 邮箱
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /mailbox/accounts [get]
func (h *MailboxHandler) ListAccounts(c *gin.Context) {
	accounts, err := h.accountRepo.List(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "查询邮箱账号失败", err.Error())
		return
	}

	result := make([]gin.H, 0, len(accounts))
	for _, acc := range accounts {
		result = append(result, gin.H{
			"id":           acc.ID,
			"provider":     acc.Provider,
			"email":        acc.Email,
			"lastScanAt":   acc.LastScanAt,
			"createdAt":    acc.CreatedAt,
			"updatedAt":    acc.UpdatedAt,
		})
	}

	response.Success(c, result)
}

// DeleteAccount 解绑邮箱账号
// @Summary 解绑邮箱账号
// @Description 删除指定的邮箱账号连接
// @Tags 邮箱
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "账号 ID"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /mailbox/accounts/{id} [delete]
func (h *MailboxHandler) DeleteAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(c, http.StatusBadRequest, 40001, "账号 ID 不合法", nil)
		return
	}

	if err := h.accountRepo.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "删除邮箱账号失败", err.Error())
		return
	}

	response.Success(c, nil)
}

// TriggerScan 手动触发邮箱扫描
// @Summary 手动触发邮箱扫描
// @Description 触发指定邮箱账号的扫描任务，异步执行，返回任务 ID 供轮询状态
// @Tags 邮箱
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body object{accountId=int64} true "扫描请求"
// @Success 200 {object} response.APIResponse{data=object{taskId=int64}}
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /mailbox/scan [post]
func (h *MailboxHandler) TriggerScan(c *gin.Context) {
	var req struct {
		AccountID int64 `json:"accountId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 40001, "参数错误", err.Error())
		return
	}

	taskID, err := h.mailboxService.EnqueueScan(c.Request.Context(), req.AccountID, service.ScanTriggerManual)
	if err != nil {
		if errors.Is(err, service.ErrScanInProgress) {
			response.Error(c, http.StatusBadRequest, 40001, "该邮箱账号正在扫描中", nil)
			return
		}
		response.Error(c, http.StatusInternalServerError, 50001, "触发扫描失败", err.Error())
		return
	}

	response.Success(c, gin.H{"taskId": taskID})
}

// GetScanStatus 查询扫描任务状态
// @Summary 查询扫描任务状态
// @Description 根据任务 ID 查询扫描任务的状态与统计（scanned/imported/skipped）
// @Tags 邮箱
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param taskId path int true "任务 ID"
// @Success 200 {object} response.APIResponse
// @Failure 400 {object} response.APIResponse
// @Failure 401 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /mailbox/scan/{taskId} [get]
func (h *MailboxHandler) GetScanStatus(c *gin.Context) {
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil || taskID <= 0 {
		response.Error(c, http.StatusBadRequest, 40001, "任务 ID 不合法", nil)
		return
	}

	status, err := h.mailboxService.GetScanTaskStatus(c.Request.Context(), taskID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 50001, "查询扫描任务失败", err.Error())
		return
	}

	response.Success(c, status)
}
