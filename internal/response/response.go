package response

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

//标准返回结构

type APIResponse struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`  //interface{} → 这个字段类型不固定，啥都能放
	Error     interface{} `json:"error,omitempty"` //omitempty → 如果没值就别出现在 JSON 里，别占位置
	RequestID string      `json:"requestId"`
	Timestamp string      `json:"timestamp"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

type PageResult struct {
	Items      interface{} `json:"items"`
	Pagination Pagination  `json:"pagination"`
}

// 封装好的成功响应函数
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Code:      0,
		Message:   "success",
		Data:      data,
		RequestID: c.GetString("requestId"),
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, APIResponse{
		Code:      0,
		Message:   "success",
		Data:      data,
		RequestID: c.GetString("requestId"),
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func Error(c *gin.Context, httpStatus int, code int, message string, errData interface{}) {
	c.JSON(httpStatus, APIResponse{
		Code:      code,
		Message:   message,
		Error:     errData,
		RequestID: c.GetString("requestId"),
		Timestamp: time.Now().Format(time.RFC3339),
	})
}
