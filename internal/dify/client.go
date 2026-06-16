package dify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

const defaultRequestTimeout = 180 * time.Second

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
	upload, err := c.uploadFile(ctx, req, user)
	if err != nil {
		return nil, err
	}

	return c.runWorkflow(ctx, req, upload.ID, user)
}

func (c *Client) uploadFile(ctx context.Context, req RunResumeScreeningRequest, user string) (*uploadFileResponse, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("user", user); err != nil {
		return nil, err
	}

	filename := defaultString(req.Filename, "resume")
	contentType := defaultString(req.ContentType, "application/octet-stream")
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, escapeFilename(filename)))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, req.File); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/files/upload"), &body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	var result uploadFileResponse
	if err := c.doJSON(httpReq, &result); err != nil {
		return nil, fmt.Errorf("上传文件到 Dify 失败: %w", err)
	}
	if result.ID == "" {
		return nil, errors.New("上传文件到 Dify 失败: 返回文件 ID 为空")
	}

	return &result, nil
}

func (c *Client) runWorkflow(ctx context.Context, req RunResumeScreeningRequest, uploadFileID string, user string) (*RunResumeScreeningResponse, error) {
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
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/workflows/run"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	var result workflowRunResponse
	raw, err := c.doJSONWithRaw(httpReq, &result)
	if err != nil {
		return nil, fmt.Errorf("运行 Dify workflow 失败: %w", err)
	}
	if result.Data.Error != nil && strings.TrimSpace(*result.Data.Error) != "" {
		return nil, errors.New(*result.Data.Error)
	}

	resultText := c.extractResultText(result.Data.Outputs)
	if strings.TrimSpace(resultText) == "" {
		return nil, errors.New("Dify workflow 返回结果为空")
	}

	return &RunResumeScreeningResponse{
		WorkflowRunID: result.WorkflowRunID,
		TaskID:        result.TaskID,
		ResultText:    resultText,
		RawResponse:   raw,
	}, nil
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

func (c *Client) doJSON(req *http.Request, target interface{}) error {
	_, err := c.doJSONWithRaw(req, target)
	return err
}

func (c *Client) doJSONWithRaw(req *http.Request, target interface{}) (map[string]interface{}, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if err := json.Unmarshal(data, target); err != nil {
		return nil, err
	}

	raw := make(map[string]interface{})
	if err := json.Unmarshal(data, &raw); err != nil {
		return map[string]interface{}{}, nil
	}
	return raw, nil
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
