package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yangnuowen1-arch/resume_back/internal/auth"
	"github.com/yangnuowen1-arch/resume_back/internal/config"
	"github.com/yangnuowen1-arch/resume_back/internal/response"
)

func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		if authHeader == "" {
			response.Error(c, http.StatusUnauthorized, 40101, "未登录，请先登录", nil)
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			response.Error(c, http.StatusUnauthorized, 40101, "Authorization 格式错误", nil)
			c.Abort()
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if tokenString == "" {
			response.Error(c, http.StatusUnauthorized, 40101, "Token 不能为空", nil)
			c.Abort()
			return
		}

		claims, err := auth.ParseToken(tokenString, cfg.JWTSecret)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, 40101, "Token 无效或已过期", nil)
			//中断
			c.Abort()
			return
		}

		//把解析出来的用户信息存到当前请求的上下文里
		c.Set("userId", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("roles", claims.Roles)

		c.Next()
	}
}
