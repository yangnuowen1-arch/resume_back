package dify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

const defaultRequestTimeout = 180 * time.Second

const (
	// 最多尝试次数（含首次），即首次失败后最多再重试 maxAttempts-1 次
	maxAttempts = 3
	// 第一次重试前的基础等待时间，之后每次翻倍
	retryBaseDelay = 500 * time.Millisecond
	// 退避等待的上限，避免等待过久
	retryMaxDelay = 5 * time.Second
)

type Client struct {
	baseURL                 string
	apiKey                  string
	user                    string
	resumeFileInputName     string
	jobContextInputName     string
	outputLanguageInputName string
	resultOutputName        string
	httpClient              *http.Client
}

type Config struct {
	BaseURL                 string
	APIKey                  string
	User                    string
	ResumeFileInputName     string
	JobContextInputName     string
	OutputLanguageInputName string
	ResultOutputName        string
}

type RunResumeScreeningRequest struct {
	File           io.Reader
	Filename       string
	ContentType    string
	JobContext     string
	OutputLanguage string
	User           string
}

type RunResumeScreeningResponse struct {
	WorkflowRunID string                 `json:"workflowRunId"`
	TaskID        string                 `json:"taskId"`
	ResultText    string                 `json:"resultText"`
	RawResponse   map[string]interface{} `json:"rawResponse"`
	RetryCount    int                    `json:"retryCount"`
}

type Error struct {
	Err        error
	RetryCount int
}

func (e *Error) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func RetryCount(err error) int {
	var difyErr *Error
	if errors.As(err, &difyErr) {
		return difyErr.RetryCount
	}
	return 0
}

type uploadFileResponse struct {
	ID string `json:"id"`
}

type workflowRunResponse struct {
	WorkflowRunID string `json:"workflow_run_id"`
	TaskID        string `json:"task_id"`
	Data          struct {
		Status  string                 `json:"status"`
		Outputs map[string]interface{} `json:"outputs"`
		Error   *string                `json:"error"`
	} `json:"data"`
}

func NewClient(config Config) *Client {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL != "" && !strings.HasSuffix(baseURL, "/v1") {
		baseURL += "/v1"
	}

	return &Client{
		baseURL:                 baseURL,
		apiKey:                  strings.TrimSpace(config.APIKey),
		user:                    defaultString(config.User, "resume_back"),
		resumeFileInputName:     defaultString(config.ResumeFileInputName, "resume_file"),
		jobContextInputName:     defaultString(config.JobContextInputName, "job_context"),
		outputLanguageInputName: defaultString(config.OutputLanguageInputName, "output_language"),
		resultOutputName:        defaultString(config.ResultOutputName, "screening_result"),
		httpClient:              &http.Client{Timeout: defaultRequestTimeout},
	}
}

func (c *Client) RunResumeScreening(ctx context.Context, req RunResumeScreeningRequest) (*RunResumeScreeningResponse, error) {
	if c == nil || c.apiKey == "" {
		return nil, errors.New("Dify 未配置")
	}
	if c.baseURL == "" {
		return nil, errors.New("Dify 地址未配置")
	}
	if req.File == nil {
		return nil, errors.New("简历文件不能为空")
	}

	user := defaultString(req.User, c.user)
	upload, uploadRetries, err := c.uploadFile(ctx, req, user)
	if err != nil {
		return nil, &Error{Err: err, RetryCount: uploadRetries}
	}

	result, workflowRetries, err := c.runWorkflow(ctx, req, upload.ID, user)
	if err != nil {
		return nil, &Error{Err: err, RetryCount: uploadRetries + workflowRetries}
	}
	result.RetryCount = uploadRetries + workflowRetries
	return result, nil
}

func (c *Client) uploadFile(ctx context.Context, req RunResumeScreeningRequest, user string) (*uploadFileResponse, int, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("user", user); err != nil {
		return nil, 0, err
	}

	filename := defaultString(req.Filename, "resume")
	contentType := defaultString(req.ContentType, "application/octet-stream")
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, escapeFilename(filename)))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, 0, err
	}
	if _, err := io.Copy(part, req.File); err != nil {
		return nil, 0, err
	}
	if err := writer.Close(); err != nil {
		return nil, 0, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/files/upload"), &body)
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	var result uploadFileResponse
	retries, err := c.doJSON(httpReq, &result)
	if err != nil {
		return nil, retries, fmt.Errorf("上传文件到 Dify 失败: %w", err)
	}
	if result.ID == "" {
		return nil, retries, errors.New("上传文件到 Dify 失败: 返回文件 ID 为空")
	}

	return &result, retries, nil
}

func (c *Client) runWorkflow(ctx context.Context, req RunResumeScreeningRequest, uploadFileID string, user string) (*RunResumeScreeningResponse, int, error) {
	payload := map[string]interface{}{
		"inputs": map[string]interface{}{
			c.resumeFileInputName: map[string]interface{}{
				"transfer_method": "local_file",
				"upload_file_id":  uploadFileID,
				"type":            "document",
			},
			c.jobContextInputName:     req.JobContext,
			c.outputLanguageInputName: defaultString(req.OutputLanguage, "Chinese"),
		},
		"response_mode": "blocking",
		"user":          user,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/workflows/run"), bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	var result workflowRunResponse
	raw, retries, err := c.doJSONWithRaw(httpReq, &result)
	if err != nil {
		return nil, retries, fmt.Errorf("运行 Dify workflow 失败: %w", err)
	}
	if result.Data.Error != nil && strings.TrimSpace(*result.Data.Error) != "" {
		return nil, retries, errors.New(*result.Data.Error)
	}

	resultText := c.extractResultText(result.Data.Outputs)
	if strings.TrimSpace(resultText) == "" {
		return nil, retries, errors.New("Dify workflow 返回结果为空")
	}

	return &RunResumeScreeningResponse{
		WorkflowRunID: result.WorkflowRunID,
		TaskID:        result.TaskID,
		ResultText:    resultText,
		RawResponse:   raw,
		RetryCount:    retries,
	}, retries, nil
}

func (c *Client) extractResultText(outputs map[string]interface{}) string {
	if len(outputs) == 0 {
		return ""
	}
	if value, ok := outputs[c.resultOutputName]; ok {
		return stringifyOutputValue(value)
	}
	if value, ok := outputs["text"]; ok {
		return stringifyOutputValue(value)
	}

	encoded, err := json.Marshal(outputs)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func (c *Client) doJSON(req *http.Request, target interface{}) (int, error) {
	_, retries, err := c.doJSONWithRaw(req, target)
	return retries, err
}

func (c *Client) doJSONWithRaw(req *http.Request, target interface{}) (map[string]interface{}, int, error) {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// http.Request 的 Body 是一次性的，重试前必须用 GetBody 重新构造一份
		if attempt > 1 {
			if err := resetRequestBody(req); err != nil {
				return nil, attempt - 1, err
			}
		}

		start := time.Now()
		data, statusCode, err := c.doOnce(req)
		duration := time.Since(start)

		if err == nil {
			log.Printf("dify request succeeded method=%s path=%s attempt=%d status=%d duration=%s",
				req.Method, req.URL.Path, attempt, statusCode, duration)

			if err := json.Unmarshal(data, target); err != nil {
				return nil, attempt - 1, err
			}
			raw := make(map[string]interface{})
			if err := json.Unmarshal(data, &raw); err != nil {
				return map[string]interface{}{}, attempt - 1, nil
			}
			return raw, attempt - 1, nil
		}

		lastErr = err
		retryable := isRetryable(statusCode, err)
		log.Printf("dify request failed method=%s path=%s attempt=%d status=%d duration=%s retryable=%t error=%v",
			req.Method, req.URL.Path, attempt, statusCode, duration, retryable, err)

		// 不可重试，或已经是最后一次尝试，直接返回
		if !retryable || attempt == maxAttempts {
			return nil, attempt - 1, lastErr
		}

		delay := backoffDelay(attempt)
		log.Printf("dify request retrying method=%s path=%s nextAttempt=%d delay=%s",
			req.Method, req.URL.Path, attempt+1, delay)

		select {
		case <-time.After(delay):
		case <-req.Context().Done():
			return nil, attempt - 1, req.Context().Err()
		}
	}

	return nil, maxAttempts - 1, lastErr
}

// doOnce 发送一次请求，返回响应体、状态码和错误。
// 对于非 2xx 响应返回一个携带状态码的错误，便于上层判断是否重试。
func (c *Client) doOnce(req *http.Request) ([]byte, int, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return data, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	return data, resp.StatusCode, nil
}

// resetRequestBody 在重试前重置请求体，使其可以被再次读取。
func resetRequestBody(req *http.Request) error {
	if req.Body == nil {
		return nil
	}
	if req.GetBody == nil {
		return errors.New("请求不支持重试: 缺少 GetBody")
	}
	body, err := req.GetBody()
	if err != nil {
		return err
	}
	req.Body = body
	return nil
}

// isRetryable 判断错误是否值得重试。
// 网络层错误（statusCode==0）以及 429、5xx 视为瞬时错误，可重试；
// 4xx（参数、鉴权等）为永久错误，不重试。
func isRetryable(statusCode int, err error) bool {
	if err == nil {
		return false
	}
	if statusCode == 0 {
		// httpClient.Do 直接失败，通常是网络错误或超时
		return true
	}
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	return statusCode >= 500 && statusCode <= 599
}

// backoffDelay 计算第 attempt 次失败后的等待时间：指数退避 + 随机抖动。
func backoffDelay(attempt int) time.Duration {
	delay := retryBaseDelay << (attempt - 1)
	if delay > retryMaxDelay {
		delay = retryMaxDelay
	}
	// 加入 0~25% 的随机抖动，避免多个任务同时重试打到 Dify
	jitter := time.Duration(rand.Int63n(int64(delay) / 4))
	return delay + jitter
}

func (c *Client) endpoint(path string) string {
	return c.baseURL + path
}

func defaultString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func stringifyOutputValue(value interface{}) string {
	if text, ok := value.(string); ok {
		return text
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func escapeFilename(filename string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace(filename)
}
