package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"github.com/yangnuowen1-arch/resume_back/internal/auth"
	"github.com/yangnuowen1-arch/resume_back/internal/config"
	"github.com/yangnuowen1-arch/resume_back/internal/dal/model"
	"github.com/yangnuowen1-arch/resume_back/internal/mailbox"
	"github.com/yangnuowen1-arch/resume_back/internal/middleware"
	"github.com/yangnuowen1-arch/resume_back/internal/repository"
	"github.com/yangnuowen1-arch/resume_back/internal/service"
)

func TestMailboxOAuthCallbackIsPublicAndStateIsOneTime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := &testOAuthProvider{}
	accountRepo := &testMailboxAccountRepo{}
	handler := NewMailboxHandlerWithStateStore(
		testMailboxHandlerService{},
		accountRepo,
		map[string]mailbox.Provider{"google": provider},
		"",
		mailbox.NewInMemoryOAuthStateStore(),
	)
	router := newMailboxOAuthTestRouter(handler)

	flow := requestOAuthState(t, router)
	wrongBrowser := httptest.NewRequest(http.MethodGet, "/api/v1/mailbox/oauth/google/callback?code=auth-code&state="+url.QueryEscape(flow.state), nil)
	wrongBrowser.AddCookie(&http.Cookie{Name: oauthCallbackCookieName, Value: strings.Repeat("x", oauthBrowserBindingLength)})
	wrongBrowserResponse := httptest.NewRecorder()
	router.ServeHTTP(wrongBrowserResponse, wrongBrowser)
	if wrongBrowserResponse.Code != http.StatusBadRequest {
		t.Fatalf("callback from a different browser status=%d body=%s", wrongBrowserResponse.Code, wrongBrowserResponse.Body.String())
	}
	if provider.exchangeCalls != 0 {
		t.Fatalf("callback from a different browser must not exchange code, calls=%d", provider.exchangeCalls)
	}

	callback := httptest.NewRequest(http.MethodGet, "/api/v1/mailbox/oauth/google/callback?code=auth-code&state="+url.QueryEscape(flow.state), nil)
	callback.AddCookie(flow.cookie)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, callback)

	if response.Code != http.StatusOK {
		t.Fatalf("callback status=%d body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("callback must disable caching, got %q", response.Header().Get("Cache-Control"))
	}
	if provider.exchangeCalls != 1 || accountRepo.createCalls != 1 {
		t.Fatalf("exchange/create calls = %d/%d, want 1/1", provider.exchangeCalls, accountRepo.createCalls)
	}

	replay := httptest.NewRequest(http.MethodGet, "/api/v1/mailbox/oauth/google/callback?code=auth-code&state="+url.QueryEscape(flow.state), nil)
	replay.AddCookie(flow.cookie)
	replayResponse := httptest.NewRecorder()
	router.ServeHTTP(replayResponse, replay)
	if replayResponse.Code != http.StatusBadRequest {
		t.Fatalf("replayed callback status=%d body=%s", replayResponse.Code, replayResponse.Body.String())
	}
	if provider.exchangeCalls != 1 {
		t.Fatalf("replayed state must not exchange another code, calls=%d", provider.exchangeCalls)
	}
}

func TestMailboxOAuthCallbackEnqueuesInitialScan(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := &testOAuthProvider{}
	accountRepo := &testMailboxAccountRepo{}
	mailboxService := &recordingMailboxHandlerService{taskID: 23}
	handler := NewMailboxHandlerWithStateStore(
		mailboxService,
		accountRepo,
		map[string]mailbox.Provider{"google": provider},
		"",
		mailbox.NewInMemoryOAuthStateStore(),
	)
	router := newMailboxOAuthTestRouter(handler)

	flow := requestOAuthState(t, router)
	callback := httptest.NewRequest(http.MethodGet, "/api/v1/mailbox/oauth/google/callback?code=auth-code&state="+url.QueryEscape(flow.state), nil)
	callback.AddCookie(flow.cookie)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, callback)

	if response.Code != http.StatusOK {
		t.Fatalf("callback status=%d body=%s", response.Code, response.Body.String())
	}
	if mailboxService.enqueueCalls != 1 {
		t.Fatalf("enqueue calls=%d, want 1", mailboxService.enqueueCalls)
	}
	if mailboxService.accountID != 17 {
		t.Fatalf("enqueued accountId=%d, want 17", mailboxService.accountID)
	}
	if mailboxService.triggerSource != service.ScanTriggerManual {
		t.Fatalf("enqueue triggerSource=%q, want %q", mailboxService.triggerSource, service.ScanTriggerManual)
	}

	var payload struct {
		Data struct {
			AccountID int64  `json:"accountId"`
			Email     string `json:"email"`
			TaskID    int64  `json:"taskId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode callback response: %v", err)
	}
	if payload.Data.AccountID != 17 || payload.Data.Email != "test@example.com" || payload.Data.TaskID != 23 {
		t.Fatalf("unexpected callback response: %+v", payload.Data)
	}
}

func TestMailboxOAuthCallbackRedirectsToFrontendWithScanTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := &testOAuthProvider{}
	mailboxService := &recordingMailboxHandlerService{taskID: 23}
	handler := NewMailboxHandlerWithStateStore(
		mailboxService,
		&testMailboxAccountRepo{},
		map[string]mailbox.Provider{"google": provider},
		"",
		mailbox.NewInMemoryOAuthStateStore(),
		"http://frontend.example/candidates?tab=mailbox",
	)
	router := newMailboxOAuthTestRouter(handler)

	flow := requestOAuthState(t, router)
	callback := httptest.NewRequest(http.MethodGet, "/api/v1/mailbox/oauth/google/callback?code=auth-code&state="+url.QueryEscape(flow.state), nil)
	callback.AddCookie(flow.cookie)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, callback)

	if response.Code != http.StatusFound {
		t.Fatalf("callback status=%d body=%s", response.Code, response.Body.String())
	}
	redirectURL, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse redirect location: %v", err)
	}
	if redirectURL.String() == "" || redirectURL.Host != "frontend.example" || redirectURL.Path != "/candidates" {
		t.Fatalf("unexpected redirect location: %q", response.Header().Get("Location"))
	}
	query := redirectURL.Query()
	if query.Get("tab") != "mailbox" || query.Get("mailboxConnected") != "true" || query.Get("accountId") != "17" || query.Get("email") != "test@example.com" || query.Get("taskId") != "23" {
		t.Fatalf("unexpected redirect query: %s", redirectURL.RawQuery)
	}
}

func TestMailboxOAuthCallbackRejectsMissingInvalidAndDeniedState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := &testOAuthProvider{}
	handler := NewMailboxHandlerWithStateStore(
		testMailboxHandlerService{},
		&testMailboxAccountRepo{},
		map[string]mailbox.Provider{"google": provider},
		"",
		mailbox.NewInMemoryOAuthStateStore(),
	)
	router := newMailboxOAuthTestRouter(handler)

	for _, path := range []string{
		"/api/v1/mailbox/oauth/google/callback?code=auth-code",
		"/api/v1/mailbox/oauth/google/callback?code=auth-code&state=not-issued",
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid callback %q status=%d body=%s", path, response.Code, response.Body.String())
		}
	}

	flow := requestOAuthState(t, router)
	denied := httptest.NewRequest(http.MethodGet, "/api/v1/mailbox/oauth/google/callback?error=access_denied&state="+url.QueryEscape(flow.state), nil)
	denied.AddCookie(flow.cookie)
	deniedResponse := httptest.NewRecorder()
	router.ServeHTTP(deniedResponse, denied)
	if deniedResponse.Code != http.StatusBadRequest {
		t.Fatalf("denied callback status=%d body=%s", deniedResponse.Code, deniedResponse.Body.String())
	}
	if provider.exchangeCalls != 0 {
		t.Fatalf("invalid or denied callback must not exchange code, calls=%d", provider.exchangeCalls)
	}
}

func TestMailboxOAuthURLRequiresJWTAndReturnsRandomState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := &testOAuthProvider{}
	handler := NewMailboxHandlerWithStateStore(
		testMailboxHandlerService{},
		&testMailboxAccountRepo{},
		map[string]mailbox.Provider{"google": provider},
		"",
		mailbox.NewInMemoryOAuthStateStore(),
	)
	router := newMailboxOAuthTestRouter(handler)

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/mailbox/oauth/google/url", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("OAuth URL without JWT status=%d body=%s", unauthorized.Code, unauthorized.Body.String())
	}

	flow := requestOAuthState(t, router)
	state := flow.state
	if state == "state" || len(state) < 40 {
		t.Fatalf("state must be random and opaque, got %q", state)
	}
	if !flow.cookie.HttpOnly {
		t.Fatal("OAuth transaction cookie must be HttpOnly")
	}
	if flow.cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("OAuth transaction cookie SameSite=%v, want Lax", flow.cookie.SameSite)
	}
	if flow.cookie.Secure {
		t.Fatal("localhost OAuth transaction cookie must allow HTTP local development")
	}
}

func newMailboxOAuthTestRouter(h *MailboxHandler) *gin.Engine {
	router := gin.New()
	api := router.Group("/api/v1")
	api.GET("/mailbox/oauth/:provider/callback", h.OAuthCallback)
	private := api.Group("")
	private.Use(middleware.AuthMiddleware(&config.Config{JWTSecret: "test-secret"}))
	private.GET("/mailbox/oauth/:provider/url", h.GetOAuthURL)
	return router
}

type oauthAuthorizationFlow struct {
	state  string
	cookie *http.Cookie
}

func requestOAuthState(t *testing.T, router *gin.Engine) oauthAuthorizationFlow {
	t.Helper()
	token, err := auth.GenerateToken(7, "tester", []string{"user"}, "test-secret", "1")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/mailbox/oauth/google/url", nil)
	request.Host = "localhost:8081"
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("OAuth URL status=%d body=%s", response.Code, response.Body.String())
	}

	var payload struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode OAuth URL response: %v", err)
	}
	parsed, err := url.Parse(payload.Data.URL)
	if err != nil {
		t.Fatalf("parse OAuth URL: %v", err)
	}
	state := parsed.Query().Get("state")
	if state == "" {
		t.Fatalf("OAuth URL does not include state: %q", payload.Data.URL)
	}

	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == oauthCallbackCookieName {
			return oauthAuthorizationFlow{state: state, cookie: cookie}
		}
	}
	t.Fatal("OAuth URL response does not set a transaction cookie")
	return oauthAuthorizationFlow{}
}

type testMailboxHandlerService struct{}

func (testMailboxHandlerService) ScanAndImport(context.Context, int64) (service.ScanResult, error) {
	return service.ScanResult{}, nil
}
func (testMailboxHandlerService) EnqueueScan(context.Context, int64, string) (int64, error) {
	return 0, nil
}
func (testMailboxHandlerService) GetScanTaskStatus(context.Context, int64) (*service.ScanTaskStatus, error) {
	return nil, nil
}

type recordingMailboxHandlerService struct {
	testMailboxHandlerService
	taskID        int64
	enqueueCalls  int
	accountID     int64
	triggerSource string
}

func (s *recordingMailboxHandlerService) EnqueueScan(_ context.Context, accountID int64, triggerSource string) (int64, error) {
	s.enqueueCalls++
	s.accountID = accountID
	s.triggerSource = triggerSource
	return s.taskID, nil
}

type testOAuthProvider struct {
	lastState     string
	exchangeCalls int
}

func (p *testOAuthProvider) Provider() string { return mailbox.ProviderGoogle }
func (p *testOAuthProvider) AuthURL(state string) string {
	p.lastState = state
	return "https://accounts.example/authorize?state=" + url.QueryEscape(state)
}
func (p *testOAuthProvider) Exchange(context.Context, string) (*oauth2.Token, error) {
	p.exchangeCalls++
	return &oauth2.Token{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		Expiry:       time.Now().Add(time.Hour),
	}, nil
}
func (p *testOAuthProvider) RefreshToken(context.Context, *oauth2.Token) (*oauth2.Token, error) {
	return nil, nil
}
func (p *testOAuthProvider) GetUserEmail(context.Context, *oauth2.Token) (string, error) {
	return "test@example.com", nil
}
func (p *testOAuthProvider) ListUnread(context.Context, *oauth2.Token) ([]mailbox.Message, error) {
	return nil, nil
}
func (p *testOAuthProvider) FetchAttachments(context.Context, *oauth2.Token, string) ([]mailbox.Attachment, error) {
	return nil, nil
}
func (p *testOAuthProvider) MarkRead(context.Context, *oauth2.Token, string) error { return nil }

type testMailboxAccountRepo struct {
	account     *model.MailboxAccount
	createCalls int
}

func (r *testMailboxAccountRepo) Create(_ context.Context, provider, email, _ string, _, _ *string) error {
	r.createCalls++
	r.account = &model.MailboxAccount{ID: 17, Provider: provider, Email: email}
	return nil
}
func (r *testMailboxAccountRepo) FindByID(context.Context, int64) (*model.MailboxAccount, error) {
	return nil, nil
}
func (r *testMailboxAccountRepo) FindByProviderEmail(context.Context, string, string) (*model.MailboxAccount, error) {
	return r.account, nil
}
func (r *testMailboxAccountRepo) List(context.Context) ([]model.MailboxAccount, error) {
	return nil, nil
}
func (r *testMailboxAccountRepo) UpdateToken(context.Context, int64, string, *string, *time.Time) error {
	return nil
}
func (r *testMailboxAccountRepo) UpdateTokenByID(context.Context, int64, string, *string, *string) error {
	return nil
}
func (r *testMailboxAccountRepo) UpdateLastScanAt(context.Context, int64, time.Time) error {
	return nil
}
func (r *testMailboxAccountRepo) Delete(context.Context, int64) error { return nil }

var _ repository.MailboxAccountRepository = (*testMailboxAccountRepo)(nil)
