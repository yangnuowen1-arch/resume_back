package handler

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yangnuowen1-arch/resume_back/internal/auth"
	"github.com/yangnuowen1-arch/resume_back/internal/mailbox"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
	"github.com/yangnuowen1-arch/resume_back/internal/response"
	"github.com/yangnuowen1-arch/resume_back/internal/service"
)

const (
	oauthCallbackCookieName   = "mailbox_oauth_transaction"
	oauthCallbackCookieMaxAge = 10 * 60
	oauthBrowserBindingBytes  = 32
	oauthBrowserBindingLength = 43 // 32 bytes 的 Raw URL Base64 编码长度
	maxOAuthStateLength       = 512
)

type MailboxHandler struct {
	mailboxService          service.MailboxService
	accountRepo             repository.MailboxAccountRepository
	providers               map[string]mailbox.Provider
	oauthRedirectURL        string
	oauthSuccessRedirectURL string
	oauthStates             mailbox.OAuthStateStore
}

func NewMailboxHandler(
	mailboxService service.MailboxService,
	accountRepo repository.MailboxAccountRepository,
	providers map[string]mailbox.Provider,
	oauthRedirectURL string,
	oauthSuccessRedirectURL string,
) *MailboxHandler {
	return NewMailboxHandlerWithStateStore(
		mailboxService,
		accountRepo,
		providers,
		oauthRedirectURL,
		mailbox.NewInMemoryOAuthStateStore(),
		oauthSuccessRedirectURL,
	)
}

// NewMailboxHandlerWithStateStore 允许部署环境注入共享的 OAuth state 存储。
// 默认构造函数使用单进程内存实现，适用于本地联调和单实例服务。
func NewMailboxHandlerWithStateStore(
	mailboxService service.MailboxService,
	accountRepo repository.MailboxAccountRepository,
	providers map[string]mailbox.Provider,
	oauthRedirectURL string,
	oauthStates mailbox.OAuthStateStore,
	oauthSuccessRedirectURLs ...string,
) *MailboxHandler {
	if oauthStates == nil {
		oauthStates = mailbox.NewInMemoryOAuthStateStore()
	}
	oauthSuccessRedirectURL := ""
	if len(oauthSuccessRedirectURLs) > 0 {
		oauthSuccessRedirectURL = strings.TrimSpace(oauthSuccessRedirectURLs[0])
	}
	return &MailboxHandler{
		mailboxService:          mailboxService,
		accountRepo:             accountRepo,
		providers:               providers,
		oauthRedirectURL:        oauthRedirectURL,
		oauthSuccessRedirectURL: oauthSuccessRedirectURL,
		oauthStates:             oauthStates,
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

	claims, ok := auth.ClaimsFromContext(c.Request.Context())
	if !ok || claims.UserID <= 0 {
		response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
		return
	}

	browserBinding, err := oauthBrowserBinding(c)
	if err != nil {
		log.Printf("mailbox oauth browser binding create failed provider=%s userId=%d error=%v", provider, claims.UserID, err)
		response.Error(c, http.StatusInternalServerError, 50001, "创建 OAuth 授权请求失败", nil)
		return
	}

	state, err := h.oauthStates.Create(claims.UserID, provider, browserBinding)
	if err != nil {
		log.Printf("mailbox oauth state create failed provider=%s userId=%d error=%v", provider, claims.UserID, err)
		response.Error(c, http.StatusInternalServerError, 50001, "创建 OAuth 授权请求失败", nil)
		return
	}

	url := p.AuthURL(state)
	response.Success(c, gin.H{"url": url})
}

// OAuthCallback OAuth 授权回调，保存 token 到 mailbox_accounts 并立即发起首次扫描
// @Summary OAuth 授权回调
// @Description 邮箱平台授权后的回调地址，用 code 换取 token 并存储到 mailbox_accounts；授权成功后立即异步扫描未读邮件中的简历附件
// @Tags 邮箱
// @Accept json
// @Produce json
// @Param provider path string true "邮箱平台" Enums(google)
// @Param code query string true "OAuth 授权码"
// @Param state query string true "一次性 OAuth 状态码"
// @Param error query string false "OAuth 授权错误码"
// @Success 200 {object} response.APIResponse{data=object{accountId=int64,email=string,taskId=int64}}
// @Failure 400 {object} response.APIResponse
// @Failure 500 {object} response.APIResponse
// @Router /mailbox/oauth/{provider}/callback [get]
func (h *MailboxHandler) OAuthCallback(c *gin.Context) {
	// OAuth code 会出现在 URL 中，禁止浏览器/中间层缓存或经 Referer 外泄。
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Referrer-Policy", "no-referrer")

	provider := c.Param("provider")
	state := c.Query("state")
	code := c.Query("code")

	if provider == "" || state == "" || len(state) > maxOAuthStateLength {
		response.Error(c, http.StatusBadRequest, 40001, "参数不完整", nil)
		return
	}

	p, ok := h.providers[provider]
	if !ok {
		response.Error(c, http.StatusBadRequest, 40001, "不支持的邮箱平台", nil)
		return
	}

	browserBinding, err := c.Cookie(oauthCallbackCookieName)
	if err != nil || len(browserBinding) != oauthBrowserBindingLength {
		response.Error(c, http.StatusBadRequest, 40001, "OAuth 授权请求无效或已过期，请重新发起授权", nil)
		return
	}

	request, valid := h.oauthStates.Consume(state, provider, browserBinding)
	if !valid {
		response.Error(c, http.StatusBadRequest, 40001, "OAuth 授权请求无效或已过期，请重新发起授权", nil)
		return
	}

	if c.Query("error") != "" {
		response.Error(c, http.StatusBadRequest, 40001, "OAuth 授权未完成，请重新发起授权", nil)
		return
	}
	if code == "" {
		response.Error(c, http.StatusBadRequest, 40001, "参数不完整", nil)
		return
	}

	ctx := c.Request.Context()
	token, err := p.Exchange(ctx, code)
	if err != nil {
		log.Printf("mailbox oauth exchange failed provider=%s userId=%d error=%v", provider, request.UserID, err)
		response.Error(c, http.StatusBadRequest, 40001, "OAuth 授权失败，请重新发起授权", nil)
		return
	}
	if token == nil || token.AccessToken == "" {
		response.Error(c, http.StatusBadRequest, 40001, "OAuth 授权失败，请重新发起授权", nil)
		return
	}

	// 获取邮箱地址（通过 provider 查询用户信息）
	email, err := p.GetUserEmail(ctx, token)
	if err != nil {
		log.Printf("mailbox oauth get user email failed provider=%s userId=%d error=%v", provider, request.UserID, err)
		response.Error(c, http.StatusBadRequest, 40001, "获取邮箱地址失败，请重新发起授权", nil)
		return
	}

	// 查找或创建账号
	account, err := h.accountRepo.FindByProviderEmail(ctx, provider, email)
	if err != nil {
		log.Printf("mailbox oauth find account failed provider=%s userId=%d error=%v", provider, request.UserID, err)
		response.Error(c, http.StatusInternalServerError, 50001, "查询邮箱账号失败", nil)
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
		if token.RefreshToken == "" {
			response.Error(c, http.StatusBadRequest, 40001, "未获得长期授权，请重新发起授权", nil)
			return
		}
		// 新建账号
		if err := h.accountRepo.Create(ctx, provider, email, token.AccessToken, refreshToken, expiry); err != nil {
			log.Printf("mailbox oauth create account failed provider=%s userId=%d error=%v", provider, request.UserID, err)
			response.Error(c, http.StatusInternalServerError, 50001, "创建邮箱账号失败", nil)
			return
		}
		account, err = h.accountRepo.FindByProviderEmail(ctx, provider, email)
		if err != nil || account == nil {
			log.Printf("mailbox oauth reload account failed provider=%s userId=%d error=%v", provider, request.UserID, err)
			response.Error(c, http.StatusInternalServerError, 50001, "创建邮箱账号失败", nil)
			return
		}
	} else {
		// 更新 token
		if err := h.accountRepo.UpdateTokenByID(ctx, account.ID, token.AccessToken, refreshToken, expiry); err != nil {
			log.Printf("mailbox oauth update token failed provider=%s userId=%d accountId=%d error=%v", provider, request.UserID, account.ID, err)
			response.Error(c, http.StatusInternalServerError, 50001, "更新 token 失败", nil)
			return
		}
	}

	// 首次连接（以及重新授权）完成后立即异步扫描。扫描服务会从未读邮件中提取
	// PDF/DOCX 附件，自动创建或合并候选人并保存对应简历；回调不等待扫描结束。
	taskID, err := h.mailboxService.EnqueueScan(ctx, account.ID, service.ScanTriggerManual)
	if err != nil {
		log.Printf("mailbox oauth enqueue initial scan failed provider=%s userId=%d accountId=%d error=%v", provider, request.UserID, account.ID, err)
		response.Error(c, http.StatusInternalServerError, 50001, "邮箱授权成功，但创建首次扫描任务失败", err.Error())
		return
	}

	log.Printf("mailbox oauth callback completed provider=%s userId=%d accountId=%d taskId=%d", provider, request.UserID, account.ID, taskID)
	if h.oauthSuccessRedirectURL != "" {
		redirectURL, err := buildOAuthSuccessRedirectURL(h.oauthSuccessRedirectURL, account.ID, email, taskID)
		if err != nil {
			// 回跳地址由部署配置提供。配置错误时仍返回 JSON，避免已经创建的扫描任务
			// 因前端跳转失败而对调用方不可见。
			log.Printf("mailbox oauth success redirect ignored accountId=%d taskId=%d error=%v", account.ID, taskID, err)
		} else {
			c.Redirect(http.StatusFound, redirectURL)
			return
		}
	}

	response.Success(c, gin.H{"accountId": account.ID, "email": email, "taskId": taskID})
}

// buildOAuthSuccessRedirectURL 把授权和扫描任务信息带回前端，保留配置地址已有的查询参数。
func buildOAuthSuccessRedirectURL(rawURL string, accountID int64, email string, taskID int64) (string, error) {
	redirectURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	if (redirectURL.Scheme != "http" && redirectURL.Scheme != "https") || redirectURL.Host == "" {
		return "", errors.New("OAuth 成功回跳地址必须是完整的 HTTP(S) URL")
	}

	query := redirectURL.Query()
	query.Set("mailboxConnected", "true")
	query.Set("accountId", strconv.FormatInt(accountID, 10))
	query.Set("email", email)
	query.Set("taskId", strconv.FormatInt(taskID, 10))
	redirectURL.RawQuery = query.Encode()
	return redirectURL.String(), nil
}

// oauthBrowserBinding 返回当前浏览器的短期 transaction nonce。state 只会保存该值的
// SHA-256，Google 回调必须带回同一浏览器的 HttpOnly cookie 才能继续。
func oauthBrowserBinding(c *gin.Context) (string, error) {
	if binding, err := c.Cookie(oauthCallbackCookieName); err == nil && len(binding) == oauthBrowserBindingLength {
		return binding, nil
	}

	bytes := make([]byte, oauthBrowserBindingBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	binding := base64.RawURLEncoding.EncodeToString(bytes)

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		oauthCallbackCookieName,
		binding,
		oauthCallbackCookieMaxAge,
		"/api/v1/mailbox/oauth/",
		"",
		oauthCookieSecure(c),
		true,
	)
	return binding, nil
}

func oauthCookieSecure(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}

	host := c.Request.Host
	if hostname, _, err := net.SplitHostPort(host); err == nil {
		host = hostname
	}
	host = strings.Trim(host, "[]")
	// 本地 OAuth 回调采用 http://localhost；其他主机必须走 HTTPS，因此默认
	// 设置 Secure cookie。这样不会信任客户端可伪造的 X-Forwarded-Proto。
	return host != "localhost" && host != "127.0.0.1" && host != "::1"
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
			"id":         acc.ID,
			"provider":   acc.Provider,
			"email":      acc.Email,
			"lastScanAt": acc.LastScanAt,
			"createdAt":  acc.CreatedAt,
			"updatedAt":  acc.UpdatedAt,
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
