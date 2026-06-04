package middleware

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yangnuowen1-arch/resume_back/internal/dto"
	"github.com/yangnuowen1-arch/resume_back/internal/service"
)

type responseCaptureWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *responseCaptureWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *responseCaptureWriter) WriteString(data string) (int, error) {
	w.body.WriteString(data)
	return w.ResponseWriter.WriteString(data)
}

func OperationLogMiddleware(operationLogService service.OperationLogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if operationLogService == nil || !shouldRecordOperation(c.Request.Method) {
			c.Next()
			return
		}

		writer := &responseCaptureWriter{ResponseWriter: c.Writer}
		c.Writer = writer

		c.Next()

		if c.Writer.Status() < http.StatusOK || c.Writer.Status() >= http.StatusMultipleChoices {
			return
		}

		req := buildRecordOperationLogRequest(c, writer.body.Bytes())
		if req.Action == "" {
			return
		}

		if err := operationLogService.Record(c.Request.Context(), req); err != nil {
			log.Printf("记录操作日志失败: %v", err)
		}
	}
}

func shouldRecordOperation(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

func buildRecordOperationLogRequest(c *gin.Context, responseBody []byte) dto.RecordOperationLogRequest {
	routePath := c.FullPath()
	if routePath == "" {
		routePath = c.Request.URL.Path
	}

	module := operationModule(routePath)
	targetType := singularizeModule(module)
	targetID := operationTargetID(c, responseBody)
	afterData := operationResponseData(c, routePath, responseBody)
	ipAddress := c.ClientIP()
	userAgent := c.Request.UserAgent()

	return dto.RecordOperationLogRequest{
		Action:     operationAction(c.Request.Method, routePath, module),
		Module:     optionalString(module),
		TargetType: optionalString(targetType),
		TargetID:   targetID,
		AfterData:  afterData,
		IPAddress:  optionalString(ipAddress),
		UserAgent:  optionalString(userAgent),
	}
}

func operationModule(routePath string) string {
	parts := routeParts(routePath)
	if len(parts) == 0 {
		return ""
	}

	return parts[0]
}

func operationAction(method string, routePath string, module string) string {
	parts := routeParts(routePath)
	nameParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.HasPrefix(part, ":") {
			continue
		}
		nameParts = append(nameParts, strings.ReplaceAll(part, "-", "_"))
	}

	name := strings.Join(nameParts, "_")
	if name == "" {
		name = strings.ReplaceAll(module, "-", "_")
	}

	switch method {
	case http.MethodPost:
		if strings.Contains(name, "batch_analyze") {
			return "batch_analyze_" + strings.ReplaceAll(module, "-", "_")
		}
		if strings.Contains(name, "resume") {
			return "upload_resume"
		}
		if strings.Contains(name, "upload") {
			return "upload_" + singularizeModule(module)
		}
		if strings.Contains(name, "members") {
			return "assign_" + singularizeModule(module) + "_member"
		}
		return "create_" + singularizeModule(module)
	case http.MethodPut:
		return "update_" + name
	case http.MethodDelete:
		return "delete_" + singularizeModule(module)
	default:
		return strings.ToLower(method) + "_" + strings.ReplaceAll(module, "-", "_")
	}
}

func operationTargetID(c *gin.Context, responseBody []byte) *int64 {
	for _, key := range []string{"id", "candidateId", "jobId", "resumeId"} {
		if value := c.Param(key); value != "" {
			if id, err := strconv.ParseInt(value, 10, 64); err == nil && id > 0 {
				return &id
			}
		}
	}

	return responseDataID(responseBody)
}

func operationResponseData(c *gin.Context, routePath string, responseBody []byte) *string {
	data := map[string]interface{}{
		"method":    c.Request.Method,
		"path":      c.Request.URL.Path,
		"route":     routePath,
		"status":    c.Writer.Status(),
		"requestId": c.GetString("requestId"),
	}

	if rawData := responseDataRawMessage(responseBody); len(rawData) > 0 {
		data["responseData"] = rawData
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return nil
	}

	result := string(encoded)
	return &result
}

func responseDataID(responseBody []byte) *int64 {
	rawData := responseDataRawMessage(responseBody)
	if len(rawData) == 0 {
		return nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal(rawData, &data); err != nil {
		return nil
	}

	for _, key := range []string{"id", "candidateId", "jobId", "resumeId"} {
		value, ok := data[key]
		if !ok {
			continue
		}
		if id, ok := numberAsInt64(value); ok && id > 0 {
			return &id
		}
	}

	return nil
}

func responseDataRawMessage(responseBody []byte) json.RawMessage {
	var apiResponse struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &apiResponse); err != nil {
		return nil
	}
	if len(apiResponse.Data) == 0 || string(apiResponse.Data) == "null" {
		return nil
	}

	return apiResponse.Data
}

func routeParts(routePath string) []string {
	trimmed := strings.Trim(routePath, "/")
	if trimmed == "" {
		return nil
	}

	parts := strings.Split(trimmed, "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		result = append(result, part)
	}

	return result
}

func singularizeModule(module string) string {
	normalized := strings.ReplaceAll(module, "-", "_")
	switch normalized {
	case "job_categories":
		return "job_category"
	case "tag_groups":
		return "tag_group"
	case "applications":
		return "application"
	case "candidates":
		return "candidate"
	case "jobs":
		return "job"
	case "resumes":
		return "resume"
	case "roles":
		return "role"
	case "tags":
		return "tag"
	case "users":
		return "user"
	default:
		return normalized
	}
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	return &value
}

func numberAsInt64(value interface{}) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), typed == float64(int64(typed))
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case json.Number:
		id, err := typed.Int64()
		return id, err == nil
	default:
		return 0, false
	}
}
