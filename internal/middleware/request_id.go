package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// 每个请求都生成一个 requestId，方便查日志

// gin.HandlerFunc 就是 Gin 规定的中间件/处理函数的格式 返回了一个函数
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = "req_" + uuid.NewString()
		}

		c.Set("requestId", requestID)
		c.Header("X-Request-ID", requestID)

		c.Next() //放行
	}
}
